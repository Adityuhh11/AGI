# Search quality experiments (phase 6)

Date: 2026-10-15. Real model (GPU bf16), 77 golden queries over `Sample Data/` (96 files, 431 chunks), same offline
method as `2026-10-13-fusion-experiments.md`: embed every chunk under each document format and dimension once, cache the
keyword rankings, replay fusion rules. Nothing here changed the engine. Cells are top1 / MRR; groups: nl = original
code/doc/PDF queries (32), img = image queries (24), lex = lexical (17), mix = identifier plus description (4).

## Embedding side (vector only, no keyword)

| dimension | document text | all top1 | all MRR | nl | img | lex |
|---|---|---|---|---|---|---|
| 256 | text only (today) | 0.779 | 0.858 | .81/.88 | .79/.90 | .71/.77 |
| 256 | `title: <full path> \| text:` | 0.831 | 0.892 | .88/.92 | .83/.92 | .76/.82 |
| 256 | `title: <file name> \| text:` | 0.857 | 0.906 | .94/.95 | .83/.92 | .76/.83 |
| 512 | file name title | 0.870 | 0.915 | .94/.95 | .83/.92 | .82/.86 |
| 768 | text only | 0.857 | 0.905 | .94/.95 | .83/.92 | .76/.82 |
| 768 | full path title | 0.857 | 0.907 | .97/.97 | .83/.92 | .71/.79 |
| 768 | file name title | 0.896 | 0.929 | .97/.97 | .88/.94 | .82/.86 |

Query prompts `SearchQuery`, `CodeRetrieval`, `QuestionAnswering` (256-d, full path title): top1 0.831 / 0.831 / 0.844,
MRR 0.892 / 0.898 / 0.898: noise, keep `SearchQuery`. Titles and dimension are independent gains that stack.

## Keyword side (keyword only)

| variant | top1 | lex group |
|---|---|---|
| current: text 1.0, identifiers 0.5 | 0.506 | .71/.82 |
| + path field weight 1 to 4 | 0.532 | .76/.85 |
| + path field 2 + phrase terms | 0.545 | .82/.90 |
| + prefix matching | 0.532 | .82/.90 (images worse) |
| trigram phrase over path + text | 0.247 | .88/.91 (natural language 0.12) |

Prefix matching is not adopted. Trigram alone is useless for questions but the best lexical matcher.

## Blended score on 768-d, file name title (top1 / MRR; lex)

| rule | top1 | recall@3 | MRR | lex |
|---|---|---|---|---|
| vector only | 0.896 | 0.961 | 0.929 | .82 |
| + 0.02 x bm25 (path field, phrases) | 0.883 | 1.000 | 0.933 | .82 |
| + 0.05 x bm25 | 0.883 | 1.000 | 0.937 | .88 |
| + 0.02 x bm25 + 0.1..0.4 x trigram on path | 0.896 | 1.000 | 0.942 | .88 |
| + 0.02 x bm25 + 0.1..0.4 x trigram on path + text | 0.909 | 1.000 | 0.950 | .94 |

The trigram weight is flat from 0.05 to 0.4. Trigram index size on the sample: raw text 400 KB, word index 725 KB,
trigram over path and text 2040 KB (5.1x raw text); over the path alone it is negligible. Path-only is adopted; the
full-text version gains one query for five times the text on disk.

The adopted code scores every chunk whose path matches 1.0 (the rows above used the trigram bm25 relative to the best
match); the real eval is identical (top1 0.896, recall@3 1.000, MRR 0.942) and the order among a file's chunks is left
to the embedding.

Remaining rank-1 misses with the adopted rule (7 of 77): two pairs of near-duplicate photos (Raspberry Pi, swapped), two
diagram queries where the README that describes the diagram outranks the diagram, and three lexical queries whose
answer is rank 2 or 3.

## Text inside images

12 queries quoting text that appears only inside screenshots and posters ("Innovator Leaderboard", "Spider-Man: Brand
New Day", a Hindi song title, `check_gradient`, ...): the vector search ranks the right image first for 11 and third
for one (identifiers inside a code screenshot). Keyword and trigram search find none of them (no text is indexed for
images). OCR is not needed for these.

## "No good answer"

Top cosine (768-d, file name title): 77 golden queries min 0.674, 10th percentile 0.724, median 0.774; the 12
out-of-corpus queries (recipe, tax invoice, passport renewal, ...) max 0.666, 90th percentile 0.626, median 0.599. The
text-in-image queries sit at 0.65 to 0.81. A threshold near 0.65 separates the sets on this corpus with a thin margin
(the passport query reaches 0.666).
