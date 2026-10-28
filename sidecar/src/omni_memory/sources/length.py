"""How long a sound is, guessed from the file's size, because a survey opens no file.

A survey must not read what it counts (a cloud-only OneDrive file would be fetched to be counted),
so the length of an audio file or a video comes from its size and its format's usual data rate. It
is a plan, not a measurement: the owner's files ran from 16 KB/s (a 128 kbit/s mp3) to 32 KB/s (a
256 kbit/s one), and videos from 157 to 696 KB/s. The time left that indexing reports once it runs
corrects what this says.
"""

from __future__ import annotations

from pathlib import PurePosixPath

# Bytes per second of the sound by format: constant-bitrate lossy audio at the usual 128 to 160
# kbit/s, Ogg and Opus a little lower, FLAC about 60% of a CD, WAV as a CD.
_AUDIO_BYTES_PER_SECOND = {
    ".mp3": 20_000,
    ".m4a": 16_000,
    ".aac": 16_000,
    ".wma": 16_000,
    ".ogg": 12_000,
    ".opus": 12_000,
    ".flac": 100_000,
    ".wav": 176_400,
}
VIDEO_BYTES_PER_SECOND = 300_000  # the middle of 157 to 696 KB/s on real videos


def estimated_seconds(uri: str, size: int, *, video: bool) -> float:
    """The length in seconds of a file's sound, from its size alone."""
    if video:
        return size / VIDEO_BYTES_PER_SECOND
    rate = _AUDIO_BYTES_PER_SECOND.get(PurePosixPath(uri).suffix.lower(), 20_000)
    return size / rate
