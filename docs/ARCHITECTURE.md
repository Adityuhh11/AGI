# Architecture

Living document. Update it in the same commit as the code it describes.
Spec: `superpowers/specs/2026-10-12-memory-engine-design.md`.

## Overview

AGI is a local-first, AI-native launcher for Windows. Two processes cooperate:

- the **Go launcher** (later): hotkey, UI (Wails v3 + Svelte), calls the sidecar;
- the **Python sidecar** (`sidecar/`): the *memory engine* that indexes files and answers search queries.

They talk over authenticated localhost HTTP. The GPU hosts only the embedding model; index storage and search run on the CPU.

## Memory engine modules (`sidecar/src/agi_memory/`)

Dependencies point downward only. Status is `planned` or `built`.

| Module | Job | Status |
|---|---|---|
| `types.py` | Plain data: `Chunk`, `ChunkKind`, `Filters`, `VectorHit`, `chunk_id` | built |
| `config.py` | `Settings`: paths, model id, dimensions, batch sizes | built |
| `embedding/` | `Embedder` contract; `GemmaEmbedder` (real), `FakeEmbedder` (tests), `SerializedEmbedder` (lets the job thread and request threads share one model) | built |
| `store/` | `FlatStore`: metadata + keyword index (SQLite) and exact vector search (memory-mapped file) | built |
| `sources/` | `rules` (secret deny list, file routing), `folder` (gitignore-aware scanner, never follows links) | built |
| `chunking/` | Line-window and Markdown-section chunks, PDF pages (text + image), images | built |
| `indexer.py` | `Indexer.sync`: scan, diff against the files table, chunk, embed, store; embeds only what changed | built |
| `jobs.py` | `JobManager`: one background index job at a time, with progress | built |
| `search/` | `Searcher`: vector + FTS5 keyword search, reciprocal rank fusion, filters | built |
| `eval/` | Golden queries, metrics, runner/CLI (`uv run python -m agi_memory.eval`), baseline gate | built |
| `api/` | FastAPI app: bearer auth, host check, background model load, `/health` `/sources` `/jobs` `/search`; see `api.md` | built |
| `__main__.py` | `python -m agi_memory`: reads token and port from the environment, serves on `127.0.0.1` | built |

## Data flow

Indexing: source file -> chunks -> embeddings (256-d unit vectors) -> `FlatStore.upsert`.
Search: query -> embedding + keyword query -> two ranked lists -> fusion -> hits.
API: the Go launcher calls `POST /sources` (a background `JobManager` job runs the indexer) and `POST /search` over authenticated localhost HTTP; the contract is in `api.md`.

## On-disk format

Index directory `%LOCALAPPDATA%\AGI\memory\` (tests and tools use `AGI_MEMORY_DIR`):

- `meta.db` (SQLite, WAL, `synchronous=FULL`), schema version 2: `meta(key, value)` holds `schema_version`,
  `model_id` and `dim` (opening with different values raises `IndexMismatchError`: reindex required);
  `chunks(rid, id, vec_row, source_id, uri, kind, locator, content_hash, mtime_ns, text, identifiers)`;
  `chunks_fts` (FTS5 external-content over `text` and `identifiers`, kept in sync by triggers). The FTS table is
  keyed by `rid`, an explicit integer primary key, so it survives `VACUUM`.
- `vectors.f32`: 64-byte header (`AGIVEC01`, version, dim, capacity) then `capacity` rows of `dim` float32, row r
  at byte 64 + r*dim*4. Grows by doubling; freed rows are reused lowest-first. The file handle is unbuffered:
  on Windows a buffered handle makes `close()` fail after the file was mapped and fsynced.

Write order for a batch: vectors are written and flushed, then one SQLite transaction commits the metadata. A
vector row with no `chunks` entry is garbage and is ignored by search. All store methods hold one lock, because
growing the file remaps it. Search is one float32 matrix multiply on the CPU (4.3 ms at 100k, 47 ms at 1M measured
in ADR 0001), ties broken by chunk id. Crash safety is tested by killing a real process during an upsert, both
before the metadata commit and inside the vector flush (`tests/test_flat_crash.py`).

Opening is strict about the vector file. If chunks are indexed and `vectors.f32` is missing, unreadable, or too short
for a recorded row, `FlatStore` raises `IndexMismatchError` ("reindex required") rather than returning zero scores.
If nothing is indexed, any leftover vector file (including one cut short while first created) is recreated. A failed
growth (for example disk full) rolls the store back to its previous size and stays usable. `delete` and `get` reject a
bare string, which would otherwise be treated as a list of one-character ids.

Schema version 2 adds `sources(id, root, last_scan_ns)` and `files(source_id, uri, size, mtime_ns, sha256)`. `FlatStore.replace_file` upserts a file's new chunks, deletes its stale ones, and writes the `files` row last, so an interrupted sync leaves the file stale and it is redone. A v1 index is refused (`reindex required`); there is no v1 data to migrate.

## Invariants

1. **Determinism.** Chunk ids are content-derived; every ranked list breaks ties by chunk id; tests never depend on the GPU or the real model unless marked `model`.
2. **Privacy.** Nothing leaves the machine. `.env*`, keys and similar files are never read (deny list in `sources/rules.py`).
3. **SQLite is the source of truth** for what exists in the index; vector rows without a metadata entry are garbage.
4. **GPU = models only.** Index and search live on the CPU.
5. **Embedder output** is float32, shape (n, 256), rows unit length to 1e-5. The GPU runs bf16, the CPU float32;
   measured cosine between them on 4 test sentences is >= 0.99995. Real-model tests are marked `model`
   (`uv run pytest -m model`); peak VRAM stayed under 1.5 GiB.
6. **Sync safety.** Denied files are never opened; links are never followed; a directory that cannot be read never causes its indexed files to be deleted; a missing source root is an error.
7. **Retrieval quality is gated.** `docs/eval/baseline.json` holds the accepted metrics; `uv run python -m agi_memory.eval` fails if any metric drops more than 0.02 or a `forbid` query surfaces a file it must not.
8. **The API is local and authenticated.** It binds 127.0.0.1, rejects foreign `Host` headers before anything else, and refuses to start without a token of at least 16 characters; every route including `/health` needs the token.
9. **The API never blocks on the model.** The port opens immediately; `/health` reports `ready: false` while the model loads and every other route answers 503.

## Conventions

- One module = one job; docstrings say *why*; public behaviour has tests.
- Errors carry context and are never swallowed.
- Commits are small, conventional (`feat:`, `fix:`, `docs:`, `refactor:`, `chore:`, `test:`).
