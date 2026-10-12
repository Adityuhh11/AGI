# Status

Updated at the end of every phase. Last updated: 2026-10-12.

| Phase | Scope | State |
|---|---|---|
| 0 Foundations | Package, tooling, settings, docs scaffold | done |
| 1 Embedding | `Embedder`, `FakeEmbedder`, `GemmaEmbedder` | not started |
| 2 Store | `FlatStore`: metadata, FTS, exact search, crash safety | not started |
| 3 Ingestion | Ignore rules, chunkers, incremental sync, jobs | not planned yet |
| 4 Search + eval | Keyword + vector fusion, eval harness, baseline | not planned yet |
| 5 API | Authenticated FastAPI app | not planned yet |
| 6 Quality iteration | tree-sitter chunking, rerankers, "no good answer" | ideas only |

## Next

Execute `superpowers/plans/2026-10-12-memory-engine-phases-0-2.md`.

## Known gaps and deviations from the spec

- Chunk id also hashes `source_id` (spec said uri + locator + content hash).
- SQLite column is `vec_row`; `chunks` has an integer primary key `rid` (VACUUM-safe FTS mapping).
- `sources`/`files` tables arrive with ingestion (phase 3); the `Store` protocol is extracted in phase 4.
