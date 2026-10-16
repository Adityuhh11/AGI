# 0003: `confident` from a noise ceiling the index measures itself

Date: 2026-10-16. Status: accepted. Replaces the fixed 0.65 of 0002. Numbers: `eval/2026-10-16-confidence-gate-experiments.md`.

## Decision

`Searcher.calibrate()` runs 60 fixed nonsense queries (`search/calibration.py`) against the index and takes the cosine that 9 in
10 of them stayed under. A search is `confident` when its best hit's cosine is at or above that ceiling, or the query is part of
the best hit's path (unchanged). The engine calibrates before it announces itself ready (about 1.5 s) and after every index
job. An index under 100 chunks, or one that cannot answer 10 probes, keeps the old 0.65.

## Why

The best cosine of a query nothing answers grows with the index: 0.671 at 431 chunks, 0.717 at 44,000. The fixed 0.65 passed 71%
of nonsense queries on the real index. Cosine itself separates answerable from unanswerable almost perfectly (AUC 0.993 on the real
index), so the fix is the threshold, and the index is the only place that knows where its own noise ends.

## Alternatives measured

| alternative | outcome |
|---|---|
| keep a fixed cosine, re-tuned | no number works on both corpora (0.67 passes 46% of nonsense on the real index, 0.72 flags 8% of answerable queries on the sample) |
| DigUp's per-query z-score of the best cosine | worse separation (AUC 0.89 to 0.93) and thresholds that do not transfer (3.06 passes all nonsense on the real index, 4.86 flags 53% of answerable queries on the sample) |
| flat-tail signal (best minus 10th or 50th) | AUC 0.64 to 0.80 on the real index |
| probe ceiling, p90 | **0 / 0 on the sample, 0.03 / 0.03 on the real index, with no tuning** |
| probe ceiling at the maximum or mean + 2 sd | too strict on the real index (0.17 and 0.11 of answerable queries flagged) |

## Consequences

- The threshold moves when the index does; the engine logs it (`confidence threshold: 0.717 (measured on this index)`).
- About 1 in 10 nonsense queries still passes, by the choice of percentile.
- The eval measures what production does (`runner.run` calibrates first), so its confidence metrics are production's.
- The probes are English words and random letters; an index in another language would want its own probes.
