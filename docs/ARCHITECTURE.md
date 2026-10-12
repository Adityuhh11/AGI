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
| `embedding/` | `Embedder` contract; `GemmaEmbedder` (real), `FakeEmbedder` (tests) | planned |
| `store/` | `FlatStore`: metadata + keyword index (SQLite) and exact vector search (memory-mapped file) | planned |
| `chunking/`, `sources/`, `jobs.py` | Turn files into chunks and index them incrementally | phase 3 |
| `search/` | Vector + keyword search, rank fusion | phase 4 |
| `eval/` | Golden queries, metrics, baseline | phase 4 |
| `api/` | Authenticated localhost HTTP API | phase 5 |

## Data flow

Indexing: source file -> chunks -> embeddings (256-d unit vectors) -> `FlatStore.upsert`.
Search: query -> embedding + keyword query -> two ranked lists -> fusion -> hits.

## On-disk format

Filled in as each piece is built (see the store section after phase 2).

## Invariants

1. **Determinism.** Chunk ids are content-derived; every ranked list breaks ties by chunk id; tests never depend on the GPU or the real model unless marked `model`.
2. **Privacy.** Nothing leaves the machine. `.env*`, keys and similar files are never read (deny list, phase 3).
3. **SQLite is the source of truth** for what exists in the index; vector rows without a metadata entry are garbage.
4. **GPU = models only.** Index and search live on the CPU.

## Conventions

- One module = one job; docstrings say *why*; public behaviour has tests.
- Errors carry context and are never swallowed.
- Commits are small, conventional (`feat:`, `fix:`, `docs:`, `refactor:`, `chore:`, `test:`).
