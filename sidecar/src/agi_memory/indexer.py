"""Indexer: brings a store in line with a folder, doing embedding work only for what changed.

Flow for one `sync`: scan the folder, diff against the files the store already knows, remove what
disappeared, then for every new or changed file read it once, hash it and chunk it. Text chunks
from many small files are embedded together (one GPU call has a ~200 ms fixed cost, so one call
per file would dominate indexing time), and so are image files, a few at a time, so the embedder
can keep two in flight; PDFs and videos are embedded per file because their pictures are made
lazily. Each file lands in the store atomically (`FlatStore.replace_file`).

Nothing is embedded twice: a chunk whose id is already stored keeps its vector (an id is derived
from everything its embedding is computed from), and a moved, renamed or copied file takes the
picture vectors of the identical file already indexed.

Index options (`IndexOptions`) say what a PDF page, a photo and a video become. The options of a
source's last finished sync are stored; when they change, every file of the kinds concerned is
processed again, even if it did not change on disk, and the reuse above makes that cheap.

Failure policy: a file that cannot be read, chunked or rendered, whatever the reason, is recorded
in the report and skipped, so one bad file never costs the rest of the folder. Files that are
plainly bad (unreadable, corrupt) are skipped however many there are; but a run of
`MAX_CONSECUTIVE_FAILURES` files in a row that fail in unexpected ways means the cause is not the
files (a failing GPU or disk), and the sync aborts with the last error. A failure while embedding
a batch of text chunks or images, or writing to the store, aborts at once. Everything committed so
far stays valid, and the next sync continues from there because file records are written last.
"""

from __future__ import annotations

import hashlib
import time
from collections.abc import Callable, Collection, Sequence
from dataclasses import dataclass, field
from pathlib import Path

import numpy as np
from PIL.Image import Image

from agi_memory.chunking.common import ChunkingError
from agi_memory.chunking.image import image_chunk, load_image
from agi_memory.chunking.pdf import PdfPages, pages_as_pictures, pdf_page_chunks, pdf_text_chunks
from agi_memory.chunking.text import NotTextError, chunk_text_document
from agi_memory.chunking.video import Video, video_frame_chunks, video_identity, video_name_chunk
from agi_memory.config import Settings
from agi_memory.embedding.base import Embedder
from agi_memory.sources.folder import ScannedFile, scan
from agi_memory.sources.rules import Route
from agi_memory.store import FlatStore
from agi_memory.types import LEGACY_OPTIONS, Chunk, ChunkKind, FileRecord, IndexOptions

_TEXT_ROUTES = frozenset({Route.CODE, Route.MARKDOWN, Route.TEXT})
MAX_CONSECUTIVE_FAILURES = 5


def _title(uri: str) -> str:
    """The file name: measured better as the embedding title than the whole path."""
    return uri.rsplit("/", 1)[-1]


def _changed_routes(before: IndexOptions, now: IndexOptions) -> frozenset[Route]:
    """The kinds of file whose chunks depend on an option that changed."""
    changed = set()
    if before.pdf_pages != now.pdf_pages:
        changed.add(Route.PDF)
    if before.photos != now.photos:
        changed.add(Route.IMAGE)
    if before.videos != now.videos:
        changed.add(Route.VIDEO)
    return frozenset(changed)


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
    reused: int = 0  # files that got the vectors of an identical indexed file instead of new ones
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


@dataclass(slots=True)
class _PendingImage:
    record: FileRecord
    chunk: Chunk
    is_new: bool
    image: Image


@dataclass(slots=True)
class _Run:
    """What one sync works with: its options, the kinds to redo, and what waits to be embedded."""

    source_id: str
    options: IndexOptions
    redo: frozenset[Route]  # kinds whose option changed: processed again even when unchanged
    donors: dict[str, str]  # content hash -> uri of an indexed file to take picture vectors from
    report: SyncReport
    pending: list[_Pending] = field(default_factory=list)
    images: list[_PendingImage] = field(default_factory=list)


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
        exclude: Collection[str] = (),
        options: IndexOptions | None = None,
        on_progress: Callable[[int, int], None] | None = None,
    ) -> SyncReport:
        """Index new and changed files under `root`, drop vanished ones, return what happened.

        `exclude` lists folders below the root to leave out (and to drop from the index if they
        were indexed before), as '/'-separated relative paths. `options` default to
        `IndexOptions()`."""
        if not root.is_dir():
            raise FileNotFoundError(f"source root is not a folder: {root}")
        options = options or IndexOptions()
        skipped = frozenset(f"{p.strip('/').lower()}/" for p in exclude if p.strip("/"))
        scanned = scan(root, self._settings, skipped)
        report = SyncReport(
            denied=scanned.denied,
            ignored=scanned.ignored,
            unsupported=scanned.unsupported,
            too_large=scanned.too_large,
        )
        report.errors.extend(FileError(uri, message) for uri, message in scanned.unreadable)
        self._store.register_source(source_id, str(root))
        known = self._store.file_records(source_id)
        before = self._store.source_options(source_id) or LEGACY_OPTIONS

        on_disk = {file.uri for file in scanned.files}
        protected = tuple(prefix for prefix, _ in scanned.unreadable)  # could not look in there
        gone = [u for u in sorted(known) if u not in on_disk and not u.startswith(protected)]
        run = _Run(
            source_id,
            options,
            _changed_routes(before, options),
            self._donors(known, gone, scanned.files),
            report,
        )

        total = len(scanned.files)
        failures = 0  # files in a row that failed
        for number, file in enumerate(scanned.files, start=1):
            try:
                self._index_file(run, file, known.get(file.uri))
                failures = 0
            except NotTextError:
                report.unsupported += 1
                failures = 0
            except (ChunkingError, OSError) as error:  # this file is the problem: skip it
                report.errors.append(FileError(file.uri, str(error)))
                failures = 0
            except Exception as error:  # a library failed on it, or something else is wrong
                report.errors.append(FileError(file.uri, f"{type(error).__name__}: {error}"))
                failures += 1
                if failures >= MAX_CONSECUTIVE_FAILURES:
                    raise
            if sum(len(item.chunks) for item in run.pending) >= self._settings.flush_chunks:
                self._flush(run)
            if len(run.images) >= self._settings.image_batch_size:
                self._flush_images(run)
            if on_progress is not None:
                on_progress(number, total)
        self._flush(run)
        self._flush_images(run)
        # After the files above, so a moved file's old chunks were still there to take vectors from.
        report.removed = self._store.remove_files(source_id, gone)
        self._store.mark_scanned(source_id, self._clock(), options)
        return report

    @staticmethod
    def _donors(
        known: dict[str, FileRecord], gone: list[str], scanned: list[ScannedFile]
    ) -> dict[str, str]:
        """Content hash -> uri of an indexed file whose chunks are safe to take vectors from.

        That is one that vanished (moved or renamed) or is still on disk unchanged. A file edited by
        this very sync is not: its old vectors are about to be replaced."""
        same = {
            f.uri
            for f in scanned
            if (record := known.get(f.uri))
            and (record.size, record.mtime_ns) == (f.size, f.mtime_ns)
        }
        usable = same | set(gone)
        donors: dict[str, str] = {}
        for uri in sorted(known):
            if uri in usable:
                donors.setdefault(known[uri].sha256, uri)
        return donors

    # --- per file -----------------------------------------------------------------------

    def _index_file(self, run: _Run, file: ScannedFile, known: FileRecord | None) -> None:
        redo = file.route in run.redo
        unchanged = known is not None and (known.size, known.mtime_ns) == (file.size, file.mtime_ns)
        if unchanged and not redo:
            run.report.unchanged += 1
            return
        if file.route is Route.VIDEO:
            data, digest = b"", video_identity(file.path, file.size)  # never read whole
        else:
            data = file.path.read_bytes()
            digest = hashlib.sha256(data).hexdigest()
        record = FileRecord(run.source_id, file.uri, file.size, file.mtime_ns, digest)
        if known is not None and known.sha256 == digest and not redo:
            self._store.touch_file(record)  # same bytes, new mtime: nothing to embed
            run.report.unchanged += 1
            return
        is_new = known is None
        donor = run.donors.get(digest) if is_new else None  # moved, renamed or copied: same bytes
        if file.route in _TEXT_ROUTES:
            chunks = chunk_text_document(
                run.source_id, file.uri, file.route, data, file.mtime_ns, self._settings
            )
            run.pending.append(_Pending(record, chunks, is_new))
        elif file.route is Route.PDF:
            self._index_pdf(run, record, data, is_new, donor)
        elif file.route is Route.VIDEO:
            self._index_video(run, record, file.path, is_new, donor)
        else:
            self._index_image(run, record, data, is_new, donor)

    def _flush_images(self, run: _Run) -> None:
        """Embed the pending image files in one call (the embedder keeps two in flight), then
        store each one."""
        if not run.images:
            return
        vectors = self._embedder.embed_images([item.image for item in run.images])
        for item, vector in zip(run.images, vectors, strict=True):
            self._commit(run, item.record, [item.chunk], vector[None, :], item.is_new)
        run.images.clear()

    def _flush(self, run: _Run) -> None:
        """Embed every pending text chunk in one call, then store each file atomically."""
        if not run.pending:
            return
        vectors = self._text_vectors([chunk for item in run.pending for chunk in item.chunks])
        offset = 0
        for item in run.pending:
            count = len(item.chunks)
            self._commit(
                run, item.record, item.chunks, vectors[offset : offset + count], item.is_new
            )
            offset += count
        run.pending.clear()

    def _text_vectors(self, chunks: list[Chunk]) -> np.ndarray:
        """Vectors for text chunks, in order: stored ones are reused, the rest embedded in one
        call."""
        vectors = self._store.vectors_for([chunk.id for chunk in chunks])
        missing = [chunk for chunk in chunks if chunk.id not in vectors]
        if missing:
            fresh = self._embedder.embed_documents(
                [chunk.text for chunk in missing], [_title(chunk.uri) for chunk in missing]
            )
            vectors.update(zip((chunk.id for chunk in missing), fresh, strict=True))
        if not chunks:
            return np.zeros((0, self._embedder.dim), dtype=np.float32)
        return np.stack([vectors[chunk.id] for chunk in chunks])

    def _picture_vectors(
        self,
        run: _Run,
        record: FileRecord,
        chunks: list[Chunk],
        donor: str | None,
        kind: ChunkKind,
        draw: Callable[[Sequence[int]], list[Image]],
    ) -> np.ndarray:
        """Vectors for picture chunks (PDF pages, video frames), in order. A picture's vector comes
        from its pixels alone, so the file's own stored chunks and an identical file's (`donor`,
        matched by locator) are reused; `draw` makes the pictures of the rest, a batch at a time,
        so a long file never sits in memory as bitmaps."""
        stored = self._store.vectors_for([chunk.id for chunk in chunks])
        reuse = self._store.vectors_of(record.source_id, donor, (kind,)) if donor else {}
        vectors = np.zeros((len(chunks), self._embedder.dim), dtype=np.float32)
        missing = []
        for index, chunk in enumerate(chunks):
            if chunk.id in stored:
                vectors[index] = stored[chunk.id]
            elif chunk.locator in reuse:
                vectors[index] = reuse[chunk.locator]
            else:
                missing.append(index)
        if reuse:
            run.report.reused += 1
        step = self._settings.image_batch_size
        for start in range(0, len(missing), step):
            batch = missing[start : start + step]
            vectors[batch] = self._embedder.embed_images(draw(batch))
        return vectors

    def _commit(
        self, run: _Run, record: FileRecord, chunks: list[Chunk], vectors: np.ndarray, is_new: bool
    ) -> None:
        self._store.replace_file(record, chunks, vectors)
        run.report.chunks_written += len(chunks)
        if is_new:
            run.report.added += 1
        else:
            run.report.updated += 1

    def _index_pdf(
        self, run: _Run, record: FileRecord, data: bytes, is_new: bool, donor: str | None
    ) -> None:
        with PdfPages(data) as pdf:
            text_chunks = pdf_text_chunks(
                record.source_id, record.uri, pdf, record.mtime_ns, self._settings
            )
            pages = pages_as_pictures(pdf, run.options.pdf_pages)
            page_chunks = pdf_page_chunks(
                record.source_id, record.uri, record.sha256, pages, record.mtime_ns
            )
            text_vectors = self._text_vectors(text_chunks)
            scale = self._settings.pdf_render_scale
            page_vectors = self._picture_vectors(
                run,
                record,
                page_chunks,
                donor,
                ChunkKind.PDF_PAGE,
                lambda batch: [pdf.render(pages[i] - 1, scale) for i in batch],
            )
        self._commit(
            run,
            record,
            text_chunks + page_chunks,
            np.concatenate([text_vectors, page_vectors]),
            is_new,
        )

    def _index_image(
        self, run: _Run, record: FileRecord, data: bytes, is_new: bool, donor: str | None
    ) -> None:
        by_name = run.options.photos == "name"
        chunk = image_chunk(
            record.source_id, record.uri, record.sha256, record.mtime_ns, by_name=by_name
        )
        if by_name:  # embedded from its path, with the text files
            run.pending.append(_Pending(record, [chunk], is_new))
            return
        if known := self._store.vectors_for([chunk.id]):  # this very chunk is stored already
            self._commit(run, record, [chunk], known[chunk.id][None, :], is_new)
            return
        stored = (
            self._store.vectors_of(record.source_id, donor, (ChunkKind.IMAGE,)) if donor else {}
        )
        if chunk.locator in stored:  # the same picture: its vector is already known
            run.report.reused += 1
            self._commit(run, record, [chunk], stored[chunk.locator][None, :], is_new)
            return
        image = load_image(data, self._settings.image_max_side)
        run.images.append(_PendingImage(record, chunk, is_new, image))

    def _index_video(
        self, run: _Run, record: FileRecord, path: Path, is_new: bool, donor: str | None
    ) -> None:
        """A video is found by its name; with `videos = content` also by eight of its frames. A
        video whose frames cannot be read keeps its name, and the problem is reported."""
        name = video_name_chunk(record.source_id, record.uri, record.sha256, record.mtime_ns)
        if run.options.videos == "name":
            run.pending.append(_Pending(record, [name], is_new))
            return
        frames: list[Chunk] = []
        frame_vectors = np.zeros((0, self._embedder.dim), dtype=np.float32)
        try:
            with Video(path) as video:
                times = video.frame_times()
                frames = video_frame_chunks(
                    record.source_id, record.uri, record.sha256, times, record.mtime_ns
                )
                side = self._settings.image_max_side
                frame_vectors = self._picture_vectors(
                    run,
                    record,
                    frames,
                    donor,
                    ChunkKind.VIDEO_FRAME,
                    lambda batch: [video.frame(times[i], side) for i in batch],
                )
        except ChunkingError as error:
            run.report.errors.append(FileError(record.uri, f"{error}; found by its name only"))
            frames, frame_vectors = [], frame_vectors[:0]
        self._commit(
            run,
            record,
            [name, *frames],
            np.concatenate([self._text_vectors([name]), frame_vectors]),
            is_new,
        )
