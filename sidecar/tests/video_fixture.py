from pathlib import Path

import av
import numpy as np


def make_video(path: Path, seconds: int = 4, fps: int = 10) -> Path:
    """A small MPEG-4 clip whose colour changes every second, so its frames differ."""
    with av.open(str(path), "w") as container:
        stream = container.add_stream("mpeg4", rate=fps)
        stream.width, stream.height, stream.pix_fmt = 64, 48, "yuv420p"
        for n in range(seconds * fps):
            shade = (n // fps) * 60 % 256
            pixels = np.full((48, 64, 3), (shade, 255 - shade, 90), dtype=np.uint8)
            for packet in stream.encode(av.VideoFrame.from_ndarray(pixels, format="rgb24")):
                container.mux(packet)
        for packet in stream.encode():
            container.mux(packet)
    return path
