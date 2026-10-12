"""Indexer: brings a store in line with a folder, doing embedding work only for what changed.

Flow for one `sync`: scan the folder, diff against the files the store already knows, remove what
disappeared, then for every new or changed file read it once, hash it and chunk it. Text chunks
from many small files are embedded together (one GPU call has a ~200 ms fixed cost, so one call
per file would dominate indexing time); PDFs and images are embedded per file because they are
heavy and rendered lazily. Each file lands in the store atomically (`FlatStore.replace_file`).

Failure policy: a file that cannot be read or chunked is recorded in the report and skipped; an
embedder or store failure aborts the sync (everything committed so far stays valid, and the
next sync continues from there because file records are written last).
"""

from __future__ import annotations

import hashlib
import time
from collections.abc import Callable
from dataclasses import dataclass, field
from pathlib import Path

import numpy as np

from agi_memory.chunking.common import ChunkingError
from agi_memory.chunking.image import image_chunk, load_image
from agi_memory.chunking.pdf import PdfPages, pdf_page_chunks, pdf_text_chunks
from agi_memory.chunking.text import NotTextError, chunk_text_document
from agi_memory.config import Settings
from agi_memory.embedding.base import Embedder
from agi_memory.sources.folder import ScannedFile, scan
from agi_memory.sources.rules import Route
from agi_memory.store import FlatStore
from agi_memory.types import Chunk, FileRecord

_TEXT_ROUTES = frozenset({Route.CODE, Route.MARKDOWN, Route.TEXT})


@dataclass(frozen=True, slots=True)
class FileError:
    uri: str
    message: str


@dataclass(slots=True)
class SyncReport:
    added: int = 0
    updated: int = 0
    unchanged: int = 0
    removed: int = 0
    denied: int = 0
    ignored: int = 0
    unsupported: int = 0
    too_large: int = 0
    chunks_written: int = 0
    errors: list[FileError] = field(default_factory=list)


@dataclass(slots=True)
class _Pending:
    record: FileRecord
    chunks: list[Chunk]
    is_new: bool


class Indexer:
    """Keeps one store in sync with folders, one `sync` call per source."""

    def __init__(
        self,
        store: FlatStore,
        embedder: Embedder,
        settings: Settings,
        *,
        clock: Callable[[], int] = time.time_ns,
    ) -> None:
        self._store = store
        self._embedder = embedder
        self._settings = settings
        self._clock = clock

    def sync(
        self,
        source_id: str,
        root: Path,
        *,
        on_progress: Callable[[int, int], None] | None = None,
    ) -> SyncReport:
        """Index new and changed files under `root`, drop vanished ones, return what happened."""
        if not root.is_dir():
            raise FileNotFoundError(f"source root is not a folder: {root}")
        scanned = scan(root, self._settings)
        report = SyncReport(
            denied=scanned.denied,
            ignored=scanned.ignored,
            unsupported=scanned.unsupported,
            too_large=scanned.too_large,
        )
        report.errors.extend(FileError(uri, message) for uri, message in scanned.unreadable)
        self._store.register_source(source_id, str(root))
        known = self._store.file_records(source_id)

        on_disk = {file.uri for file in scanned.files}
        protected = tuple(prefix for prefix, _ in scanned.unreadable)  # could not look in there
        gone = [u for u in sorted(known) if u not in on_disk and not u.startswith(protected)]
        report.removed = self._store.remove_files(source_id, gone)

        pending: list[_Pending] = []
        total = len(scanned.files)
        for number, file in enumerate(scanned.files, start=1):
            try:
                self._index_file(source_id, file, known.get(file.uri), report, pending)
            except NotTextError:
                report.unsupported += 1
            except (ChunkingError, OSError) as error:
                report.errors.append(FileError(file.uri, str(error)))
            if sum(len(item.chunks) for item in pending) >= self._settings.flush_chunks:
                self._flush(pending, report)
            if on_progress is not None:
                on_progress(number, total)
        self._flush(pending, report)
        self._store.mark_scanned(source_id, self._clock())
        return report

    # --- per file -----------------------------------------------------------------------

    def _index_file(
        self,
        source_id: str,
        file: ScannedFile,
        known: FileRecord | None,
        report: SyncReport,
        pending: list[_Pending],
    ) -> None:
        if known is not None and known.size == file.size and known.mtime_ns == file.mtime_ns:
            report.unchanged += 1
            return
        data = file.path.read_bytes()
        digest = hashlib.sha256(data).hexdigest()
        record = FileRecord(source_id, file.uri, file.size, file.mtime_ns, digest)
        if known is not None and known.sha256 == digest:
            self._store.touch_file(record)  # same bytes, new mtime: nothing to embed
            report.unchanged += 1
            return
        is_new = known is None
        if file.route in _TEXT_ROUTES:
            chunks = chunk_text_document(
                source_id, file.uri, file.route, data, file.mtime_ns, self._settings
            )
            pending.append(_Pending(record, chunks, is_new))
        elif file.route is Route.PDF:
            self._index_pdf(record, data, file.mtime_ns, is_new, report)
        else:
            self._index_image(record, data, file.mtime_ns, is_new, report)

    def _flush(self, pending: list[_Pending], report: SyncReport) -> None:
        """Embed every pending text chunk in one call, then store each file atomically."""
        if not pending:
            return
        texts = [chunk.text for item in pending for chunk in item.chunks]
        vectors = (
            self._embedder.embed_documents(texts)
            if texts
            else np.zeros((0, self._embedder.dim), dtype=np.float32)
        )
        offset = 0
        for item in pending:
            count = len(item.chunks)
            self._commit(
                item.record, item.chunks, vectors[offset : offset + count], item.is_new, report
            )
            offset += count
        pending.clear()

    def _commit(
        self,
        record: FileRecord,
        chunks: list[Chunk],
        vectors: np.ndarray,
        is_new: bool,
        report: SyncReport,
    ) -> None:
        self._store.replace_file(record, chunks, vectors)
        report.chunks_written += len(chunks)
        if is_new:
            report.added += 1
        else:
            report.updated += 1

    def _index_pdf(
        self, record: FileRecord, data: bytes, mtime_ns: int, is_new: bool, report: SyncReport
    ) -> None:
        with PdfPages(data) as pdf:
            text_chunks = pdf_text_chunks(
                record.source_id, record.uri, pdf, mtime_ns, self._settings
            )
            page_chunks = pdf_page_chunks(
                record.source_id, record.uri, record.sha256, len(pdf), mtime_ns
            )
            text_vectors = self._embedder.embed_documents([c.text for c in text_chunks])
            page_vectors = np.zeros((len(page_chunks), self._embedder.dim), dtype=np.float32)
            step = self._settings.image_batch_size
            for start in range(0, len(page_chunks), step):
                stop = min(start + step, len(page_chunks))
                images = [
                    pdf.render(i, self._settings.pdf_render_scale) for i in range(start, stop)
                ]
                page_vectors[start:stop] = self._embedder.embed_images(images)
        self._commit(
            record,
            text_chunks + page_chunks,
            np.concatenate([text_vectors, page_vectors]),
            is_new,
            report,
        )

    def _index_image(
        self, record: FileRecord, data: bytes, mtime_ns: int, is_new: bool, report: SyncReport
    ) -> None:
        image = load_image(data, self._settings.image_max_side)
        chunk = image_chunk(record.source_id, record.uri, record.sha256, mtime_ns)
        self._commit(record, [chunk], self._embedder.embed_images([image]), is_new, report)
