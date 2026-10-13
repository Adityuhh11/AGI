# Status

Updated at the end of every phase. Last updated: 2026-10-13.

| Phase | Scope | State |
|---|---|---|
| 0 Foundations | Package, tooling, settings, docs scaffold | done |
| 1 Embedding | `Embedder`, `FakeEmbedder`, `GemmaEmbedder` | done |
| 2 Store | `FlatStore`: metadata, FTS, exact search, crash safety | done |
| 3 Ingestion | Deny list, scanner, chunkers, incremental sync (jobs move to phase 5) | done |
| 4 Search + eval | Keyword + vector fusion, eval harness, baseline | done |
| 5 API | Authenticated FastAPI app, background index jobs, `python -m agi_memory` | done |
| 6 Quality iteration | tree-sitter chunking, rerankers, "no good answer" | ideas only |

## Next

Phase 6 (quality iteration, driven by the weak groups in the baseline report) and the Go launcher that consumes `api.md` (process start, Windows Job Object, token generation).

## Known gaps and deviations from the spec

- Chunk id also hashes `source_id` (spec said uri + locator + content hash).
- SQLite column is `vec_row`; `chunks` has an integer primary key `rid` (VACUUM-safe FTS mapping).
- GPU (bf16) vs CPU (fp32) embeddings agree to cosine >= 0.99995 on the 4-sentence test set.
- `sources`/`files` tables arrived in phase 3 (schema version 2); no `Store` protocol yet (nothing but `FlatStore` implements it).
- Image chunks carry their file path as `text` (spec said empty) so file names are keyword-searchable.
- Phase 5 deviations from the spec (full list in `api.md`): `/health` also returns `error`; `DELETE /sources/{id}` returns the removed chunk count and is idempotent; the `Embedder` contract gained `device`; IPv6 hosts (`[::1]`) are rejected; `modified_after` is `modified_after_ns`.

## Retrieval quality (baseline, 2026-10-12)

Real model (GPU bf16), 38 golden queries over `Sample Data/`, report in `eval/2026-10-12-baseline.md`:

| top1 | recall@3 | recall@10 | MRR | forbidden hits |
|---|---|---|---|---|
| 0.632 | 0.842 | 0.921 | 0.730 | 0 |

On the CPU in float32 (measured on the index as it was before the review fix) the same set gave top1 0.632, recall@3 0.842, recall@10 0.921, MRR 0.731: the bf16 GPU path costs no measurable quality. One query is worth 0.026, more than the 0.02 gate tolerance, so any single flip fails the gate; `baseline.json` records the device and `n`, a run on another device is measured but not gated, and a baseline from a different golden set or model must be re-recorded. The weakest group is images: the vector side ranks the right image first or second, and fusion demotes it (see "Reading the baseline" in the report). Search latency at 100k chunks: p95 about 43-54 ms on this laptop depending on load, keyword search dominates (`eval/perf.md`).
