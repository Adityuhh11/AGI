"""Videos: a chunk for the name, and frames to embed as pictures, through PyAV (ffmpeg's libraries).

A video is never read whole. Its identity is a hash of its size and its first and last mebibyte,
enough to notice that a file moved without reading gigabytes. Frames are taken by seeking to the
middle of each of `FRAMES` equal parts, so a 30-minute video costs about what a 5-second one does
(0.25 to 0.5 s for eight frames, docs/superpowers/specs/2026-10-20-indexing-choices-design.md).
"""

from __future__ import annotations

import hashlib
from pathlib import Path
from types import TracebackType
from typing import Self

import av
from av.error import FFmpegError
from PIL.Image import Image

from agi_memory.chunking.common import ChunkingError, locator_json, sha256_hex
from agi_memory.types import Chunk, ChunkKind

FRAMES = 8
_EDGE = 1 << 20  # bytes hashed from each end of the file


def video_identity(path: Path, size: int) -> str:
    """A content hash of the size and the first and last mebibyte."""
    digest = hashlib.sha256(str(size).encode())
    with path.open("rb") as file:
        digest.update(file.read(_EDGE))
        if size > 2 * _EDGE:
            file.seek(size - _EDGE)
        digest.update(file.read(_EDGE))
    return digest.hexdigest()


class Video:
    """An open video file. Use as a context manager so the decoder is always released."""

    def __init__(self, path: Path) -> None:
        try:
            self._container = av.open(str(path))
            if not self._container.streams.video:
                raise ChunkingError("the file has no video stream")
            self._stream = self._container.streams.video[0]
        except FFmpegError as error:
            raise ChunkingError(f"unreadable video: {error}") from error
        except ChunkingError:
            self._container.close()
            raise
        self._stream.thread_type = "AUTO"
        duration = self._container.duration
        self.duration = 0.0 if duration is None else duration / av.time_base  # seconds

    def __enter__(self) -> Self:
        return self

    def __exit__(
        self,
        exc_type: type[BaseException] | None,
        exc: BaseException | None,
        traceback: TracebackType | None,
    ) -> None:
        self._container.close()

    def frame_times(self) -> list[float]:
        """The moments to take frames at: the middle of each of `FRAMES` equal parts, each once (a
        very short video, or one of unknown length, has fewer)."""
        times = (round(self.duration * (i + 0.5) / FRAMES, 1) for i in range(FRAMES))
        return list(dict.fromkeys(times))

    def frame(self, seconds: float, max_side: int) -> Image:
        """The first frame at or after `seconds`, as an RGB picture no larger than `max_side`."""
        try:
            time_base = self._stream.time_base
            target = int(seconds / time_base) if time_base else 0
            self._container.seek(target, stream=self._stream, backward=True)
            for frame in self._container.decode(self._stream):
                if frame.time is None or frame.time >= seconds - 0.05:
                    image = frame.to_image().convert("RGB")
                    image.thumbnail((max_side, max_side))
                    return image
        except FFmpegError as error:
            raise ChunkingError(f"cannot decode the frame at {seconds} s: {error}") from error
        raise ChunkingError(f"no frame at {seconds} s")


def video_name_chunk(source_id: str, uri: str, identity: str, mtime_ns: int) -> Chunk:
    """The chunk that finds a video by its name: its text is the path, embedded as text."""
    return Chunk(
        source_id=source_id,
        uri=uri,
        kind=ChunkKind.VIDEO,
        locator="{}",
        content_hash=f"name:{identity}",
        mtime_ns=mtime_ns,
        text=uri,
    )


def video_frame_chunks(
    source_id: str, uri: str, identity: str, times: list[float], mtime_ns: int
) -> list[Chunk]:
    """One `video_frame` chunk per moment, identified by the file and the time."""
    return [
        Chunk(
            source_id=source_id,
            uri=uri,
            kind=ChunkKind.VIDEO_FRAME,
            locator=locator_json(t=seconds),
            content_hash=sha256_hex(f"{identity}:{seconds}"),
            mtime_ns=mtime_ns,
            text="",  # the name is searchable through the video chunk
        )
        for seconds in times
    ]
