import io

import pytest
from PIL import Image

from agi_memory.chunking.common import ChunkingError
from agi_memory.chunking.image import image_chunk, load_image
from agi_memory.types import ChunkKind


def encode(image, fmt, **kwargs):
    buffer = io.BytesIO()
    image.save(buffer, format=fmt, **kwargs)
    return buffer.getvalue()


def test_large_images_are_thumbnailed_to_the_max_side():
    data = encode(Image.new("RGB", (3000, 2000), "red"), "PNG")
    image = load_image(data, 1024)
    assert max(image.size) == 1024
    assert image.mode == "RGB"


@pytest.mark.parametrize("mode", ["RGBA", "P", "L"])
def test_every_mode_is_converted_to_rgb(mode):
    data = encode(Image.new(mode, (20, 10)), "PNG")
    assert load_image(data, 1024).mode == "RGB"


def test_exif_orientation_is_applied():
    exif = Image.Exif()
    exif[0x0112] = 6  # rotate 90 degrees clockwise to display
    data = encode(Image.new("RGB", (100, 50), "blue"), "JPEG", exif=exif)
    assert load_image(data, 1024).size == (50, 100)


def test_undecodable_bytes_are_a_chunking_error():
    with pytest.raises(ChunkingError, match="image"):
        load_image(b"definitely not an image", 1024)


def test_image_chunk_keeps_the_file_name_searchable():
    chunk = image_chunk("pics", "trips/Rome 2025.png", "abc123", 9)
    assert chunk.kind is ChunkKind.IMAGE
    assert (chunk.text, chunk.locator, chunk.content_hash, chunk.mtime_ns) == (
        "trips/Rome 2025.png", "{}", "abc123", 9,
    )  # fmt: skip
