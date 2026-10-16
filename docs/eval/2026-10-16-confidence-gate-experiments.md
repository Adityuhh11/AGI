# Confidence gate: DigUp's z-score, a flat-tail signal, and a self-measured noise ceiling

Date: 2026-10-16. Question: the engine's `confident` flag (best cosine at least 0.65) passes nonsense on a big index.
Does DigUp's "stands out" gate (a per-query z-score of the best cosine) fix it, or something else?

## Data

- **Sample corpus** (431 chunks): the 92 golden queries, 77 answerable and 15 not.
- **Real index** (44,552 chunks, the owner's Downloads, OneDrive and `C:\dev`): 35 answerable queries (about a file or
  folder I knew exists; the right file was in the top 5 for 34 of them) and 35 unanswerable (10 gibberish strings, 25
  off-topic sentences such as recipes and sheet music). The labels are mine, so a single query is worth about 3%. The
  real queries are not committed because they name private files.
- Exact scores over every live chunk, real model on the GPU.

## Signals tried

| signal | what it is |
|---|---|
| `cos` | best cosine (the current rule) |
| `z_all` | (best - mean) / std over every chunk |
| `z_mod` | DigUp's: subtract each modality's mean (text-like and image-like), then z |
| `z_best_modality` | z inside each modality, the larger |
| `gap` | best cosine minus the 10th or 50th best (how flat the tail is) |
| **probe ceiling** | run 60 fixed nonsense queries on the index; threshold = the cosine 9 in 10 of them stayed under |

## Results

Separation (AUC, answerable vs unanswerable; 1.0 is perfect):

| signal | sample | real |
|---|---|---|
| `cos` | 1.000 | **0.993** |
| `z_all` | 0.935 | 0.922 |
| `z_mod` (DigUp) | 0.889 | 0.904 |
| `z_best_modality` | 0.923 | 0.878 |
| `gap50` | 0.984 | 0.798 |
| `gap10` | 0.972 | 0.637 |

Cosine already separates almost perfectly; the z-scores and the tail signals are worse. What fails is the **threshold**: the
noise grows with the index (unrelated chunks reach higher cosines when there are more of them), and no fixed number holds.

| rule | sample: answerable wrongly flagged / nonsense wrongly passed | real: same |
|---|---|---|
| `cos >= 0.65` (current) | 0 / 0 | 0 / **0.71** |
| `cos >= 0.67` (best on sample) | 0 / 0 | 0 / 0.46 |
| `cos >= 0.72` (best on real) | 0.08 / 0 | 0.03 / 0.03 |
| `z_all >= 3.06` (best on sample) | 0.09 / 0.13 | 0 / **1.00** |
| `z_all >= 4.86` (best on real) | **0.53** / 0 | 0.06 / 0.17 |
| **probe ceiling (p90 of the index's own nonsense probes)** | **0 / 0** (ceiling 0.671) | **0.03 / 0.03** (ceiling 0.717) |

Other aggregations of the probes were worse on the real index: mean + 2 sd 0.11 / 0, maximum 0.17 / 0, mean + 3 sd 0.17 / 0.
A margin of 0.02 on top of p90 raised the answerable misses (0.03 to 0.11 on real) for nothing.

## Why the z-score loses here

A z-score divides by the spread of all chunks, but the best of N draws grows with N, so the same z means different things on
431 and on 44,000 chunks (the best z of a nonsense query was 3.87 on the sample and 7.15 on the real index). DigUp's threshold of 2
is tuned to its own corpora. Our model, with file-name titles and a path signal, also already ranks well on cosine alone.

## Decision

`docs/decisions/0003-confidence-from-a-measured-noise-ceiling.md`: the engine measures its noise ceiling with the probes at
startup and after every index job; `confident` means the best cosine is above it (or the query is part of the path).
Below 100 chunks it keeps the old 0.65. Measured end to end: the eval keeps top1 0.896, recall@10 1.000 and MRR 0.946,
false alarms stay 0.000, and missed nones go from 0.067 to 0.000 (`2026-10-16-confidence-calibrated.md`). On the live index
the engine measured 0.717, the number the experiment predicted.

## Limits

70 hand-made queries; one query is 3%. The probes stand for "a query with no answer", so a vague but real query that resembles
nothing strongly is also flagged, which is the intended behaviour. About 1 in 10 nonsense queries can still pass by
construction (the percentile).
