"""Image files as one image chunk each, plus the loader that prepares pixels for the model."""

from __future__ import annotations

import io

from PIL import Image, ImageOps

from agi_memory.chunking.common import ChunkingError
from agi_memory.types import Chunk, ChunkKind


def load_image(data: bytes, max_side: int) -> Image.Image:
    """Decode, apply EXIF orientation, convert to RGB and thumbnail to `max_side`."""
    try:
        with Image.open(io.BytesIO(data)) as opened:
            image = ImageOps.exif_transpose(opened).convert("RGB")
    except Exception as error:
        raise ChunkingError(f"unreadable image: {error}") from error
    image.thumbnail((max_side, max_side))
    return image


def image_chunk(source_id: str, uri: str, file_sha256: str, mtime_ns: int) -> Chunk:
    """The single chunk for an image file. Its text is the path, so file names are searchable."""
    return Chunk(
        source_id=source_id,
        uri=uri,
        kind=ChunkKind.IMAGE,
        locator="{}",
        content_hash=file_sha256,
        mtime_ns=mtime_ns,
        text=uri,
    )
