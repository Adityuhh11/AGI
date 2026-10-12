# Status

Updated at the end of every phase. Last updated: 2026-10-12.

| Phase | Scope | State |
|---|---|---|
| 0 Foundations | Package, tooling, settings, docs scaffold | done |
| 1 Embedding | `Embedder`, `FakeEmbedder`, `GemmaEmbedder` | done |
| 2 Store | `FlatStore`: metadata, FTS, exact search, crash safety | done |
| 3 Ingestion | Deny list, scanner, chunkers, incremental sync (jobs move to phase 5) | done |
| 4 Search + eval | Keyword + vector fusion, eval harness, baseline | planned |
| 5 API | Authenticated FastAPI app | not planned yet |
| 6 Quality iteration | tree-sitter chunking, rerankers, "no good answer" | ideas only |

## Next

Phase 4 tasks in `superpowers/plans/2026-10-12-memory-engine-phases-3-4.md`; then plan phase 5 (API).

## Known gaps and deviations from the spec

- Chunk id also hashes `source_id` (spec said uri + locator + content hash).
- SQLite column is `vec_row`; `chunks` has an integer primary key `rid` (VACUUM-safe FTS mapping).
- GPU (bf16) vs CPU (fp32) embeddings agree to cosine >= 0.99995 on the 4-sentence test set.
- `sources`/`files` tables arrived in phase 3 (schema version 2); the `Store` protocol is extracted in phase 4.
- Image chunks carry their file path as `text` (spec said empty) so file names are keyword-searchable.
- `jobs.py` (background wrapper) moves to phase 5 with the API; phase 3 delivers a synchronous `Indexer.sync` with a progress callback.
