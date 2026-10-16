# Search latency: from about 75 ms to about 16 ms

Date: 2026-10-16. Index: the owner's real one, 44,552 chunks. Client: a Python script over loopback HTTP with a kept-alive
connection (the Go launcher's client keeps its connection alive). The model runs on the RTX 4050 in bf16.

**Correction.** `2026-10-15-design-latency.md` and the PR description said warm search was p50 203 ms on this index. That
measurement ran while the live launcher was indexing on the same GPU, and its first requests were cold. Measured again with
nothing else running, the same engine took p50 76 ms, p95 101 ms.

## Where the 76 ms went (in-process, 64 queries)

| phase | p50 | p95 | max |
|---|---|---|---|
| embed the query | 56.9 ms | 69.6 ms | 71.2 ms |
| vector scan (44k x 768 matrix product) | 6.3 | 29.8 | 52.6 |
| keyword search (bm25) | 4.7 | 28.0 | 87.0 |
| path search | 1.0 | 6.1 | 14.5 |
| blend, fetch chunk rows | 1.9 | 6.0 | 14.3 |
| HTTP, JSON (health endpoint, for comparison) | 0.9 | 1.5 | 1.6 |

## Two fixes, each measured on its own

**1. The query model through a CUDA graph.** Embedding a 15-token query took 54 ms; tokenizing is 0.3 ms and sentence-transformers adds
almost nothing, so the model is launch-bound: Python issuing about a thousand tiny GPU kernels one by one. A graph recorded once per
padded length (16, 32, 64 tokens) and replayed takes 6.8 ms (`embedding/query_graph.py`). The vector matches the normal path to a cosine
of at least 0.99994 over 44 queries (the GPU and the CPU agree to 0.99995); a longer query, no GPU or any error falls back to the normal
path. The golden eval is unchanged on every metric (top1 0.896, recall@10 1.000, MRR 0.946) and a real-model test pins the agreement.

**2. A warm first keyword search.** The first run of a keyword query was slow and every repeat fast (`binary search`: 98 ms, then 12 ms).
Not the vocabulary lookup (under 1 ms), not the full-text data: each match is joined to its chunk row, and rows are large (the text), so
roughly one page per match, read cold through one read call each. Variants, first use of 12 queries, total:

| variant | sum of first-use keyword ms | startup cost |
|---|---|---|
| default | 488 | 0 |
| scan the full-text tables into a 256 MB cache | 507 | 0.4 s |
| memory-map the database | 265 | 0 |
| 256 MB cache + scan of the chunk rows | 36 | 1.4 s |
| **memory-map + scan of the chunk rows (adopted)** | **23** | **0.2 s** |

`FlatStore` sets `PRAGMA mmap_size` (1 GiB) and `warm()` reads every chunk row once; the engine calls it while the model loads.

## Result (fresh engine, then steady state, kept-alive connection)

| | first 12 queries after ready | steady p50 / p95 |
|---|---|---|
| before | 55, 73, 115, 205, 149, 38, 85, 31, 38, 80, 25, 59 ms | 76 / 101 ms |
| after | 50, 33, 49, 33, 30, 22, 24, 20, 24, 23, 20, 20 ms | **16 / 21 ms** |

With the launcher's 120 ms debounce, results now land about 140 ms after the last keystroke.

## Not done

The vector scan (6 to 30 ms) and the keyword search (up to about 30 ms on the commonest words) are now the largest parts. A scan
over `float16` vectors or keeping only the bm25 candidates that can change the order would shave them; neither is needed at this speed.
