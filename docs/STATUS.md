# Status

Updated at the end of every phase. Last updated: 2026-10-15.

| Phase | Scope | State |
|---|---|---|
| 0 Foundations | Package, tooling, settings, docs scaffold | done |
| 1 Embedding | `Embedder`, `FakeEmbedder`, `GemmaEmbedder` | done |
| 2 Store | `FlatStore`: metadata, FTS, exact search, crash safety | done |
| 3 Ingestion | Deny list, scanner, chunkers, incremental sync (jobs move to phase 5) | done |
| 4 Search + eval | Keyword + vector fusion, eval harness, baseline | done |
| 5 API | Authenticated FastAPI app, background index jobs, `python -m agi_memory` | done |
| 6 Search quality | Blended score, file-name titles, 768-d, path field + phrases + path trigram, confidence flag, eval with negatives | done |
| 7a Launcher engine support | `POST /search/files` (files with their best chunks), `GET /sources`, `source_id` on hits, queries served ahead of indexing | done |
| 7b Launcher blocks A-E | Sidecar client and supervisor, hotkey panel (cloaked, not hidden), search results UI, open and reveal | done |
| 7c Launcher block F | Tray icon (status line, open, rescan, quit); `internal/indexing` syncs every configured root when the engine is ready and on "Rescan now" | done |
| 7d Launcher block G | Image thumbnails: `/thumb` on the asset server (inside the indexed folders only, 160 px JPEG, cached under `%LOCALAPPDATA%\AGI	humbs`), image rows show them | done |
| 7e Launcher visual design | Transparent window, capsule that grows into a card, WebGL liquid-metal ring, orb loader, Geist type, matched-word emphasis, key hints; hotkey to painted input p50 39 ms / p95 46 ms (`docs/eval/2026-10-15-design-latency.md`) | built, awaiting the owner's review |
| 7f Big index | The owner's real folders (Downloads, OneDrive, `C:\dev`, two projects, Sample Data): 56,252 chunks, 360 MB, about 1.5 h on the GPU; one chunker bug found and fixed on the way | done |
| 7g Confidence calibration | `confident` now means above a noise ceiling the index measures with 60 nonsense probes (`search/calibration.py`), at startup and after each job. DigUp's z-score was tried and lost (`eval/2026-10-16-confidence-gate-experiments.md`); the golden eval's missed nones went 0.067 to 0.000, ranking unchanged | done |
| 7i Scopes and deleted files | `code:` `img:` `pdf:` `doc:` before a query search one kind of file (Go parses it, the engine filters); results whose file was deleted since the last sync are hidden at search time (the engine is asked for 5 spare files); a deleted root is forgotten; the ring changes metal on every opening | done |
| 7j Moves without re-embedding | A new file identical (same SHA-256) to an indexed one that vanished or is unchanged gets that file's image and PDF-page vectors instead of new ones (`reused` in the job report); text is embedded again because its chunks carry the path. Real model, 9-page PDF plus a picture: 2.2 s to index, 0.3 s to move and rename, vectors identical | done |
| 7k File watcher | `internal/watcher` watches every root with `ReadDirectoryChangesW` (one handle per tree), drops events in folders the engine never indexes (dot folders, `node_modules`, `venv`, `__pycache__`) and partial files (`.crdownload`, `.tmp`, `~$`), and after 3 s of quiet (30 s at most) the manager syncs only the changed root. Measured on the real app: a new file is in the index 3.5 s after it appears, a deleted one is gone after 4.5 s, no syncs in a quiet minute | done |
| 7l Glass and light | The ring's light is our own shader that circulates clockwise round the outline (Paper's stripes seemed to come from different sides on a thin rim); the plate is platinum-black brushed graphite over frosted glass: the Go side reads the screen behind the bar as the panel opens (9 to 17 ms, memory only, dropped on close) and the page blurs it under the tint; the search glyph is a hairline lens whose glint circles while a search runs. Seen only by the owner so far | done, owner to verify |
| 7h Search latency | Query embedding through CUDA graphs (54 ms to 7 ms, same vectors), memory-mapped database and a warm-up scan of the chunk rows; steady search p50 76 ms to 16 ms, first queries after start 20 to 50 ms (`eval/2026-10-16-query-latency.md`) | done |

## Next

Launcher block H: the acceptance list by hand, the merge on the owner's word; see `superpowers/plans/2026-10-15-launcher-v1.md`. Then, in this order of value: the search-quality items found on the real index (search is now 16 ms p50; the vector scan and common-word keyword search are the largest parts), engine robustness (one unexpected error fails a whole folder's sync, and failed syncs retry every minute forever), a per-root exclude list (one vendored skills folder is 19% of the index), file-name search for types the engine does not read (Office files, videos, archives) and a power and idle policy for big syncs. With the launcher in use: collect real queries where the right result was not first, add them to the golden set, and only then consider a reranker, full-text trigram or tree-sitter chunking (ADR 0002 lists the triggers). Index freshness (nothing watches the disk; refresh is "Rescan now" or a repeat `POST /sources`) is the other open item.

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
