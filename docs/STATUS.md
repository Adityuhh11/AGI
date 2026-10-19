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
| 7l Metal and light | The ring's light is our own shader that circulates clockwise round the outline (Paper's stripes seemed to come from different sides on a thin rim); the plate is flat opaque `#000` with a lit edge (grain, band, gradient and glass were tried and dropped); the search glyph is a hairline lens whose glint circles while a search runs. A frosted-glass plate (screen snapshot blurred under the tint, 9 to 17 ms per opening) was built and dropped at the owner's call | done |
| 7m Indexing robustness | `exclude = [...]` in `config.toml` (absolute folders inside a root) leaves folders out of the index and drops what they held at the next sync (`POST /sources` takes `exclude`); one file that fails for any reason is recorded and skipped, and five unexpected failures in a row abort the sync instead of hiding a failing GPU (corrupt files never count); a failed sync is retried after 1, 2, 4, 8, then every 15 minutes instead of every minute | done |
| 7n Apps in the bar | `internal/rank` (fuzzy matching on word starts, humps and initials, plus frecency), `internal/apps` (the Start Menu list through `Get-StartApps` and a keep-list, cached, refreshed in the background and on Start Menu changes, launched through the shell, icons made by the shell and cached), `internal/state` (what was launched and after typing what), `internal/palette` (which app rows show and what Enter does); the page gains app rows, the Tab modes (All, Apps, Files, or `app:`) and the Ctrl+K action panel. Measured: 176 of 203 Start entries kept, 0.23 ms a query, 174 icons cached 45 s after start, hotkey path unchanged (`eval/2026-10-17-app-index.md`, `eval/2026-10-17-palette-hotkey-path.md`). Launching an app that is already open focuses its window instead of starting a second copy (`focus_windows.go`: matched by exe path, else by program name or the exe's file description, because Zed and Zen are only an id; 7 ms warm, 250 ms on a cold first call; Store apps run inside ApplicationFrameHost and still start again) | done, owner to verify |
| 7o Web in the bar | `internal/web` opens searches and links in the browser set in `config.toml` (`browser = "zen"`; Firefox-based browsers search with the engine chosen inside them through `-search`, others and an explicit `search_url` use an address template; a missing browser is an error, never a fallback to another one). `[[quicklink]]` entries (`trigger`, `title`, `url` with `{q}`) turn `gh rust` into a search on GitHub. In the bar: a "Search the web for ..." row last, the Web mode (Tab, `?` or `web:`), quicklink rows first, Alt+Enter from anywhere. Spikes in `eval/2026-10-17-zen-command-line.md` | done, owner to verify |
| 7p Tags | `internal/state` keeps `tag -> things` beside the launch history (`state.json`); Ctrl+T on a file or an app toggles a tag from a picker (new names are created by typing), rows show their tags as chips, `#work` / the Tags chip lists everything tagged (apps and files together, deleted files dropped, `#work budget` narrows by name). Folders and links cannot be tagged yet because nothing in the bar shows them as rows | done, owner to verify |
| 8a Embedding runtime spike | llama.cpp (`llama-server`, Q8_0 GGUF, Vulkan or CUDA) against torch on the real model and data: golden metrics identical, vectors agree to cosine 0.994 or better on 760 real chunks, pages and images, throughput 64 to 116% of torch, file search needs about 1.1 GB instead of about 6 GB, ready in 3 s instead of 10 to 35 s. Go; the swap is part of the packaging phase (`eval/2026-10-18-llamacpp-embedder.md`, ADR 0004). Spike scripts are in `sidecar/spikes/` | spike done |
| 8b Packaging, M1 | The launcher as a zip for someone else: `[search]` settings and no repository discovery in `config.toml` (the old `[sidecar]` table still loads); `internal/settings` (validated, atomic, live: hotkey rebinding, folders, browser and quicklinks apply without a restart); the engine may be off (`StateOff`, `sidecar.Switch`) and the bar says so; a settings window (General, Quicklinks, File search folders, About; `Ctrl+,`, tray) on `#settings`; `scripts/release.ps1` and a README; first start opens Settings. Hotkey to painted p50 33.8 ms. Spec and plan: `superpowers/specs/2026-10-18-packaging-design.md`, `plans/2026-10-18-packaging.md` (`eval/2026-10-18-m1-release.md`) | built; M1 zip ready for the friend |
| 8c Engine on llama.cpp | `LlamaEmbedder` and `LlamaServer` replace the torch embedder: golden eval identical on the GPU and on the CPU, an index built with torch answers the same, engine ready in 6.3 s, torch and its libraries removed from the project (`sidecar` environment 157 MB). The server dies with the engine (job object) and the supervisor kills the whole process tree. `model_id` unchanged, so no index is rebuilt. `baseline.json` now records `device: vulkan` (`eval/2026-10-18-llamacpp-embedder.md`, ADR 0004) | done |
| 8d Turn on file search from the app | `internal/enginesetup` (pinned uv, llama.cpp b11719 Vulkan and the two GGUF files by SHA-256, resumable downloads, Python 3.12 through uv, a check, a manifest written last, update and remove, a background manager) and the File search page (what will be downloaded and from where, progress, cancel, turn off, update, delete files). The engine's source is embedded in `agi.exe`. Driven for real in a clean profile: 2 min 6 s from the button to a working search (`eval/2026-10-18-engine-install.md`) | done |
| 8e Indexing control | `pace.py` (a `Pacer` and a `PacedEmbedder`: after each bulk embedding call the indexer waits `busy * (1/duty - 1)`, pause and resume, searches never paced), `/pace` `/pause` `/resume`; `indexing.Manager` keeps the speed and the pause across engine restarts and reports each folder's state; the File search page has the three speeds (measured: 1.9 and 5.9 times as long as Fast, GPU 65%, 27%, 16% busy, search p50 93, 41, 36 ms while indexing), pause and resume and a line per folder; a tray item pauses too (`eval/2026-10-18-pace.md`) | done |
| 8f Packaging, M2 | `scripts/release.ps1` makes `agi-<version>-windows-x64.zip` (5.3 MB: `agi.exe` with the page and the engine source inside, and a README that explains file search, its 0.9 GB download and the speeds). Run from the zip in a clean profile: install in 2 min 2 s, index, search, no errors in the log, hotkey to painted p50 36.2 ms. Spec and plan: `superpowers/specs/2026-10-18-packaging-design.md`, `plans/2026-10-18-packaging.md` (`eval/2026-10-18-m2-release.md`) | built; waiting for the friend's session |
| 9a Hotkey, start at login, update check | Alt+Space in the hotkey field opened the window's system menu and never reached the page as a key: `settingsui.KeyMenuFilter` drops the system-menu message and reports Alt+Space to the field as an event, so it can be recorded (driven for real in a clean profile: capture, saved, and the hotkey then opened the bar); a Reset button puts the default back. `internal/autostart` starts the launcher at sign-in through the per-user Run key (Task Manager's off switch counts; a moved exe counts as off). `internal/update` asks GitHub for the latest release: a button in About, an opt-in daily check (`check_updates`, off by default), a tray item, never an install (`decisions/0005-update-check.md`); `release.ps1 -Version 0.2.0` stamps the release number | built on `feat/hotkey-and-startup`; owner to verify the hotkey and a sign-in start by hand. Not done: engine restart after sleep (needs a real sleep test first) |
| 7h Search latency | Query embedding through CUDA graphs (54 ms to 7 ms, same vectors), memory-mapped database and a warm-up scan of the chunk rows; steady search p50 76 ms to 16 ms, first queries after start 20 to 50 ms (`eval/2026-10-16-query-latency.md`) | done |

## Next

Packaging is in `dev` (PR #5; the friend ran the M2 zip, `v0.1.0-friend1`, without trouble). `feat/hotkey-and-startup` is the work toward `v0.2.0` (hotkey, start at login, update check, a tidier settings page), then pins, clipboard and snippets. The palette (apps, commands, browsing in Zen, tags, a decision model) is specified in `superpowers/specs/2026-10-17-palette-design.md` and planned, for phases P0 to P2, in `superpowers/plans/2026-10-17-palette-p0-p2.md`, on branch `feat/palette`. Launcher block H still has the acceptance list by hand. Then, in this order of value: the search-quality items found on the real index (search is now 16 ms p50; the vector scan and common-word keyword search are the largest parts), file-name search for types the engine does not read (Office files, videos, archives) and a power and idle policy for big syncs. With the launcher in use: collect real queries where the right result was not first, add them to the golden set, and only then consider a reranker, full-text trigram or tree-sitter chunking (ADR 0002 lists the triggers). 


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
