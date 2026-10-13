"""SQL for the `sources` and `files` tables (change detection) and per-file chunk lookups.

Pure SQL helpers over an open connection. They take no locks and never commit on their own
schedule beyond the explicit `with db:` blocks: `FlatStore` owns locking and ordering.
"""

from __future__ import annotations

import sqlite3
from collections.abc import Sequence

from agi_memory.types import FileRecord


class Catalog:
    def __init__(self, db: sqlite3.Connection) -> None:
        self._db = db

    def register_source(self, source_id: str, root: str) -> None:
        with self._db:
            self._db.execute(
                "INSERT INTO sources (id, root, last_scan_ns) VALUES (?, ?, 0) "
                "ON CONFLICT(id) DO UPDATE SET root = excluded.root",
                (source_id, root),
            )

    def source_root(self, source_id: str) -> str | None:
        row = self._db.execute("SELECT root FROM sources WHERE id = ?", (source_id,)).fetchone()
        return None if row is None else row[0]

    def mark_scanned(self, source_id: str, scanned_ns: int) -> None:
        with self._db:
            self._db.execute(
                "UPDATE sources SET last_scan_ns = ? WHERE id = ?", (scanned_ns, source_id)
            )

    def file_records(self, source_id: str) -> dict[str, FileRecord]:
        rows = self._db.execute(
            "SELECT uri, size, mtime_ns, sha256 FROM files WHERE source_id = ?", (source_id,)
        )
        return {
            uri: FileRecord(source_id, uri, size, mtime_ns, sha256)
            for uri, size, mtime_ns, sha256 in rows
        }

    def write_file(self, record: FileRecord) -> None:
        with self._db:
            self._db.execute(
                "INSERT INTO files (source_id, uri, size, mtime_ns, sha256) VALUES (?, ?, ?, ?, ?) "
                "ON CONFLICT(source_id, uri) DO UPDATE SET "
                "size = excluded.size, mtime_ns = excluded.mtime_ns, sha256 = excluded.sha256",
                (record.source_id, record.uri, record.size, record.mtime_ns, record.sha256),
            )

    def chunk_ids_for_uri(self, source_id: str, uri: str) -> list[str]:
        rows = self._db.execute(
            "SELECT id FROM chunks WHERE source_id = ? AND uri = ?", (source_id, uri)
        )
        return [chunk_id for (chunk_id,) in rows]

    def chunk_ids_for_uris(self, source_id: str, uris: Sequence[str]) -> list[str]:
        marks = ",".join("?" * len(uris))
        rows = self._db.execute(
            f"SELECT id FROM chunks WHERE source_id = ? AND uri IN ({marks})", [source_id, *uris]
        )
        return [chunk_id for (chunk_id,) in rows]

    def chunk_ids_for_source(self, source_id: str) -> list[str]:
        rows = self._db.execute("SELECT id FROM chunks WHERE source_id = ?", (source_id,))
        return [chunk_id for (chunk_id,) in rows]

    def delete_file_rows(self, source_id: str, uris: Sequence[str]) -> int:
        marks = ",".join("?" * len(uris))
        with self._db:
            cursor = self._db.execute(
                f"DELETE FROM files WHERE source_id = ? AND uri IN ({marks})", [source_id, *uris]
            )
        return cursor.rowcount

    def delete_source_rows(self, source_id: str) -> None:
        with self._db:
            self._db.execute("DELETE FROM files WHERE source_id = ?", (source_id,))
            self._db.execute("DELETE FROM sources WHERE id = ?", (source_id,))
