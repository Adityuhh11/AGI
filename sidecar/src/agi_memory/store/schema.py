"""SQL for meta.db, schema version 1: index identity, chunk metadata and the keyword index.

`chunks.rid` is an explicit integer primary key (not the implicit rowid) so the external-content
FTS table keeps pointing at the right rows even if the database is ever VACUUMed.
"""

SCHEMA_VERSION = 1

SCHEMA = """
CREATE TABLE IF NOT EXISTS meta (
    key   TEXT PRIMARY KEY,
    value TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS chunks (
    rid          INTEGER PRIMARY KEY,
    id           TEXT NOT NULL UNIQUE,
    vec_row      INTEGER NOT NULL UNIQUE,
    source_id    TEXT NOT NULL,
    uri          TEXT NOT NULL,
    kind         TEXT NOT NULL,
    locator      TEXT NOT NULL,
    content_hash TEXT NOT NULL,
    mtime_ns     INTEGER NOT NULL,
    text         TEXT NOT NULL,
    identifiers  TEXT NOT NULL
);
CREATE INDEX IF NOT EXISTS chunks_source ON chunks(source_id);

CREATE VIRTUAL TABLE IF NOT EXISTS chunks_fts USING fts5(
    text, identifiers,
    content='chunks', content_rowid='rid',
    tokenize='unicode61 remove_diacritics 2'
);

CREATE TRIGGER IF NOT EXISTS chunks_ai AFTER INSERT ON chunks BEGIN
    INSERT INTO chunks_fts(rowid, text, identifiers)
    VALUES (new.rid, new.text, new.identifiers);
END;
CREATE TRIGGER IF NOT EXISTS chunks_ad AFTER DELETE ON chunks BEGIN
    INSERT INTO chunks_fts(chunks_fts, rowid, text, identifiers)
    VALUES ('delete', old.rid, old.text, old.identifiers);
END;
CREATE TRIGGER IF NOT EXISTS chunks_au AFTER UPDATE ON chunks BEGIN
    INSERT INTO chunks_fts(chunks_fts, rowid, text, identifiers)
    VALUES ('delete', old.rid, old.text, old.identifiers);
    INSERT INTO chunks_fts(rowid, text, identifiers)
    VALUES (new.rid, new.text, new.identifiers);
END;
"""
