# Status

Updated at the end of every phase. Last updated: 2026-10-12.

| Phase | Scope | State |
|---|---|---|
| 0 Foundations | Package, tooling, settings, docs scaffold | done |
| 1 Embedding | `Embedder`, `FakeEmbedder`, `GemmaEmbedder` | done |
| 2 Store | `FlatStore`: metadata, FTS, exact search, crash safety | done |
| 3 Ingestion | Ignore rules, chunkers, incremental sync, jobs | not planned yet |
| 4 Search + eval | Keyword + vector fusion, eval harness, baseline | not planned yet |
| 5 API | Authenticated FastAPI app | not planned yet |
| 6 Quality iteration | tree-sitter chunking, rerankers, "no good answer" | ideas only |

## Next

Plan phases 3-5 (ingestion, search + eval, API) using what phases 0-2 taught us.

## Known gaps and deviations from the spec

- Chunk id also hashes `source_id` (spec said uri + locator + content hash).
- SQLite column is `vec_row`; `chunks` has an integer primary key `rid` (VACUUM-safe FTS mapping).
- GPU (bf16) vs CPU (fp32) embeddings agree to cosine >= 0.99995 on the 4-sentence test set.
- `sources`/`files` tables arrive with ingestion (phase 3); the `Store` protocol is extracted in phase 4.
