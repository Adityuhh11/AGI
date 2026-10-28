from pathlib import Path

import av
import numpy as np


def make_video(
    path: Path, seconds: int = 4, fps: int = 10, audio_seconds: int = 0, loud: bool = False
) -> Path:
    """A small clip whose colour changes every second, so its frames differ; with `audio_seconds`, a
    silent audio track of that length too (Matroska), which may outlast the picture; `loud` makes it
    a tone instead."""
    with av.open(str(path), "w") as container:
        stream = container.add_stream("mpeg4", rate=fps)
        stream.width, stream.height, stream.pix_fmt = 64, 48, "yuv420p"
        audio = container.add_stream("pcm_s16le", rate=8000) if audio_seconds else None
        for n in range(seconds * fps):
            shade = (n // fps) * 60 % 256
            pixels = np.full((48, 64, 3), (shade, 255 - shade, 90), dtype=np.uint8)
            for packet in stream.encode(av.VideoFrame.from_ndarray(pixels, format="rgb24")):
                container.mux(packet)
        for packet in stream.encode():
            container.mux(packet)
        if audio is not None:
            for second in range(audio_seconds):
                level = (9000 * np.sin(2 * np.pi * 220 * np.arange(8000) / 8000)) if loud else 0
                frame = av.AudioFrame.from_ndarray(
                    np.zeros((1, 8000), dtype=np.int16) + np.asarray(level, dtype=np.int16),
                    format="s16",
                    layout="mono",
                )
                frame.sample_rate, frame.pts = 8000, second * 8000
                for packet in audio.encode(frame):
                    container.mux(packet)
            for packet in audio.encode():
                container.mux(packet)
    return path
