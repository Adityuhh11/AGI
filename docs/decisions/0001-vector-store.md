# 0001: Custom exact-search vector engine behind a small interface

Date: 2026-10-12. Status: accepted (revisit triggers below). Supersedes an earlier "Chroma first" draft the same day.

## Decision

The Python sidecar owns the index. All access goes through a small interface (`add`, `delete`, `search`, metadata
filters). The first implementation is our own **exact (flat) search engine**: vectors in one memory-mapped file,
metadata and a SQLite FTS5 keyword index in one SQLite file, exact search on the CPU (decided 2026-10-12 so VRAM stays
free for models; the GPU benchmark below was run but not adopted). Keyword and vector results are merged by rank fusion. **Chroma** (HNSW) stays only in the benchmark harness as a
speed/quality baseline. Reasons: retrieval quality matters more than build time, exact search has no recall loss, and one
store avoids keeping Chroma and SQLite in sync.

## Spike: exact search on the GPU (RTX 4050, 256-dim fp16, 100 queries)

| Vectors | Query | VRAM | With 10% metadata filter |
|---|---|---|---|
| 1M | 3.2 ms | 0.49 GB | 3.1 ms |
| 2M | 6.0 ms | 0.96 GB | 6.2 ms |
| 4M | 11.6 ms | 1.9 GB | 11.7 ms |
| 8M | 23 ms | 3.8 GB | 23 ms |

Loading 1M vectors onto the GPU takes 64 ms. A batch of 64 queries ran at ~0.5 ms each up to 4M, then collapsed at 8M
(score matrix overflowed VRAM). VRAM is shared with the embedding model and an LLM, so realistic ceiling is ~2-4M vectors;
the CPU fallback is 47 ms at 1M. Beyond ~10M an approximate index is needed (use hnswlib, not a port of Chroma's code).

## Evidence

Benchmarks on this machine (RTX 4050 6 GB, 256-dim vectors, 100 queries). Real EmbeddingGemma 2 vectors from `Sample Data/`
(383 items) cloned with noise to reach N, so absolute accuracy figures are rough.

| Store | 100k query | 1M query | 1M accuracy | 1M build | 1M RAM | 1M disk |
|---|---|---|---|---|---|---|
| NumPy exact | 4.3 ms | 47 ms | 100% | none | ~1 GB (the matrix) | none |
| sqlite-vec | 70 ms | 701 ms | 100% | 17 s | ~0 | 1.0 GB |
| LanceDB flat | 68 ms | 1062 ms | 100% | 4 s | 1.8 GB | 1.0 GB |
| LanceDB IVF-PQ (untuned) | n/a | 9 ms | 7.6% | 22 s | 2.1 GB | 1.0 GB |
| Chroma HNSW | 0.7 ms | 1.2 ms | 93.4% | 326 s | 1.6 GB | 1.3 GB |

Retrieval quality on the sample set (383 items; code, PDF pages, images): top-1 on 10/12 known-answer queries, top-3 on 12/12.
Cross-modal (text to image) works. Similarity scores are not calibrated (nonsense query scored 0.655, real hits 0.74-0.85),
so "no result" must use the gap to the next hit, not a fixed cutoff. Exact-name queries win by thin margins, so keyword search is needed.

## Not tested yet

Delete/update behavior, filtered search, startup time at scale, Windows file locking, LanceDB index tuning.

## Why not the alternatives

- **sqlite-vec / LanceDB flat:** 0.7-1 s per query at 1M; fine below ~100k.
- **LanceDB IVF-PQ:** my untuned settings gave poor recall; not a verdict on LanceDB.
- **Qdrant:** separate server process; overkill for one machine.

## Revisit when

Item count passes ~1M (screen-memory phase), a store fails the oracle recall check, or custom GPU search (see spike) beats Chroma.
