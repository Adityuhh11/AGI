"""FlatStore: chunk metadata plus exact vector search, in one directory.

Directory layout: `meta.db` (SQLite, WAL) and `vectors.f32` (see `VectorFile`).

Invariants (mirrored in docs/ARCHITECTURE.md):
1. SQLite is the source of truth: a chunk exists iff it has a row in `chunks`. Vector rows
   without a `chunks` entry are garbage and may be overwritten.
2. Vectors are flushed to disk before the SQLite transaction that references them commits,
   so a committed chunk always points at a durable vector.
3. Every public method holds `self._lock`; the vector file is remapped when it grows, so no
   reader may overlap a writer.
4. Ranked output is ordered by (-score, id), so ties never depend on row placement.
"""

from __future__ import annotations

import heapq
import sqlite3
import threading
from collections.abc import Iterable, Iterator, Sequence
from pathlib import Path

import numpy as np

from agi_memory.store.catalog import Catalog
from agi_memory.store.filters import filter_clause
from agi_memory.store.identifiers import path_words, split_identifiers
from agi_memory.store.keyword import build_match
from agi_memory.store.schema import SCHEMA, SCHEMA_VERSION
from agi_memory.store.vectors import VectorFile, VectorFileError
from agi_memory.types import Chunk, ChunkKind, FileRecord, Filters, KeywordHit, VectorHit

_UNIT_TOLERANCE = 1e-3
_SQL_BATCH = 500  # far below SQLite's bound-variable limit

_UPSERT = """
INSERT INTO chunks (id, vec_row, source_id, uri, kind, locator, content_hash, mtime_ns,
                    text, identifiers, path_words)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
ON CONFLICT(id) DO UPDATE SET
    source_id = excluded.source_id, uri = excluded.uri, kind = excluded.kind,
    locator = excluded.locator, content_hash = excluded.content_hash,
    mtime_ns = excluded.mtime_ns, text = excluded.text, identifiers = excluded.identifiers,
    path_words = excluded.path_words
"""


class IndexMismatchError(Exception):
    """The index on disk was built with a different model, dimension or schema."""


def _batches[T](items: Sequence[T], size: int = _SQL_BATCH) -> Iterator[Sequence[T]]:
    for start in range(0, len(items), size):
        yield items[start : start + size]


def _reject_bare_string(ids: object) -> None:
    """A str is an iterable of characters: `delete(chunk.id)` would silently delete nothing."""
    if isinstance(ids, str):
        raise TypeError("expected a collection of ids, got a single string; wrap it in a list")


class FlatStore:
    """Exact-search store. One instance owns its directory."""

    def __init__(self, directory: Path, *, model_id: str, dim: int) -> None:
        directory.mkdir(parents=True, exist_ok=True)
        self._dim = dim
        self._lock = threading.RLock()
        self._db = sqlite3.connect(directory / "meta.db", check_same_thread=False)
        self._catalog = Catalog(self._db)
        try:
            self._init_db(model_id, dim)
            self._vectors = self._open_vectors(directory / "vectors.f32", dim)
            try:
                self._load_rows()
            except BaseException:
                self._vectors.close()
                raise
        except BaseException:
            self._db.close()
            raise

    def _open_vectors(self, path: Path, dim: int) -> VectorFile:
        """Open the vector file, never trusting a bad one to back chunks that already exist."""
        (indexed,) = self._db.execute("SELECT COUNT(*) FROM chunks").fetchone()
        if indexed == 0:
            # Nothing refers to the file, so a leftover (even one cut short by a crash while it
            # was first created) is garbage.
            path.unlink(missing_ok=True)
            return VectorFile(path, dim)
        if not path.exists():
            raise IndexMismatchError(
                f"vector file {path.name} is missing but {indexed} chunks are indexed: "
                "reindex required"
            )
        try:
            return VectorFile(path, dim)
        except VectorFileError as error:
            raise IndexMismatchError(
                f"vector file is unusable ({error}): reindex required"
            ) from error

    def _init_db(self, model_id: str, dim: int) -> None:
        self._db.execute("PRAGMA journal_mode=WAL")
        self._db.execute("PRAGMA synchronous=FULL")
        self._db.executescript(SCHEMA)
        stored = dict(self._db.execute("SELECT key, value FROM meta").fetchall())
        if not stored:
            with self._db:
                self._db.executemany(
                    "INSERT INTO meta VALUES (?, ?)",
                    [
                        ("schema_version", str(SCHEMA_VERSION)),
                        ("model_id", model_id),
                        ("dim", str(dim)),
                    ],
                )
        elif (
            stored.get("model_id") != model_id
            or stored.get("dim") != str(dim)
            or stored.get("schema_version") != str(SCHEMA_VERSION)
        ):
            built = (stored.get("model_id"), stored.get("dim"), stored.get("schema_version"))
            wanted = (model_id, str(dim), str(SCHEMA_VERSION))
            raise IndexMismatchError(
                f"index was built with (model, dim, schema) = {built}; "
                f"settings want {wanted}: reindex required"
            )

    def _load_rows(self) -> None:
        used = [row for (row,) in self._db.execute("SELECT vec_row FROM chunks")]
        self._high_water = max(used, default=-1) + 1  # rows [0, high_water) were ever allocated
        if self._high_water > self._vectors.capacity:
            raise IndexMismatchError(
                f"a chunk points at vector row {self._high_water - 1} but the vector file holds "
                f"only {self._vectors.capacity} rows: reindex required"
            )
        taken = set(used)
        self._free = [row for row in range(self._high_water) if row not in taken]
        heapq.heapify(self._free)  # already ascending, so already a valid heap
        self._live = np.zeros(self._vectors.capacity, dtype=bool)
        self._live[np.asarray(used, dtype=np.intp)] = True

    # --- validation -------------------------------------------------------------------

    def _validate_batch(self, chunks: Sequence[Chunk], vectors: np.ndarray) -> None:
        expected = (len(chunks), self._dim)
        if vectors.dtype != np.float32 or vectors.shape != expected:
            raise ValueError(
                f"vectors must be float32 with shape {expected}, "
                f"got {vectors.dtype} {vectors.shape}"
            )
        if len(chunks) == 0:
            return
        if not np.isfinite(vectors).all():
            raise ValueError("vectors contain NaN or infinity")
        if np.abs(np.linalg.norm(vectors, axis=1) - 1.0).max() > _UNIT_TOLERANCE:
            raise ValueError("vectors must be L2-normalised")
        ids = [chunk.id for chunk in chunks]
        if len(set(ids)) != len(ids):
            raise ValueError("duplicate chunk ids in one batch")

    # --- row bookkeeping --------------------------------------------------------------

    def _allocate_row(self) -> int:
        if self._free:
            return heapq.heappop(self._free)  # lowest free row first: deterministic reuse
        row = self._high_water
        self._high_water += 1
        return row

    def _sync_live_size(self) -> None:
        short = self._vectors.capacity - self._live.size
        if short > 0:
            self._live = np.concatenate([self._live, np.zeros(short, dtype=bool)])

    def _rows_for(self, ids: Sequence[str]) -> dict[str, int]:
        found: dict[str, int] = {}
        for batch in _batches(ids):
            marks = ",".join("?" * len(batch))
            query = f"SELECT id, vec_row FROM chunks WHERE id IN ({marks})"
            found.update(self._db.execute(query, list(batch)).fetchall())
        return found

    # --- public API -------------------------------------------------------------------

    def upsert(self, chunks: Sequence[Chunk], vectors: np.ndarray) -> None:
        """Insert or replace chunks and their vectors as one durable batch."""
        self._validate_batch(chunks, vectors)
        if len(chunks) == 0:
            return
        with self._lock:
            high_water_before = self._high_water
            existing = self._rows_for([chunk.id for chunk in chunks])
            rows: list[int] = []
            fresh: list[int] = []
            for chunk in chunks:
                row = existing.get(chunk.id)
                if row is None:
                    row = self._allocate_row()
                    fresh.append(row)
                rows.append(row)
            try:
                self._vectors.write_rows(rows, vectors)
                self._sync_live_size()
                self._vectors.flush()  # invariant 2: durable before metadata commits
                self._write_metadata(chunks, rows)
            except BaseException:
                # Give every row this batch took back: rows past the old high-water mark simply
                # stop existing (so `_live` is never shorter than the rows search looks at), and
                # rows taken from the free list return to it.
                self._high_water = high_water_before
                for row in fresh:
                    if row < high_water_before:
                        heapq.heappush(self._free, row)
                raise
            self._live[np.asarray(rows, dtype=np.intp)] = True

    def _write_metadata(self, chunks: Sequence[Chunk], rows: Sequence[int]) -> None:
        records = [
            (
                chunk.id,
                row,
                chunk.source_id,
                chunk.uri,
                chunk.kind.value,
                chunk.locator,
                chunk.content_hash,
                chunk.mtime_ns,
                chunk.text,
                split_identifiers(chunk.text),
                path_words(chunk.uri),
            )
            for chunk, row in zip(chunks, rows, strict=True)
        ]
        with self._db:
            self._db.executemany(_UPSERT, records)

    def delete(self, ids: Iterable[str]) -> int:
        """Remove chunks by id; returns how many existed."""
        _reject_bare_string(ids)
        with self._lock:
            rows = self._rows_for(list(ids))
            if not rows:
                return 0
            with self._db:
                self._db.executemany("DELETE FROM chunks WHERE id = ?", [(i,) for i in rows])
            self._live[np.asarray(list(rows.values()), dtype=np.intp)] = False
            for row in rows.values():
                heapq.heappush(self._free, row)
            return len(rows)

    def get(self, ids: Sequence[str]) -> dict[str, Chunk]:
        """Chunk metadata by id; ids that do not exist are absent from the result."""
        _reject_bare_string(ids)
        found: dict[str, Chunk] = {}
        with self._lock:
            for batch in _batches(ids):
                marks = ",".join("?" * len(batch))
                query = (
                    "SELECT id, source_id, uri, kind, locator, content_hash, mtime_ns, text "
                    f"FROM chunks WHERE id IN ({marks})"
                )
                for (
                    id_,
                    source_id,
                    uri,
                    kind,
                    locator,
                    content_hash,
                    mtime_ns,
                    text,
                ) in self._db.execute(query, list(batch)):
                    found[id_] = Chunk(
                        source_id=source_id,
                        uri=uri,
                        kind=ChunkKind(kind),
                        locator=locator,
                        content_hash=content_hash,
                        mtime_ns=mtime_ns,
                        text=text,
                    )
        return found

    def count(self) -> int:
        with self._lock:
            (total,) = self._db.execute("SELECT COUNT(*) FROM chunks").fetchone()
            return total

    # --- sources and files: change detection for incremental indexing ------------------

    def register_source(self, source_id: str, root: str) -> None:
        with self._lock:
            self._catalog.register_source(source_id, root)

    def source_root(self, source_id: str) -> str | None:
        """The folder a source was last indexed from; `None` if it was never registered."""
        with self._lock:
            return self._catalog.source_root(source_id)

    def mark_scanned(self, source_id: str, scanned_ns: int) -> None:
        with self._lock:
            self._catalog.mark_scanned(source_id, scanned_ns)

    def file_records(self, source_id: str) -> dict[str, FileRecord]:
        with self._lock:
            return self._catalog.file_records(source_id)

    def replace_file(
        self, record: FileRecord, chunks: Sequence[Chunk], vectors: np.ndarray
    ) -> None:
        """Make the index hold exactly `chunks` for this file, then remember the file.

        The file row is written last: a crash in between leaves the file looking stale, so the
        next sync re-indexes it (chunk ids are content-derived, so redoing it is harmless).
        """
        if any(c.source_id != record.source_id or c.uri != record.uri for c in chunks):
            raise ValueError(f"every chunk must belong to {record.source_id}:{record.uri}")
        with self._lock:
            # Forget the file first: if anything below is interrupted, the file looks new on the
            # next sync and is re-indexed, even if its bytes were reverted to the old content.
            self._catalog.delete_file_rows(record.source_id, [record.uri])
            self.upsert(chunks, vectors)
            keep = {chunk.id for chunk in chunks}
            stale = [
                chunk_id
                for chunk_id in self._catalog.chunk_ids_for_uri(record.source_id, record.uri)
                if chunk_id not in keep
            ]
            self.delete(stale)
            self._catalog.write_file(record)

    def touch_file(self, record: FileRecord) -> None:
        """Update a file's remembered size/mtime/hash when its content is known to be unchanged."""
        with self._lock:
            self._catalog.write_file(record)

    def remove_files(self, source_id: str, uris: Iterable[str]) -> int:
        """Forget files (and their chunks); returns how many file records were removed."""
        removed = 0
        with self._lock:
            for batch in _batches(list(uris)):
                self.delete(self._catalog.chunk_ids_for_uris(source_id, batch))
                removed += self._catalog.delete_file_rows(source_id, batch)
        return removed

    def delete_source(self, source_id: str) -> int:
        """Forget a whole source; returns how many chunks were removed."""
        with self._lock:
            ids = self._catalog.chunk_ids_for_source(source_id)
            self.delete(ids)
            self._catalog.delete_source_rows(source_id)
            return len(ids)

    def _validate_query(self, query: np.ndarray) -> np.ndarray:
        vec = np.asarray(query, dtype=np.float32)
        if vec.shape != (self._dim,):
            raise ValueError(f"query must have shape ({self._dim},), got {vec.shape}")
        if not np.isfinite(vec).all():
            raise ValueError("query contains NaN or infinity")
        return vec

    def _filter_mask(self, filters: Filters, size: int) -> np.ndarray:
        """Boolean mask over rows [0, size) for chunks that satisfy `filters`."""
        clause = filter_clause(filters)
        if clause is None:
            return np.ones(size, dtype=bool)
        where, params = clause
        mask = np.zeros(size, dtype=bool)
        rows = [
            row for (row,) in self._db.execute(f"SELECT vec_row FROM chunks WHERE {where}", params)
        ]
        mask[np.asarray(rows, dtype=np.intp)] = True
        return mask

    def _ids_for_rows(self, rows: Sequence[int]) -> dict[int, str]:
        found: dict[int, str] = {}
        for batch in _batches(rows):
            marks = ",".join("?" * len(batch))
            query = f"SELECT vec_row, id FROM chunks WHERE vec_row IN ({marks})"
            found.update(self._db.execute(query, list(batch)).fetchall())
        return found

    def search_vectors(
        self, query: np.ndarray, n: int, filters: Filters | None = None
    ) -> list[VectorHit]:
        """Exact top-`n` by dot product among live chunks matching `filters`.

        Ordered by (-score, id). Candidates tied with the n-th score are all kept before the
        final sort, so which of several equal scores make the cut never depends on row order.
        """
        vec = self._validate_query(query)
        with self._lock:
            size = self._high_water
            if size == 0 or n <= 0:
                return []
            allowed = self._live[:size].copy()
            if filters is not None:
                allowed &= self._filter_mask(filters, size)
            rows = np.flatnonzero(allowed)
            if rows.size == 0:
                return []
            scores = self._vectors.scores(vec, size)[rows]
            if scores.size > n:
                keep = scores >= np.partition(scores, -n)[-n]
                rows, scores = rows[keep], scores[keep]
            row_list = rows.tolist()
            ids = self._ids_for_rows(row_list)
        ranked = sorted(
            zip(row_list, scores.tolist(), strict=True), key=lambda p: (-p[1], ids[p[0]])
        )
        return [VectorHit(id=ids[row], score=score) for row, score in ranked[:n]]

    def keyword_search(
        self, query: str, n: int, filters: Filters | None = None
    ) -> list[KeywordHit]:
        """Top-`n` chunks by bm25 over text, identifiers and path words, ordered by (-score, id)."""
        match = build_match(query)
        if not match or n <= 0:
            return []
        sql = (
            "SELECT c.id, bm25(chunks_fts, 1.0, 0.5, 2.0) AS score FROM chunks_fts "
            "JOIN chunks c ON c.rid = chunks_fts.rowid WHERE chunks_fts MATCH ?"
        )
        params: list[object] = [match]
        clause = filter_clause(filters) if filters is not None else None
        if clause is not None:
            sql += f" AND ({clause[0]})"
            params.extend(clause[1])
        sql += " ORDER BY score, c.id LIMIT ?"
        params.append(n)
        with self._lock:
            rows = self._db.execute(sql, params).fetchall()
        return [KeywordHit(id=chunk_id, score=-score) for chunk_id, score in rows]

    def path_search(self, query: str, n: int, filters: Filters | None = None) -> list[KeywordHit]:
        """Top-`n` chunks whose path contains the whole query as a substring (case-insensitive).

        This is what finds a file by typing its name, or part of it. Every chunk of a matching
        file matches; shorter paths score higher. Queries under 3 characters match nothing.
        """
        needle = query.strip().replace('"', '""')
        if len(needle) < 3 or n <= 0:
            return []
        sql = (
            "SELECT chunks.id, m.score FROM (SELECT rowid AS rid, bm25(chunks_path_tri) AS score "
            "FROM chunks_path_tri WHERE chunks_path_tri MATCH ?) AS m "
            "JOIN chunks ON chunks.rid = m.rid"
        )
        params: list[object] = [f'"{needle}"']
        clause = filter_clause(filters) if filters is not None else None
        if clause is not None:
            sql += f" WHERE ({clause[0]})"
            params.extend(clause[1])
        sql += " ORDER BY m.score, chunks.id LIMIT ?"
        params.append(n)
        with self._lock:
            rows = self._db.execute(sql, params).fetchall()
        return [KeywordHit(id=chunk_id, score=-score) for chunk_id, score in rows]

    def close(self) -> None:
        """Release the database and the vector file (both are removable afterwards)."""
        with self._lock:
            try:
                self._vectors.close()
            finally:
                self._db.close()  # even if the vector file failed to close
