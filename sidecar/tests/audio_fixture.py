import wave
from pathlib import Path

import numpy as np


def tone(seconds: float, rate: int = 16_000, level: float = 0.3, freq: float = 220.0) -> np.ndarray:
    """A steady tone: loud enough to count as sound (RMS about 0.2)."""
    t = np.arange(int(seconds * rate)) / rate
    return (level * np.sin(2 * np.pi * freq * t)).astype(np.float32)


def make_wav(
    path: Path,
    seconds: float = 4,
    *,
    rate: int = 16_000,
    channels: int = 1,
    sounds: tuple[tuple[float, float], ...] | None = None,
) -> Path:
    """A WAV silent except for tones in `sounds` (start, end) seconds; by default all loud."""
    samples = np.zeros(int(seconds * rate), dtype=np.float32)
    for start, end in sounds or ((0.0, seconds),):
        first, last = int(start * rate), int(end * rate)
        samples[first:last] = tone((last - first) / rate, rate)[: last - first]
    pcm = np.repeat((samples * 32767).astype("<i2")[:, None], channels, axis=1)
    with wave.open(str(path), "wb") as out:
        out.setnchannels(channels)
        out.setsampwidth(2)
        out.setframerate(rate)
        out.writeframes(pcm.tobytes())
    return path
