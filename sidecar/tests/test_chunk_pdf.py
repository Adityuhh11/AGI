import io

import pytest
from PIL import Image

from agi_memory.chunking.common import ChunkingError
from agi_memory.chunking.pdf import PdfPages, pdf_page_chunks, pdf_text_chunks
from agi_memory.config import Settings
from agi_memory.types import ChunkKind
from pdf_fixture import make_text_pdf


@pytest.fixture
def settings(tmp_path):
    return Settings(index_dir=tmp_path)


def image_only_pdf() -> bytes:
    buffer = io.BytesIO()
    Image.new("RGB", (200, 100), "white").save(buffer, format="PDF")
    return buffer.getvalue()


def test_text_pages_become_text_chunks(settings):
    data = make_text_pdf(["Hello PDF world", "Second page (with parens)"])
    with PdfPages(data) as pdf:
        assert len(pdf) == 2
        chunks = pdf_text_chunks("docs", "a/Report.pdf", pdf, 5, settings)
    assert [c.locator for c in chunks] == ['{"page":1}', '{"page":2}']
    assert all(c.kind is ChunkKind.PDF_TEXT for c in chunks)
    assert chunks[0].text.startswith("a/Report.pdf\nHello PDF world")
    assert "(with parens)" in chunks[1].text
    assert chunks[0].mtime_ns == 5


def test_page_image_chunks_are_keyed_by_file_hash_and_page(settings):
    first = pdf_page_chunks("docs", "a.pdf", "hash-one", 3, 5)
    again = pdf_page_chunks("docs", "a.pdf", "hash-one", 3, 5)
    other = pdf_page_chunks("docs", "a.pdf", "hash-two", 3, 5)
    assert [c.locator for c in first] == ['{"page":1}', '{"page":2}', '{"page":3}']
    assert all(c.kind is ChunkKind.PDF_PAGE and c.text == "" for c in first)
    assert [c.id for c in first] == [c.id for c in again]
    assert {c.id for c in first}.isdisjoint({c.id for c in other})


def test_a_scanned_pdf_has_no_text_chunks_but_still_renders(settings):
    with PdfPages(image_only_pdf()) as pdf:
        assert pdf_text_chunks("docs", "scan.pdf", pdf, 1, settings) == []
        image = pdf.render(0, 1.5)
    assert image.mode == "RGB"
    assert image.size[0] > 200  # a 200pt-wide page at 1.5x scale


def test_long_page_text_is_capped(settings):
    data = make_text_pdf(["word " * 5000])
    with PdfPages(data) as pdf:
        (chunk,) = pdf_text_chunks("docs", "long.pdf", pdf, 1, settings)
    assert len(chunk.text) == settings.max_chunk_chars


def test_corrupt_pdf_is_a_chunking_error():
    with pytest.raises(ChunkingError, match="PDF"):
        PdfPages(b"%PDF-1.4 this is not a pdf")
