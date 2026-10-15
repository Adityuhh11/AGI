# Search quality (memory engine, phase 6)

Date: 2026-10-15. Status: approved by the owner's direction ("make retrieval pretty good; BM25 and keyword matching
included; build the search feature out completely"). Extends `2026-10-12-memory-engine-design.md` (sections 5, 7, 8)
and supersedes its 256-d vectors and rank-fusion rules. Evidence: `docs/eval/2026-10-13-fusion-experiments.md` and
`docs/eval/2026-10-15-search-quality-experiments.md`.

## Goal

Retrieval good enough that the launcher can trust rank 1: exact names, natural-language questions, text inside images,
and "there is nothing about this" all behave sensibly. Measured on the 77-query golden set (24 images, 17 lexical
queries) plus negative queries; the gate fails on regressions.

## What the experiments decided

Baseline (equal-weight rank fusion, 256-d): top1 0.545, recall@3 0.701, MRR 0.635.

| change | result (top1 / MRR) | decision |
|---|---|---|
| vector score + small keyword boost instead of rank fusion | 0.79 / 0.87 | adopt |
| document embedded as `title: <file name> \| text: <chunk>` (the model's own title slot) | +0.05 to +0.08 top1 | adopt |
| 768-d vectors instead of 256-d | +0.08 top1; 512-d gives half of that | adopt 768 |
| file path as a keyword field, phrase terms for punctuated and short queries | lexical group 0.71 to 0.82 | adopt |
| exact-substring (trigram) match on the file path as a third signal | lexical group 0.82 to 0.88 | adopt |
| trigram over all chunk text instead of the path only | one more query (0.909), 5x text size on disk | not now |
| OCR text for images | the vision encoder already reads on-image text (11 of 12 such queries rank 1) | no |
| other query prompts (`CodeRetrieval`, `QuestionAnswering`) | within noise | no |
| cross-encoder reranker, tree-sitter chunking | recall@3 is already 1.000; remaining misses are near-duplicate images and README-vs-diagram ties | not now, revisit on real data |
| a score threshold for "no good answer" | on-topic top cosine at least 0.674, off-topic at most 0.666 (12 negatives) | adopt as a flag |

Best measured configuration: top1 0.896-0.909, recall@3 1.000, recall@10 1.000, MRR 0.94-0.95.

## Design

### Documents are embedded with their file name

`Embedder.embed_documents(texts, titles=None)`. `GemmaEmbedder` formats `title: <title> | text: <text>` when titles are
given (and the stock `title: none` prompt otherwise). The indexer passes the file's base name (not the directory path:
measured better, the directories add noise) for text and `pdf_text` chunks. The chunk text itself is unchanged and still
starts with its relative path line, as when the experiments were run. Image and `pdf_page` chunks have no text and
are unchanged. Queries keep the `SearchQuery` prompt.

### 768 dimensions

`EMBEDDING_DIM = 768` (the model's native size; no Matryoshka truncation). Storage is 3 KB per chunk, 3x before: 100k
chunks are 307 MB of memory-mapped vectors, 1M chunks 3 GB. Exact CPU search stays about 3x slower per query than at
256-d (ADR 0001: 4.3 ms at 100k, so roughly 13 ms), well inside the budget. An existing index is refused on open
(dimension mismatch, "reindex required"); there is no migration, re-indexing is cheap.

### Keyword side (BM25 done properly)

Schema version 3. `chunks` gains `path_words` (the path's words plus its split identifiers). `chunks_fts` indexes
`text`, `identifiers`, `path_words`, ranked by `bm25(chunks_fts, 1.0, 0.5, 2.0)`. The match expression is the quoted
words OR-ed (as now) plus phrase terms: every punctuated token (`refresh-token`, `react-leaflet`, `a.b.c`) as an
adjacent-words phrase, and the whole query as a phrase when it has 2 to 8 words (so an error message or sentence
copied from a file matches as a unit). A second FTS5 table `chunks_path_tri` (trigram tokenizer, over `uri`) answers
"does the query appear literally in a file's path or name", case-insensitively, for queries of 3 or more characters.

### One blended score instead of rank fusion

```
score(chunk) = cosine(query, chunk)
             + KEYWORD_WEIGHT * bm25(chunk) / bm25(best chunk)          (0 if not matched)
             + PATH_WEIGHT    * pathscore(chunk) / pathscore(best chunk) (0 unless the query is in the path)
```

`KEYWORD_WEIGHT = 0.02`, `PATH_WEIGHT = 0.2`. A chunk found only by keyword or path search takes the lowest cosine
in the vector pool as its cosine, so it can still surface. Cosine of unit vectors is comparable across chunk kinds
(text and images), which rank fusion was not: that is what fixes the image ranking problem. The weights come from the
sweep in the experiments note; results are flat for the path weight between 0.05 and 0.4. Ties break by chunk id.
Pools stay 200 per channel. `reciprocal_rank_fusion` and its tests are deleted.

### Cost guards (found by the 100k latency run)

A path needle found in more than 1,000 chunks is ignored (a fragment like `txt` or `.js` is not a file name; scoring
every match cost 849 ms). A query word in more than 20% of the chunks, and in more than 1,000 of them, is left out of the
bm25 match (near-zero weight, but it costs a scan; one such word cost 349 ms). Counts come from an `fts5vocab` table.
Corpora under 1,000 chunks, including the sample, are never affected.

### "No good answer"

`Searcher.search` returns `SearchResult(hits, confident)`. `confident` is true when the best hit's cosine is at least
`confident_cosine` (default `CONFIDENT_COSINE = 0.65`, a `Searcher` argument so tests with the fake embedder can
lower it) or the best hit matched on the path signal; with no hits it is false. The launcher shows
"nothing relevant found" instead of a ranking of noise. The threshold is calibrated on one corpus and one model; it is
a constant in `search/fusion.py` and is re-checked by the eval on every run (see Evaluation).

### API

`POST /search` response: `{"hits": [...], "confident": true}`. Hit fields are unchanged; `score` is now the blended
score (about the cosine, 0.5 to 1.0 for real matches), no longer a rank-fusion sum, and `vector_score` stays the
cosine. `docs/api.md` documents both.

### Evaluation

- Golden entries may declare `none: true` (no file answers the query) instead of `expect`. 15 such queries are added (the last 3 written after the threshold was chosen, as a holdout).
  Metrics for answerable queries are unchanged; two new ones are reported and gated: `false_alarms` (answerable queries
  flagged not confident) and `missed_nones` (unanswerable queries flagged confident).
- Each task re-records `docs/eval/baseline.json` only when the numbers improve and records the delta in its commit.
- Latency: `tests/test_perf.py` is rerun at 768-d with all three channels; budget p95 under 100 ms at 100k chunks
  (query embedding excluded), measured on this laptop and written to `docs/eval/perf.md`.

## Out of scope (measured or reasoned, recorded for later)

Reranker, tree-sitter chunking, full-text trigram, OCR, approximate indexes, float16 vector storage, per-query adaptive
weights. Each needs evidence from real data (the launcher in use) before it earns a place.

## Risks

- The golden set is small and was written by one author after reading the files, which favours vector search; the
  weights could be tuned to it. Mitigation: the chosen weights sit on flat regions of the sweep, and the first thing to
  do once the launcher runs is to log (locally) queries where the user picks a result below rank 1 and add them.
- 768-d triples memory per chunk. Mitigation: documented; float16 storage is the next lever if an index passes about 1M
  chunks.
- The confidence threshold may not transfer to a corpus with many near-topic documents. It is a flag, not a filter:
  the hits are still returned.
