"""SQL for meta.db, schema version 3: index identity, chunk metadata, the keyword index (text,
identifiers and path words), a trigram index over paths and the source/file catalog.

`chunks.rid` is an explicit integer primary key (not the implicit rowid) so the external-content
FTS table keeps pointing at the right rows even if the database is ever VACUUMed.
"""

SCHEMA_VERSION = 3

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
    identifiers  TEXT NOT NULL,
    path_words   TEXT NOT NULL
);
CREATE INDEX IF NOT EXISTS chunks_source ON chunks(source_id);
CREATE INDEX IF NOT EXISTS chunks_source_uri ON chunks(source_id, uri);

CREATE TABLE IF NOT EXISTS sources (
    id           TEXT PRIMARY KEY,
    root         TEXT NOT NULL,
    last_scan_ns INTEGER NOT NULL
);

CREATE TABLE IF NOT EXISTS files (
    source_id TEXT NOT NULL,
    uri       TEXT NOT NULL,
    size      INTEGER NOT NULL,
    mtime_ns  INTEGER NOT NULL,
    sha256    TEXT NOT NULL,
    PRIMARY KEY (source_id, uri)
);

CREATE VIRTUAL TABLE IF NOT EXISTS chunks_fts USING fts5(
    text, identifiers, path_words,
    content='chunks', content_rowid='rid',
    tokenize='unicode61 remove_diacritics 2'
);

-- Per-term document counts of chunks_fts, so very common query terms can be skipped.
CREATE VIRTUAL TABLE IF NOT EXISTS chunks_vocab USING fts5vocab(chunks_fts, 'row');

-- Substring match on the path (file names typed whole or in part); case-insensitive.
CREATE VIRTUAL TABLE IF NOT EXISTS chunks_path_tri USING fts5(
    uri,
    content='chunks', content_rowid='rid',
    tokenize='trigram'
);

CREATE TRIGGER IF NOT EXISTS chunks_ai AFTER INSERT ON chunks BEGIN
    INSERT INTO chunks_fts(rowid, text, identifiers, path_words)
    VALUES (new.rid, new.text, new.identifiers, new.path_words);
    INSERT INTO chunks_path_tri(rowid, uri) VALUES (new.rid, new.uri);
END;
CREATE TRIGGER IF NOT EXISTS chunks_ad AFTER DELETE ON chunks BEGIN
    INSERT INTO chunks_fts(chunks_fts, rowid, text, identifiers, path_words)
    VALUES ('delete', old.rid, old.text, old.identifiers, old.path_words);
    INSERT INTO chunks_path_tri(chunks_path_tri, rowid, uri) VALUES ('delete', old.rid, old.uri);
END;
CREATE TRIGGER IF NOT EXISTS chunks_au AFTER UPDATE ON chunks BEGIN
    INSERT INTO chunks_fts(chunks_fts, rowid, text, identifiers, path_words)
    VALUES ('delete', old.rid, old.text, old.identifiers, old.path_words);
    INSERT INTO chunks_path_tri(chunks_path_tri, rowid, uri) VALUES ('delete', old.rid, old.uri);
    INSERT INTO chunks_fts(rowid, text, identifiers, path_words)
    VALUES (new.rid, new.text, new.identifiers, new.path_words);
    INSERT INTO chunks_path_tri(rowid, uri) VALUES (new.rid, new.uri);
END;
"""
