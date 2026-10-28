"""Audio files and the soundtracks of videos: 30 s windows, a new one every 15 s, through PyAV.

The model embeds what it hears (docs/superpowers/specs/2026-10-28-audio-search-design.md), so a
window is simply the samples. Three measured facts shape this file (docs/eval/2026-10-28-audio-
embedding.md):

- A window is at most 30 s: the server's batch holds about 40 s, and a longer window blurs topics.
- A window starts every 15 s. Speech cut by a 30 s grid scored 0.605 at worst, the level of
  nonsense; with the overlap the worst case is 0.664 and a passage always sits whole inside one
  window.
- Silence looks like an answer (cosine 0.60 to 0.64 to queries, above the 0.57 of an off-corpus
  query), so a window with no sound at all is never embedded. The model ignores loudness (speech
  at -60 dBFS is found as well as at -20), and noise of any level scores 0.62 to 0.63, so the line
  cannot be drawn between quiet speech and noise: it only removes silence and hiss.

Decoding streams: a three-hour podcast is never held as one array.
"""

from __future__ import annotations

from collections.abc import Iterator
from pathlib import Path
from types import TracebackType
from typing import Self

import av
import numpy as np

from omni_memory.chunking.common import ChunkingError, locator_json, sha256_hex
from omni_memory.embedding.base import AUDIO_RATE, MAX_AUDIO_SECONDS
from omni_memory.types import Chunk, ChunkKind

WINDOW = MAX_AUDIO_SECONDS * AUDIO_RATE  # samples
STRIDE = 15 * AUDIO_RATE
SHORTEST = AUDIO_RATE  # a sound under a second is not worth a vector
# Root mean square of the loudest 0.5 s below which a window is silence (-54 dBFS): digital silence
# and hiss measured 0.000 to 0.001. Quiet recordings stay far above it (speech here averages 0.099).
SILENCE = 0.002
_BLOCK = AUDIO_RATE // 2
# Part of every window's identity: a different length or stride re-embeds instead of mixing.
GEOMETRY = "audio1"


def is_silent(window: np.ndarray) -> bool:
    """True when no half second of the window is louder than `SILENCE` (RMS)."""
    blocks = len(window) // _BLOCK
    if blocks == 0:
        return float(np.sqrt(np.mean(np.square(window)))) < SILENCE if len(window) else True
    loud = np.sqrt(np.mean(np.square(window[: blocks * _BLOCK].reshape(blocks, _BLOCK)), axis=1))
    return float(loud.max()) < SILENCE


class AudioTrack:
    """The first audio stream of a file, opened for reading. Use as a context manager."""

    def __init__(self, path: Path) -> None:
        # Any failure to open is this file's problem (an odd container, a legacy metadata encoding,
        # whatever the decoder raises), so it is a ChunkingError and the file keeps its name chunk.
        try:
            self._container = av.open(str(path))
        except Exception as error:
            raise ChunkingError(f"unreadable audio: {error}") from error
        self._stream = self._container.streams.audio[0] if self._container.streams.audio else None

    @property
    def present(self) -> bool:
        """False for a file with no audio stream (a silent screen recording, a rendered clip)."""
        return self._stream is not None

    def __enter__(self) -> Self:
        return self

    def __exit__(
        self,
        exc_type: type[BaseException] | None,
        exc: BaseException | None,
        traceback: TracebackType | None,
    ) -> None:
        self._container.close()

    def _samples(self) -> Iterator[np.ndarray]:
        """The track as float32 mono at `AUDIO_RATE`, frame by frame."""
        assert self._stream is not None
        resampler = av.AudioResampler(format="flt", layout="mono", rate=AUDIO_RATE)
        try:
            for frame in self._container.decode(self._stream):
                for out in resampler.resample(frame):
                    yield out.to_ndarray()[0]
            for out in resampler.resample(None):
                yield out.to_ndarray()[0]
        except Exception as error:  # any decoder failure is this file's problem
            raise ChunkingError(f"cannot decode the audio: {error}") from error

    def windows(self) -> Iterator[tuple[float, np.ndarray]]:
        """`(start in seconds, samples)` of every window worth embedding, in order.

        Starts are 0, 15, 30, ... and the last, shorter window is kept when it runs past the one
        before it (the part left over is longer than 15 s, the overlap), so every second is in at
        least one window; a track under a second has none. Silent windows are skipped (but still
        advance the clock)."""
        if self._stream is None:
            return
        parts: list[np.ndarray] = []
        have = 0
        start = 0  # samples
        emitted = False

        def window(samples: np.ndarray) -> Iterator[tuple[float, np.ndarray]]:
            if not is_silent(samples):
                yield start / AUDIO_RATE, samples

        for chunk in self._samples():
            parts.append(chunk)
            have += len(chunk)
            if have < WINDOW:
                continue
            buffer = np.concatenate(parts)
            while len(buffer) >= WINDOW:
                yield from window(buffer[:WINDOW])
                emitted = True
                buffer = buffer[STRIDE:]
                start += STRIDE
            parts, have = [buffer], len(buffer)
        tail = np.concatenate(parts) if parts else np.zeros(0, dtype=np.float32)
        if len(tail) > STRIDE or (not emitted and len(tail) >= SHORTEST):
            yield from window(tail)


def audio_name_chunk(source_id: str, uri: str, identity: str, mtime_ns: int) -> Chunk:
    """The chunk that finds an audio file by its name: its text is the path, embedded as text."""
    return Chunk(
        source_id=source_id,
        uri=uri,
        kind=ChunkKind.AUDIO,
        locator="{}",
        content_hash=f"name:{identity}",
        mtime_ns=mtime_ns,
        text=uri,
    )


def audio_window_chunk(
    source_id: str, uri: str, identity: str, seconds: float, mtime_ns: int
) -> Chunk:
    """One window of sound, identified by the file and where it starts."""
    return Chunk(
        source_id=source_id,
        uri=uri,
        kind=ChunkKind.AUDIO_WINDOW,
        locator=locator_json(t=seconds),
        content_hash=sha256_hex(f"{identity}:{GEOMETRY}:{seconds}"),
        mtime_ns=mtime_ns,
        text="",  # the name is searchable through the audio chunk
    )
