# Architecture

Living document. Update it in the same commit as the code it describes.
Spec: `superpowers/specs/2026-10-12-memory-engine-design.md`.

## Overview

AGI is a local-first, AI-native launcher for Windows. Two processes cooperate:

- the **Go launcher** (`cmd/agi`, `internal/`): hotkey, tray, indexing manager, UI (Wails v3 + Svelte), calls the sidecar;
- the **Python sidecar** (`sidecar/`): the *memory engine* that indexes files and answers search queries.

They talk over authenticated localhost HTTP. The GPU hosts only the embedding model, which runs in a llama.cpp server the
engine owns (about 1.1 GB of VRAM); index storage and search run on the CPU. The engine is installed under
`%LOCALAPPDATA%\AGI\engine\` and is optional: the launcher works without it (see `superpowers/specs/2026-10-18-packaging-design.md`).

## Memory engine modules (`sidecar/src/agi_memory/`)

Dependencies point downward only. Status is `planned` or `built`.

| Module | Job | Status |
|---|---|---|
| `types.py` | Plain data: `Chunk`, `ChunkKind`, `Filters`, `VectorHit`, `chunk_id` | built |
| `config.py` | `Settings`: paths, model id, dimensions, batch sizes | built |
| `embedding/` | `Embedder` contract; `LlamaEmbedder` (real: EmbeddingGemma 2, Q8_0 GGUF, through a `llama-server` that `LlamaServer` starts on a loopback port behind an API key, on the discrete GPU through Vulkan or on the CPU; ADR 0004), `FakeEmbedder` (tests), `SerializedEmbedder` (lets the job thread and request threads share one model; bulk calls run in small slices and queries go first). Images go to the server two at a time, which overlaps its image decoding with the GPU | built |
| `store/` | `FlatStore`: metadata + keyword index (SQLite) and exact vector search (memory-mapped file) | built |
| `sources/` | `rules` (secret deny list, file routing), `folder` (gitignore-aware scanner, never follows links) | built |
| `chunking/` | Line-window and Markdown-section chunks, PDF pages (text + image), images | built |
| `indexer.py` | `Indexer.sync`: scan, diff against the files table, chunk, embed, store; embeds only what changed, and a moved, renamed or copied file takes its image and PDF-page vectors from the identical file already indexed | built |
| `jobs.py` | `JobManager`: one background index job at a time, with progress | built |
| `search/` | `Searcher`: vector, bm25 and path search (the last two while the query is embedded) blended into one score (`fusion.py`), `confident` flag against a noise ceiling the index measures itself (`calibration.py`), filters | built |
| `eval/` | Golden queries, metrics, runner/CLI (`uv run python -m agi_memory.eval`), baseline gate | built |
| `api/` | FastAPI app: bearer auth, host check, background model load, `/health` `/sources` `/jobs` `/search`; see `api.md` | built |
| `__main__.py` | `python -m agi_memory`: reads token and port from the environment, serves on `127.0.0.1` | built |

## Data flow

Indexing: source file -> chunks -> embeddings (768-d unit vectors; text is embedded with its file name as title) -> `FlatStore.upsert`.
Search: query -> embedding, bm25 query (words, identifiers, path words, phrases) and path substring -> three candidate lists -> one blended score (`cosine + 0.02 x relative bm25 + 0.2 if the query is in the path`) -> hits plus a `confident` flag.
API: the Go launcher calls `POST /sources` (a background `JobManager` job runs the indexer) and `POST /search` over authenticated localhost HTTP; the contract is in `api.md`.

## On-disk format

Index directory `%LOCALAPPDATA%\AGI\memory\` (tests and tools use `AGI_MEMORY_DIR`):

- `meta.db` (SQLite, WAL, `synchronous=FULL`), schema version 3: `meta(key, value)` holds `schema_version`,
  `model_id` and `dim` (opening with different values raises `IndexMismatchError`: reindex required);
  `chunks(rid, id, vec_row, source_id, uri, kind, locator, content_hash, mtime_ns, text, identifiers, path_words)`;
  `chunks_fts` (FTS5 external-content over `text`, `identifiers` and `path_words`, bm25 weights 1.0 / 0.5 / 2.0, kept in
  sync by triggers) and `chunks_path_tri` (FTS5 trigram over `uri`: substring match on file names and paths). The FTS table is
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

Schema version 3 adds `path_words` and `chunks_path_tri`; version 2 added `sources(id, root, last_scan_ns)` and `files(source_id, uri, size, mtime_ns, sha256)`. `FlatStore.replace_file` upserts a file's new chunks, deletes its stale ones, and writes the `files` row last, so an interrupted sync leaves the file stale and it is redone. An index from another schema version, model or dimension is refused (`reindex required`); there is nothing to migrate.

## Invariants

1. **Determinism.** Chunk ids are content-derived; every ranked list breaks ties by chunk id; tests never depend on the GPU or the real model unless marked `model`.
2. **Privacy.** Nothing leaves the machine. `.env*`, keys and similar files are never read (deny list in `sources/rules.py`).
3. **SQLite is the source of truth** for what exists in the index; vector rows without a metadata entry are garbage.
4. **GPU = models only.** Index and search live on the CPU.
5. **Embedder output** is float32, shape (n, 768), rows unit length to 1e-5. The model is the Q8_0 GGUF of EmbeddingGemma 2;
   measured against the torch bf16 model it replaced, vectors agree to cosine 0.9999 (text) and 0.994 or better (images and
   pages), the golden set gives identical results on the GPU and on the CPU, and an index built with torch answers the same
   under the new query vectors (`eval/2026-10-18-llamacpp-embedder.md`). Real-model tests are marked `model`
   (`AGI_LLAMA_SERVER` and `AGI_MODEL_DIR` set; `uv run pytest -m model`).
6. **Sync safety.** Denied files are never opened; links are never followed; a directory that cannot be read never causes its indexed files to be deleted; a missing source root is an error.
7. **Retrieval quality is gated.** `docs/eval/baseline.json` holds the accepted metrics; `uv run python -m agi_memory.eval` fails if any metric drops more than 0.02 or a `forbid` query surfaces a file it must not.
8. **The API is local and authenticated.** It binds 127.0.0.1, rejects foreign `Host` headers before anything else, and refuses to start without a token of at least 16 characters; every route including `/health` needs the token.
9. **The API never blocks on the model.** The port opens immediately; `/health` reports `ready: false` while the model loads and every other route answers 503.

10. **One score, comparable across kinds.** Text, PDF pages and images are ranked by cosine plus small boosts, never by rank fusion (an image has no keyword text and lost every fusion). Weights and the confidence threshold live in `search/fusion.py` and are measured by the eval.
11. **An index is tied to its model, dimension and schema.** Documents are embedded with their file name as title; changing the model, the dimension, the schema or the document format means rebuilding the index (open refuses a mismatch).

## Conventions

- One module = one job; docstrings say *why*; public behaviour has tests.
- Errors carry context and are never swallowed.
- Commits are small, conventional (`feat:`, `fix:`, `docs:`, `refactor:`, `chore:`, `test:`).
