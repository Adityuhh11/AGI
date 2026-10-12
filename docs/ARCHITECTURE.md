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
| `embedding/` | `Embedder` contract; `GemmaEmbedder` (real), `FakeEmbedder` (tests) | built |
| `store/` | `FlatStore`: metadata + keyword index (SQLite) and exact vector search (memory-mapped file) | built |
| `chunking/`, `sources/`, `jobs.py` | Turn files into chunks and index them incrementally | phase 3 |
| `search/` | Vector + keyword search, rank fusion | phase 4 |
| `eval/` | Golden queries, metrics, baseline | phase 4 |
| `api/` | Authenticated localhost HTTP API | phase 5 |

## Data flow

Indexing: source file -> chunks -> embeddings (256-d unit vectors) -> `FlatStore.upsert`.
Search: query -> embedding + keyword query -> two ranked lists -> fusion -> hits.

## On-disk format

Index directory `%LOCALAPPDATA%\AGI\memory\` (tests and tools use `AGI_MEMORY_DIR`):

- `meta.db` (SQLite, WAL, `synchronous=FULL`), schema version 1: `meta(key, value)` holds `schema_version`,
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

## Invariants

1. **Determinism.** Chunk ids are content-derived; every ranked list breaks ties by chunk id; tests never depend on the GPU or the real model unless marked `model`.
2. **Privacy.** Nothing leaves the machine. `.env*`, keys and similar files are never read (deny list, phase 3).
3. **SQLite is the source of truth** for what exists in the index; vector rows without a metadata entry are garbage.
4. **GPU = models only.** Index and search live on the CPU.
5. **Embedder output** is float32, shape (n, 256), rows unit length to 1e-5. The GPU runs bf16, the CPU float32;
   measured cosine between them on 4 test sentences is >= 0.99995. Real-model tests are marked `model`
   (`uv run pytest -m model`); peak VRAM stayed under 1.5 GiB.

## Conventions

- One module = one job; docstrings say *why*; public behaviour has tests.
- Errors carry context and are never swallowed.
- Commits are small, conventional (`feat:`, `fix:`, `docs:`, `refactor:`, `chore:`, `test:`).
