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
| 6 Search quality | Blended score, file-name titles, 768-d, path field + phrases + path trigram, confidence flag, eval with negatives | done |

## Next

The Go launcher that consumes `api.md` (process start, Windows Job Object, token generation, hotkey panel). After that, with the launcher in use: collect real queries where the right result was not first, add them to the golden set, and only then consider a reranker, full-text trigram or tree-sitter chunking (ADR 0002 lists the triggers). Index freshness (nothing watches the disk; refresh is a repeat `POST /sources`) is the other open item.

## Known gaps and deviations from the spec

- Chunk id also hashes `source_id` (spec said uri + locator + content hash).
- SQLite column is `vec_row`; `chunks` has an integer primary key `rid` (VACUUM-safe FTS mapping).
- GPU (bf16) vs CPU (fp32) embeddings agree to cosine >= 0.99995 on the 4-sentence test set.
- `sources`/`files` tables arrived in phase 3 (schema version 2); no `Store` protocol yet (nothing but `FlatStore` implements it).
- Image chunks carry their file path as `text` (spec said empty) so file names are keyword-searchable.
- Phase 5 deviations from the spec (full list in `api.md`): `/health` also returns `error`; `DELETE /sources/{id}` returns the removed chunk count and is idempotent; the `Embedder` contract gained `device`; IPv6 hosts (`[::1]`) are rejected; `modified_after` is `modified_after_ns`.
- Phase 5 review leftovers (accepted): the store opens during app startup, so on a very large index the port opens later than the usual second (not yet measured); stopping the process mid-job is a hard stop (safe, the sync is redone); a `Thread.start()` failure would leave a job `running` until restart.

## Retrieval quality after phase 6 (2026-10-15)

Real model (GPU bf16), 92 golden queries over `Sample Data/` (77 answerable, 15 with no answer), gated by
`baseline.json`; decision record `decisions/0002-search-scoring-and-768-dimensions.md`.

| step | top1 | recall@3 | recall@10 | MRR |
|---|---|---|---|---|
| start of phase 6 (rank fusion, 256-d) | 0.545 | 0.701 | 0.818 | 0.635 |
| blended score | 0.792 | 0.961 | 0.974 | 0.873 |
| + file-name title, 768-d | 0.883 | 0.987 | 1.000 | 0.932 |
| + path field, phrases, path trigram | 0.896 | 1.000 | 1.000 | 0.942 |
| + confidence flag, negatives | 0.896 | 1.000 | 1.000 | 0.942 |

Confidence: false alarms 0.000 (answerable queries flagged not confident), missed nones 0.067 (1 of 15 off-corpus queries
flagged confident: "passport renewal form", cosine 0.666 against the 0.65 threshold). Remaining rank-1 misses (8 of 77):
two near-duplicate photo pairs, two diagram queries where the README that describes the diagram ranks first, and lexical
queries whose answer is rank 2 or 3. Latency at 100k synthetic chunks: p50 about 26 ms, p95 about 30 ms with the machine
idle, about 80 / 105 ms under load (`eval/perf.md`).

Known limits: the golden set is small and was written by one author; the confidence threshold has a thin margin on one
corpus; 768-d needs 3 KB per chunk (float16 or Matryoshka 512 if an index passes about 1M chunks).

## Retrieval quality, widened set before phase 6 (2026-10-13)

The golden set grew to 77 queries (24 more images, 17 lexical, 4 mixed) and `baseline.json` was re-recorded for the
current engine: top1 0.545, recall@3 0.701, recall@10 0.818, MRR 0.635, forbidden 0. Offline fusion experiments
(`eval/2026-10-13-fusion-experiments.md`) show vector search alone reaches top1 0.792 / MRR 0.865, so equal-weight rank
fusion is the main weakness, not only for images.

## Retrieval quality (original 38-query baseline, 2026-10-12)

Real model (GPU bf16), 38 golden queries over `Sample Data/`, report in `eval/2026-10-12-baseline.md`:

| top1 | recall@3 | recall@10 | MRR | forbidden hits |
|---|---|---|---|---|
| 0.632 | 0.842 | 0.921 | 0.730 | 0 |

On the CPU in float32 (measured on the index as it was before the review fix) the same set gave top1 0.632, recall@3 0.842, recall@10 0.921, MRR 0.731: the bf16 GPU path costs no measurable quality. One query is worth 0.026, more than the 0.02 gate tolerance, so any single flip fails the gate; `baseline.json` records the device and `n`, a run on another device is measured but not gated, and a baseline from a different golden set or model must be re-recorded. The weakest group is images: the vector side ranks the right image first or second, and fusion demotes it (see "Reading the baseline" in the report). Search latency at 100k chunks: p95 about 43-54 ms on this laptop depending on load, keyword search dominates (`eval/perf.md`).
