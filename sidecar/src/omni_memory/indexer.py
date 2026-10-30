"""Indexer: brings a store in line with a folder, doing embedding work only for what changed.

Flow for one `sync`: scan the folder, diff against the files the store already knows, remove what
disappeared, then for every new or changed file read it once, hash it and chunk it. Text chunks
from many small files are embedded together (one GPU call has a ~200 ms fixed cost, so one call
per file would dominate indexing time), and so are image files, a few at a time, so the embedder
can keep two in flight; PDFs and videos are embedded per file because their pictures are made
lazily, and so are audio files and soundtracks, whose 30 s windows are decoded as a stream.
Each file lands in the store atomically (`FlatStore.replace_file`).

Nothing is embedded twice: a chunk whose id is already stored keeps its vector (an id is derived
from everything its embedding is computed from), and a moved, renamed or copied file takes the
picture vectors of the identical file already indexed.

Index options (`IndexOptions`) say what a PDF page, a photo, a video and a sound become. When they
change, every file of the kinds concerned is processed again, even if it did not change on
disk, and the reuse above makes that cheap. The options and the kinds still to redo are stored
before the first file is touched and the redo is cleared when the sync finishes, so a sync that
stops part-way (and options changed back meanwhile) never leaves files in the wrong mode.

Failure policy: a file that cannot be read, chunked or rendered, whatever the reason, is recorded
in the report and skipped, so one bad file never costs the rest of the folder. Files that are
plainly bad (unreadable, corrupt) are skipped however many there are; but a run of
`MAX_CONSECUTIVE_FAILURES` files in a row that fail in unexpected ways means the cause is not the
files (a failing GPU or disk), and the sync aborts with the last error. A batch of images that
fails is tried again one picture at a time, so a picture the model refuses is that file's error;
all of them failing, a failure while embedding a batch of text chunks, or a failure writing to the
store aborts at once. Everything committed so far stays valid, and the next sync continues from
there because file records are written last.
"""

from __future__ import annotations

import hashlib
import time
from collections.abc import Callable, Collection, Sequence
from dataclasses import dataclass, field
from pathlib import Path, PurePosixPath

import numpy as np
from PIL.Image import Image

from omni_memory.chunking.audio import AudioTrack, audio_name_chunk, audio_window_chunk
from omni_memory.chunking.common import ChunkingError, media_identity
from omni_memory.chunking.image import image_chunk, load_image
from omni_memory.chunking.name import file_name_chunk, name_identity
from omni_memory.chunking.office import office_text
from omni_memory.chunking.pdf import PdfPages, pages_as_pictures, pdf_page_chunks, pdf_text_chunks
from omni_memory.chunking.text import NotTextError, chunk_text_document
from omni_memory.chunking.video import Video, video_frame_chunks, video_name_chunk
from omni_memory.config import Settings
from omni_memory.embedding.base import Embedder
from omni_memory.sources.folder import ScannedFile, excluded_folders, scan
from omni_memory.sources.rules import Route
from omni_memory.store import FlatStore
from omni_memory.types import LEGACY_OPTIONS, Chunk, ChunkKind, FileRecord, IndexOptions

_TEXT_ROUTES = frozenset({Route.CODE, Route.MARKDOWN, Route.TEXT})
MAX_CONSECUTIVE_FAILURES = 5


def _title(uri: str) -> str:
    """The file name: measured better as the embedding title than the whole path."""
    return uri.rsplit("/", 1)[-1]


def _changed_routes(before: IndexOptions, now: IndexOptions) -> frozenset[Route]:
    """The kinds of file whose chunks depend on an option that changed."""
    changed: set[Route] = set()
    if before.pdf_pages != now.pdf_pages:
        changed.add(Route.PDF)
    if before.photos != now.photos:
        changed.add(Route.IMAGE)
    if before.videos != now.videos:
        changed.add(Route.VIDEO)
    if before.audio != now.audio:  # a video's soundtrack is sound too
        changed.update((Route.AUDIO, Route.VIDEO))
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
        scanned = scan(root, self._settings, excluded_folders(exclude))
        report = SyncReport(
            denied=scanned.denied,
            ignored=scanned.ignored,
            unsupported=scanned.unsupported,
            too_large=scanned.too_large,
        )
        report.errors.extend(FileError(uri, message) for uri, message in scanned.unreadable)
        self._store.register_source(source_id, str(root))
        known = self._store.file_records(source_id)
        # An index without options predates audio search too: its audio files are plain `file`
        # chunks.
        before, unfinished = self._store.source_options(source_id) or (
            LEGACY_OPTIONS,
            frozenset({Route.AUDIO.value}),
        )
        # Kinds whose option changed, plus those an interrupted sync left half done: recorded
        # before any file is touched, so stopping part-way never strands files in the wrong mode.
        redo = _changed_routes(before, options) | {Route(route) for route in unfinished}
        self._store.begin_options(source_id, options, redo)

        on_disk = {file.uri for file in scanned.files}
        protected = tuple(prefix for prefix, _ in scanned.unreadable)  # could not look in there
        gone = [u for u in sorted(known) if u not in on_disk and not u.startswith(protected)]
        run = _Run(source_id, options, redo, self._donors(known, gone, scanned.files), report)

        # Newest first, so what a person worked on last is searchable first: "search works while it
        # indexes" then holds for what they are most likely to look for. The uri makes the order
        # deterministic.
        order = sorted(scanned.files, key=lambda f: (-f.mtime_ns, f.uri))
        total = len(order)
        failures = 0  # files in a row that failed
        for number, file in enumerate(order, start=1):
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
        if file.route is Route.VIDEO or (
            file.route is Route.AUDIO and run.options.audio == "content"
        ):
            data, digest = b"", media_identity(file.path, file.size)  # never read whole
        elif file.route in (Route.AUDIO, Route.FILE):
            # By name the file is not opened at all (a cloud-only OneDrive file would be fetched to
            # read its ends), and its size notices a move.
            data, digest = b"", name_identity(file.size)
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
        elif file.route is Route.AUDIO:
            self._index_audio(run, record, file.path, is_new, donor)
        elif file.route is Route.OFFICE:
            self._index_office(run, record, file, data, is_new)
        elif file.route is Route.FILE:
            chunk = file_name_chunk(run.source_id, file.uri, digest, file.mtime_ns)
            run.pending.append(_Pending(record, [chunk], is_new))
        else:
            self._index_image(run, record, data, is_new, donor)

    def _index_office(
        self, run: _Run, record: FileRecord, file: ScannedFile, data: bytes, is_new: bool
    ) -> None:
        """An Office file is searched by its text; one with none (pictures only) or that cannot be
        unpacked keeps its name, and the second is reported."""
        chunks: list[Chunk] = []
        try:
            text = office_text(
                data, PurePosixPath(file.uri).suffix.lower(), self._settings.max_text_file_bytes
            )
            chunks = chunk_text_document(
                run.source_id, file.uri, Route.TEXT, text, file.mtime_ns, self._settings
            )
        except ChunkingError as error:
            run.report.errors.append(FileError(file.uri, f"{error}; found by its name only"))
        if not chunks:
            chunks = [file_name_chunk(run.source_id, file.uri, record.sha256, file.mtime_ns)]
        run.pending.append(_Pending(record, chunks, is_new))

    def _flush_images(self, run: _Run) -> None:
        """Embed the pending image files in one call (the embedder keeps two in flight), then
        store each one."""
        if not run.images:
            return
        images, run.images = run.images, []
        try:
            vectors = self._embedder.embed_images([item.image for item in images])
            done = list(zip(images, vectors, strict=True))
        except Exception:
            done = self._images_one_by_one(run, images)
        for item, vector in done:
            self._commit(run, item.record, [item.chunk], vector[None, :], item.is_new)

    def _images_one_by_one(
        self, run: _Run, images: list[_PendingImage]
    ) -> list[tuple[_PendingImage, np.ndarray]]:
        """After a batch failed: one picture the server refuses must not cost the others, so each
        is embedded alone and a refused one is that file's error. When several were tried and all
        failed, the cause is not the pictures (the GPU or the server): that aborts the sync."""
        done: list[tuple[_PendingImage, np.ndarray]] = []
        last: Exception | None = None
        for item in images:
            try:
                done.append((item, self._embedder.embed_images([item.image])[0]))
            except Exception as error:
                last = error
                run.report.errors.append(
                    FileError(item.record.uri, f"{type(error).__name__}: {error}")
                )
        if not done and len(images) > 1 and last is not None:
            raise last
        return done

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
        matched by content hash) are reused; `draw` makes the pictures of the rest, a batch at a
        time, so a long file never sits in memory as bitmaps."""
        stored = self._store.vectors_for([chunk.id for chunk in chunks])
        reuse = self._store.vectors_of(record.source_id, donor, (kind,)) if donor else {}
        vectors = np.zeros((len(chunks), self._embedder.dim), dtype=np.float32)
        missing = []
        reused = False
        for index, chunk in enumerate(chunks):
            if chunk.id in stored:
                vectors[index] = stored[chunk.id]
            elif chunk.content_hash in reuse:
                vectors[index] = reuse[chunk.content_hash]
                reused = True
            else:
                missing.append(index)
        if reused:
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
        if chunk.content_hash in stored:  # the same picture, embedded the same way
            run.report.reused += 1
            self._commit(run, record, [chunk], stored[chunk.content_hash][None, :], is_new)
            return
        image = load_image(data, self._settings.image_max_side)
        run.images.append(_PendingImage(record, chunk, is_new, image))

    def _index_video(
        self, run: _Run, record: FileRecord, path: Path, is_new: bool, donor: str | None
    ) -> None:
        """A video is found by its name; with `videos = content` also by eight of its frames, and
        with `audio = content` also by its soundtrack. What cannot be read keeps the name, and the
        problem is reported."""
        name = video_name_chunk(record.source_id, record.uri, record.sha256, record.mtime_ns)
        if run.options.videos == "name" and run.options.audio == "name":
            run.pending.append(_Pending(record, [name], is_new))
            return
        empty = np.zeros((0, self._embedder.dim), dtype=np.float32)
        frames, frame_vectors = [], empty
        if run.options.videos == "content":
            frames, frame_vectors = self._video_frames(run, record, path, donor)
        sound, sound_vectors = [], empty
        if run.options.audio == "content":
            sound, sound_vectors = self._sound(
                run, record, path, donor, required=False, kept="found without its sound"
            )
        self._commit(
            run,
            record,
            [name, *frames, *sound],
            np.concatenate([self._text_vectors([name]), frame_vectors, sound_vectors]),
            is_new,
        )

    def _video_frames(
        self, run: _Run, record: FileRecord, path: Path, donor: str | None
    ) -> tuple[list[Chunk], np.ndarray]:
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
            # With the sound searched too, the file may be found by more than its name.
            kept = (
                "found by its name only"
                if run.options.audio == "name"
                else "its frames are not searched"
            )
            run.report.errors.append(FileError(record.uri, f"{error}; {kept}"))
            frames, frame_vectors = [], frame_vectors[:0]
        return frames, frame_vectors

    def _index_audio(
        self, run: _Run, record: FileRecord, path: Path, is_new: bool, donor: str | None
    ) -> None:
        """An audio file is found by its name; with `audio = content` also by 30 s windows of what
        it sounds like. A file that cannot be decoded keeps its name, and the problem is
        reported."""
        name = audio_name_chunk(record.source_id, record.uri, record.sha256, record.mtime_ns)
        if run.options.audio == "name":
            run.pending.append(_Pending(record, [name], is_new))
            return
        windows, vectors = self._sound(
            run, record, path, donor, required=True, kept="found by its name only"
        )
        self._commit(
            run,
            record,
            [name, *windows],
            np.concatenate([self._text_vectors([name]), vectors]),
            is_new,
        )

    def _sound(
        self,
        run: _Run,
        record: FileRecord,
        path: Path,
        donor: str | None,
        *,
        required: bool,
        kept: str,
    ) -> tuple[list[Chunk], np.ndarray]:
        """The windows of a file's sound and their vectors. A window whose chunk is stored already
        keeps its vector, and so does one of an identical file (`donor`, by content hash); the rest
        are embedded a few at a time while the track is decoded, so a long recording never sits in
        memory. `required` is false for a video, which may simply have no sound. A track that cannot
        be read is reported with `kept` saying what the file is found by instead."""
        none = np.zeros((0, self._embedder.dim), dtype=np.float32)
        chunks: list[Chunk] = []
        vectors: list[np.ndarray] = []
        reuse = (
            self._store.vectors_of(record.source_id, donor, (ChunkKind.AUDIO_WINDOW,))
            if donor
            else {}
        )
        batch: list[tuple[Chunk, np.ndarray]] = []
        reused = False
        try:
            with AudioTrack(path) as track:
                if not track.present:
                    if required:
                        raise ChunkingError("the file has no sound")
                    return [], none
                for seconds, samples in track.windows():
                    chunk = audio_window_chunk(
                        record.source_id, record.uri, record.sha256, seconds, record.mtime_ns
                    )
                    batch.append((chunk, samples))
                    if len(batch) >= self._settings.audio_batch_size:
                        reused |= self._embed_sound(batch, reuse, chunks, vectors)
                reused |= self._embed_sound(batch, reuse, chunks, vectors)
        except ChunkingError as error:
            run.report.errors.append(FileError(record.uri, f"{error}; {kept}"))
            return [], none
        if reused:
            run.report.reused += 1
        return (chunks, np.stack(vectors)) if vectors else ([], none)  # none: silence, or under 1 s

    def _embed_sound(
        self,
        batch: list[tuple[Chunk, np.ndarray]],
        reuse: dict[str, np.ndarray],
        chunks: list[Chunk],
        vectors: list[np.ndarray],
    ) -> bool:
        """Appends the batch's chunks and vectors, embedding only what is neither stored nor
        taken from the donor; empties the batch. True if the donor gave a vector."""
        if not batch:
            return False
        stored = self._store.vectors_for([chunk.id for chunk, _ in batch])
        rows: list[np.ndarray | None] = [None] * len(batch)
        missing: list[int] = []
        reused = False
        for index, (chunk, _) in enumerate(batch):
            if chunk.id in stored:
                rows[index] = stored[chunk.id]
            elif chunk.content_hash in reuse:
                rows[index] = reuse[chunk.content_hash]
                reused = True
            else:
                missing.append(index)
        if missing:
            fresh = self._embedder.embed_audio([batch[i][1] for i in missing])
            for index, vector in zip(missing, fresh, strict=True):
                rows[index] = vector
        chunks.extend(chunk for chunk, _ in batch)
        vectors.extend(row for row in rows if row is not None)
        batch.clear()
        return reused
