# 0002: One blended score, file-name titles, 768 dimensions, a path signal and a confidence flag

Date: 2026-10-15. Status: accepted. Supersedes the 256-d vectors and rank-fusion parts of 0001 and of the 2026-10-12
memory engine spec. Design: `superpowers/specs/2026-10-15-search-quality-design.md`. Numbers:
`eval/2026-10-13-fusion-experiments.md`, `eval/2026-10-15-search-quality-experiments.md`.

## Decision

- **Score = cosine + 0.02 x relative bm25 + 0.2 if the query is part of the chunk's path.** Cosine of unit vectors is the base because it is
  comparable across text, PDF pages and images; bm25 (text, identifiers and path words, weights 1.0 / 0.5 / 2.0, with
  phrase terms) and a trigram match of the query inside the file path only add to it.
- **Documents are embedded as `title: <file name> | text: <chunk>`** (EmbeddingGemma's own title slot).
- **768 dimensions**, the model's native size, instead of 256.
- **`confident` flag** on every search: the best hit's cosine is at least 0.65, or the query is part of its path.
- Two cost guards: a path needle found in more than 1,000 chunks or 50 files of the searched scope is ignored, and a
  query word in more than 20% of the chunks (and more than 1,000) is left out of the bm25 match. Matching chunks of a
  file all get the same path boost, so the embedding orders them.

## Why

On the 77 answerable golden queries (24 images, 17 lexical): top1 0.545 to 0.896, recall@3 0.701 to 1.000, MRR 0.635 to
0.942 (each step committed with its own eval run). The causes, in order of effect: equal-weight rank fusion let a weak
keyword list outvote a strong vector list and could not compare images with text (+0.25 top1 for the blend); the title
slot (+0.05 to +0.08) and the full 768 dimensions (+0.08) are independent gains that stack; the path field, phrases and
the path trigram fix lexical queries (0.71 to 0.88). On 15 off-corpus queries the flag was right 14 times, with no
false alarm on answerable queries.

## Alternatives measured or considered

| alternative | outcome |
|---|---|
| keep rank fusion, vector weight 3 | top1 0.584: not enough |
| min-max or z-score blends, confidence-gated boosts | within one or two queries of the simple boost |
| 256-d or 512-d vectors | 0.857 / 0.870 top1 against 0.896 at 768 (with titles) |
| full path as the title | worse than the file name alone (0.857 vs 0.896 at 768) |
| trigram over all chunk text | one more query right (0.909), five times the text size on disk |
| OCR for images | not needed: the vision encoder already reads on-image text (11 of 12 quoted-text queries rank 1) |
| other query prompts | no difference |
| cross-encoder reranker, tree-sitter chunking | not tried: recall@3 is 1.000 and the remaining misses are ambiguous near-duplicates |

## Costs

768-d triples vector storage and memory (3 KB per chunk: 307 MB per 100k chunks) and the exact search about 1.5x
(16 ms against 11 ms at 100k on the CPU). Phase 6 adds roughly 10 ms to a search at 100k chunks. An existing index is
refused on open and must be rebuilt (a model, dimension or schema change always did). The confidence threshold is
calibrated on one corpus with a thin margin (on-topic at least 0.674, off-topic at most 0.666); it is a flag, hits are
still returned, and a top hit found only by keyword search is never confident. The path index repeats a file's path
per chunk (a few MB at 100k chunks).

## Revisit when

- Real use shows misses the golden set does not (add them to it first): then try a reranker, full-text trigram or
  tree-sitter chunking, in that order of cost.
- An index passes about 1M chunks (3 GB of vectors): float16 storage or Matryoshka 512.
- The corpus changes character (many near-topic documents): recalibrate the confidence threshold on negatives from it.
