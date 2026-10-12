"""PDF pages as text chunks and as image chunks, through pdfium.

Each page yields up to two chunks: `pdf_text` (the extracted text, searchable by keyword and by
meaning) and `pdf_page` (the rendered page, embedded as an image so tables, diagrams and scanned
pages are findable). Rendering is lazy: the caller renders pages in small batches so a 500-page
PDF never sits in memory as bitmaps.
"""

from __future__ import annotations

from types import TracebackType
from typing import Self

import pypdfium2 as pdfium
from PIL.Image import Image

from agi_memory.chunking.common import ChunkingError, locator_json, sha256_hex
from agi_memory.config import Settings
from agi_memory.types import Chunk, ChunkKind


class PdfPages:
    """An open PDF. Use as a context manager so the native handle is always released."""

    def __init__(self, data: bytes) -> None:
        try:
            self._pdf = pdfium.PdfDocument(data)
        except pdfium.PdfiumError as error:
            raise ChunkingError(f"unreadable PDF: {error}") from error

    def __enter__(self) -> Self:
        return self

    def __exit__(
        self,
        exc_type: type[BaseException] | None,
        exc: BaseException | None,
        traceback: TracebackType | None,
    ) -> None:
        self.close()

    def close(self) -> None:
        self._pdf.close()

    def __len__(self) -> int:
        return len(self._pdf)

    def text(self, index: int) -> str:
        """Extracted text of page `index` (0-based); empty for scanned pages."""
        try:
            page = self._pdf[index]
            try:
                textpage = page.get_textpage()
                try:
                    return textpage.get_text_range()
                finally:
                    textpage.close()
            finally:
                page.close()
        except pdfium.PdfiumError as error:
            raise ChunkingError(f"cannot read text of PDF page {index + 1}: {error}") from error

    def render(self, index: int, scale: float) -> Image:
        """Page `index` (0-based) as an RGB image."""
        try:
            page = self._pdf[index]
            try:
                # pdfium's stubs type `scale` as int (from its default); floats are documented
                bitmap = page.render(scale=scale)  # pyright: ignore[reportArgumentType]
                return bitmap.to_pil().convert("RGB")
            finally:
                page.close()
        except pdfium.PdfiumError as error:
            raise ChunkingError(f"cannot render PDF page {index + 1}: {error}") from error


def pdf_text_chunks(
    source_id: str, uri: str, pdf: PdfPages, mtime_ns: int, settings: Settings
) -> list[Chunk]:
    """One `pdf_text` chunk for every page that has any text."""
    chunks = []
    for index in range(len(pdf)):
        body = pdf.text(index).replace("\r\n", "\n").strip()
        if not body:
            continue
        text = f"{uri}\n{body}"[: settings.max_chunk_chars]
        chunks.append(
            Chunk(
                source_id=source_id,
                uri=uri,
                kind=ChunkKind.PDF_TEXT,
                locator=locator_json(page=index + 1),
                content_hash=sha256_hex(text),
                mtime_ns=mtime_ns,
                text=text,
            )
        )
    return chunks


def pdf_page_chunks(
    source_id: str, uri: str, file_sha256: str, page_count: int, mtime_ns: int
) -> list[Chunk]:
    """One `pdf_page` (image) chunk per page, identified by the file's hash and the page number."""
    return [
        Chunk(
            source_id=source_id,
            uri=uri,
            kind=ChunkKind.PDF_PAGE,
            locator=locator_json(page=page),
            content_hash=sha256_hex(f"{file_sha256}:{page}"),
            mtime_ns=mtime_ns,
            text="",  # the page's words are already searchable through its pdf_text chunk
        )
        for page in range(1, page_count + 1)
    ]
