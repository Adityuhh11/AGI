# Memory Engine, Phase 6 (search quality) Implementation Plan

**Goal:** Implement `docs/superpowers/specs/2026-10-15-search-quality-design.md`: blended scoring, file-name titles,
768-d vectors, a stronger keyword side with a path-trigram signal, and a "no good answer" flag, each step gated by the
77-query eval.

**Architecture:** `search/fusion.py` becomes the blend; `Searcher` combines three channels (vector, bm25, path
trigram) and returns `SearchResult(hits, confident)`; `store/` gains schema v3 (`path_words`, `chunks_path_tri`);
`GemmaEmbedder` formats titles; `eval/` learns negative queries.

**Spec / evidence:** the spec above and `docs/eval/2026-10-15-search-quality-experiments.md`.

## Global constraints

- Run commands from `C:\dev\VS_CODE\Omni\sidecar`. Quality gate before every commit: `uv run ruff check .`,
  `uv run ruff format --check .`, `uv run pyright`, `uv run pytest` clean (check the real exit status).
- Real-model eval after every task that changes ranking: `uv run python -m omni_memory.eval` (about a minute; uses
  `Sample Data/`, git-ignored). A task that improves the numbers re-records the baseline with `--write-baseline` and
  states old and new numbers in the commit body. A task must never lower a metric by more than the gate tolerance.
- Tests use `FakeEmbedder`; real-model tests carry `@pytest.mark.model`. Determinism: ties break by chunk id.
- Never read, index, log or commit `.env*` or `Sample Data/`. No AI attribution trailers in commits.
- Branch `feat/phase-6-fusion` (already holds the golden set and baseline commits). Conventional commits, small.

## Task 1: blended score replaces rank fusion

**Files:** modify `search/fusion.py`, `search/searcher.py`, `tests/test_fusion.py`, `tests/test_searcher.py`; update
`docs/eval/baseline.json`.

- `fusion.py` (rewrite): `KEYWORD_WEIGHT = 0.02`; `blend(vector, keyword, keyword_weight=KEYWORD_WEIGHT) ->
  list[tuple[str, float]]` where `vector` is `list[VectorHit]`, `keyword` is `list[KeywordHit]`. Score = cosine + weight
  x (keyword score / best keyword score); a chunk missing from the vector pool takes the pool's lowest cosine (0.0 for
  an empty pool); output sorted by `(-score, id)`; duplicate ids in one list count once (first wins). Delete
  `reciprocal_rank_fusion`.
- `Searcher`: drop `rrf_k`; call `blend`; `SearchHit.score` is the blended score.
- Tests (table style): a vector-only hit keeps its cosine order; a strong keyword hit lifts a slightly lower cosine
  above a higher one but not above a clearly higher one; keyword-only chunks surface at the floor cosine; empty inputs;
  ties by id; duplicates; the `Searcher` still returns filters, blank-query `[]`, deterministic output.
- Eval expectation: top1 about 0.79, MRR about 0.87 on the 77 queries.
- Commit: `feat(search): blend cosine with a small bm25 boost instead of rank fusion`.

## Task 2: titles and 768 dimensions

**Files:** modify `embedding/base.py`, `embedding/gemma.py`, `embedding/fake.py`, `embedding/serialized.py`,
`indexer.py`, `config.py`, every test that hard-codes 256 (grep `256` and `EMBEDDING_DIM` in `tests/` and `src/`),
`tests/test_gemma_embedder.py`, `tests/test_fake_embedder.py`, `tests/test_indexer.py`.

- `Embedder.embed_documents(texts, titles=None)`: `titles`, if given, has one entry per text. `GemmaEmbedder`: with
  titles, encodes `f"title: {title} | text: {text}"` with `prompt_name=None`; without, the existing `Document` prompt.
  `FakeEmbedder`: tokens of the title join the bag of words. `SerializedEmbedder` passes `titles` through.
- `Indexer`: text chunks (`_flush`) and `pdf_text` chunks pass `titles=[base name of chunk.uri ...]`.
- `config.EMBEDDING_DIM = 768`. Update the spec-independent docs that say 256 in the same commit.
- Tests: the fake embedder's vector changes when the title changes and equals the old vector when `titles=None`; the
  indexer passes the file's base name (spy embedder); an index built at 256-d is refused (existing mismatch test with
  the new default); a `@pytest.mark.model` test that the real embedder's title prompt is applied (a query naming the
  title ranks its chunk above an identical-text chunk with another title).
- Eval expectation: top1 about 0.88, recall@3 about 0.96. Re-record the baseline. Commit:
  `feat(embedding): embed documents with their file name as title, 768 dimensions`.

## Task 3: keyword side v3 and the path signal

**Files:** modify `store/schema.py` (version 3), `store/identifiers.py` (path words helper), `store/keyword.py`,
`store/flat.py`, `store/catalog.py` (insert `path_words`), `search/fusion.py`, `search/searcher.py`, tests
`test_keyword.py`, `test_flat_store.py`, `test_flat_search.py`, `test_fusion.py`, `test_searcher.py`, `test_catalog.py`.

- Schema v3: `chunks.path_words TEXT NOT NULL`; `chunks_fts(text, identifiers, path_words)` with triggers updated;
  `chunks_path_tri` = `fts5(uri, content='chunks', content_rowid='rid', tokenize='trigram')` with its three triggers.
  A v2 index is refused as before ("reindex required").
- `path_words(uri)`: the uri with `/`, `.`, `-`, `_` as separators, lowercased words, plus the split identifiers of each
  component (so `locationService.js` gives `locationservice location service js`).
- `build_match(query)`: quoted OR-ed words as now, plus a quoted phrase for each whitespace token that contains
  punctuation and yields two or more words, plus the whole query as one phrase when it has 2 to 8 words; deduplicated;
  still never exposes FTS5 syntax. `keyword_search` ranks by `bm25(chunks_fts, 1.0, 0.5, 2.0)`.
- `FlatStore.path_search(query, n, filters)`: trigram `MATCH` on the quoted whole query (strip, double any `"`),
  empty for fewer than 3 characters or on an FTS error, grouped so each chunk id appears once, filters applied like
  `keyword_search`, returns `list[KeywordHit]` (score 1.0 each, ordered by id). Chunks of the matched file all match.
- `fusion.blend(vector, keyword, path, ...)`: adds `PATH_WEIGHT = 0.2` for ids in `path`.
- Tests: `path_words` table test; punctuated-token and whole-query phrases; the user text `NEAR(`, `*`, `"` never raises;
  a query equal to a file name surfaces that file's chunks even when no vector hit matches; case-insensitive path match;
  query under 3 characters gives no path hits; filters apply; deleting/replacing a file keeps both FTS tables in sync;
  schema v2 refusal.
- Eval expectation: top1 about 0.90, lexical group about 0.88. Re-record. Commit:
  `feat(search): path field and phrase terms for bm25, a path trigram signal in the blend`.

## Task 4: "no good answer"

**Files:** modify `search/searcher.py`, `search/fusion.py` (`CONFIDENT_COSINE = 0.65`), `eval/golden.py`,
`eval/metrics.py`, `eval/runner.py`, `eval/__main__.py` if needed, `eval/golden.yaml`, `api/schemas.py`, `api/routes.py`,
tests `test_searcher.py`, `test_eval.py`, `test_api_search.py`, `docs/api.md`.

- `SearchResult(hits: list[SearchHit], confident: bool)` frozen dataclass in `types.py`. `Searcher.search` returns it;
  `confident` is true when the top blended hit has `vector_score >= CONFIDENT_COSINE` or was in the path channel; false
  when there are no hits.
- Golden: `none: true` instead of `expect`; exactly one of the two is required; `GoldenQuery.expect` is `()` for those.
  Add 14 off-corpus negative queries (recipe, tax invoice, passport renewal, flight booking, bank statement, symptoms of
  the flu, lease agreement, guitar chords, yoga routine, puppy training, wedding seating plan, plus three more written
  without looking at the index). `QueryResult` gains `confident: bool`. `Metrics` gains `false_alarms` (answerable
  queries flagged not confident, as a fraction) and `missed_nones` (negatives flagged confident, as a fraction); the
  existing ranking metrics ignore negatives. Baseline gate: neither may rise by more than 0.02 (the baseline stores
  both).
- API: `SearchResponse.confident: bool`. Update `docs/api.md` (response, `score` semantics, `confident`).
- Tests: confidence true/false cases with `FakeEmbedder` and a hand-built store; golden parse errors (both keys, neither
  key); metrics arithmetic; baseline compare with the two new keys; API returns `confident` and `[]` plus false for an
  empty index.
- Eval expectation: `false_alarms` 0.0 to 0.05, `missed_nones` at most 0.15. If worse, adjust `CONFIDENT_COSINE`
  against the measured distributions and say so in the commit. Re-record. Commit:
  `feat(search): confident flag for queries with no good answer, negative queries in the eval`.

## Task 5: latency, docs, final gate

**Files:** `tests/test_perf.py`, `docs/eval/perf.md`, `docs/ARCHITECTURE.md`, `docs/STATUS.md`, `docs/api.md`,
`docs/decisions/0002-search-scoring-and-768-dimensions.md`, the 2026-10-12 spec (notes pointing at the new spec).

- Perf: rerun at 768-d with three channels; budget p95 under 100 ms at 100k chunks (best of three rounds, as now);
  record the table in `perf.md`. If it misses, profile (keyword pool and word cap are the levers named in `perf.md`)
  before changing any ranking constant.
- ADR 0002: decision (blend, titles, 768, path signal, confident flag), the numbers, alternatives not taken (rank
  fusion, 256/512-d, full-text trigram, OCR, reranker, tree-sitter) and revisit triggers (real-data misses; index above
  1M chunks: float16 or Matryoshka 512).
- ARCHITECTURE: invariants 10 (one blended score, comparable across kinds), 11 (indexes carry path and title; a model,
  dimension or schema change means reindex); STATUS: phase 6 done with the final table and known gaps; ADR 0001 gets a
  one-line "256-d superseded by 0002".
- Final gate: all four checks, `uv run pytest -m model -q`, `uv run python -m omni_memory.eval` (exit 0), the real-process
  test, `git status` clean of `Sample Data/`. Merge `feat/phase-6-fusion` to `main` fast-forward and push.
- Commit: `docs: search quality decision record, perf and status for phase 6`.
