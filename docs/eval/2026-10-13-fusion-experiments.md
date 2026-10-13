# Fusion experiments (phase 6 groundwork)

Date: 2026-10-13. Real model (GPU bf16), 77 golden queries over `Sample Data/` (96 files, 431 chunks).

## Method

The index is built once. For every query the two rankings (top 200 vector hits with cosine, top 200 keyword hits with
bm25) are cached, then each fusion rule is replayed offline on the cache. Replaying the current rule reproduces the real
eval run exactly (top1 0.545, recall@3 0.701, MRR 0.635), so the harness is trustworthy. Nothing here changed the engine.

Golden set: the 38 original queries, plus 18 queries describing the 24 images added on 2026-10-13, 17 lexical queries
(identifiers, file names, error strings, path fragments) and 4 mixed ones. The old image query 37 expected any
`WhatsApp Image`; with five such files it was tightened to the one file it describes. Groups below: **nl** = the
original code/doc/PDF queries (32), **img** = image queries (24), **lex** = lexical (17), **mix** = identifier plus
description (4).

## Results (top1 / MRR per group)

| rule | all top1 | all r@3 | all MRR | nl | img | lex | mix |
|---|---|---|---|---|---|---|---|
| RRF k=60, equal weights (current) | 0.545 | 0.701 | 0.635 | .72/.82 | .17/.23 | .65/.76 | 1.0/1.0 |
| RRF, vector weight 3 | 0.584 | 0.792 | 0.699 | .72/.83 | .29/.43 | .71/.78 | .75/.88 |
| keyword only | 0.506 | 0.610 | 0.565 | .66/.74 | .08/.09 | .71/.81 | 1.0/1.0 |
| vector only | 0.792 | 0.922 | 0.865 | .84/.90 | .79/.90 | .71/.77 | .75/.81 |
| cosine + 0.02 x relative bm25 | 0.792 | 0.961 | 0.873 | .84/.91 | .79/.90 | .71/.79 | .75/.83 |
| cosine + 0.1 x relative bm25 | 0.766 | 0.961 | 0.863 | .78/.87 | .71/.85 | .76/.84 | 1.0/1.0 |
| min-max score blend, alpha 0.9 | 0.779 | 0.948 | 0.862 | .81/.88 | .79/.90 | .71/.79 | .75/.83 |
| cosine + 0.02 boost, 0.1-0.3 boost when the query looks lexical | 0.805 | 0.961 | 0.885 | .84/.91 | .79/.90 | .76/.84 | .75/.83 |

## Reading it

- **Equal-weight RRF is the wrong rule for this model**, not just for images. Vector search alone beats it by 0.25
  top1 and 0.23 MRR. Rank fusion lets a weak keyword list outvote a strong vector list: bm25 on a natural-language
  question mostly matches common words.
- **Keyword search still earns its place on exact names** (`addcountries`: vector rank 5, RRF rank 1), but its
  value is a small tie-break, not a co-equal ranking. A tiny additive boost keeps those wins and loses nothing.
- **Differences below about 0.03 are one or two queries** and are not claimed as wins: the lexical-query router
  (0.805 vs 0.792) and every score-blend variant. The robust result is the move from rank fusion to a vector-led score.
- **Keyword failures worth fixing separately** (lexical group, keyword-only run): hyphenated and path-like queries are
  tokenised into loose words (`/refresh-token` ranks `jwt.js` first because it contains "refresh" and "token";
  `react-leaflet` ranks `main.jsx` first), and a file-name query such as `Screenshot 2026-07-29 233342` is found only by
  keyword search (rank 1) and then lost by any rule that leans on the vector side (rank 5 under RRF, miss under a boost).
  Both point at the keyword side: phrase handling for punctuated tokens and a file-name match signal.
- **Caveats:** the image and lexical queries were written after looking at the files, which favours the vector side;
  the sample has 24 images, so a group result is 24 queries at best; all numbers are one corpus. The adopted rule must
  be re-checked on the owner's real data before it is trusted for ranking quality.
