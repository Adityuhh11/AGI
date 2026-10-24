# Performance

How fast AGI is and what it costs to run, measured, with and without a graphics card. Numbers are from one machine and
are not promises: they are here so a reader can judge, and so a change that makes them worse is noticed.

**The machine:** Windows 11, Intel Core i5-13420H (8 cores, 12 threads), 15.7 GB RAM, NVIDIA RTX 4050 Laptop (6 GB) plus the
CPU's Intel UHD graphics. File search runs llama.cpp b11719 (Vulkan) with the Q8_0 EmbeddingGemma 2 model. "Sample Data" is the
repository's own test folder (96 files: code, PDFs, screenshots, photos; 405 text chunks, 20 PDF pages, 26 images), git-ignored.
"The real index" is the owner's: 46,000 chunks over Downloads, OneDrive, `C:\dev` and project folders (16,713 PDF pages in 533 PDFs,
997 images, 28,678 text chunks).

## At a glance

| | With the graphics card | Without (processor only) |
|---|---|---|
| Hotkey to a painted, focused input | 31 to 36 ms median | same: the path never touches the model |
| Type letters, see apps | 0.2 ms per query | same |
| File search, one query | 16 ms median, 21 ms p95 (real index) | 40 to 44 ms median, 59 ms max (small index; dominated by embedding the query on the CPU) |
| Index the sample folder, default choices | 16.5 s | **466 s** (7 min 46 s) |
| Index the sample folder, cheapest choices (PDF words only, photos by name) | 8.0 s | **292 s** (4 min 52 s) |
| One picture or PDF page | about 0.16 s | 4.9 s |
| Text | about 53 chunks/s | 1.4 chunks/s |
| Idle | 0 % CPU; about 270 MB private memory in the launcher and its web view | same |
| File search running | plus about 450 MB (Python) and 1.06 GB of video memory | plus about 450 MB and about 1 GB of ordinary memory for the model |

## Using minimal resources

Measured on the running app, idle, with file search on and an index of 46,000 chunks:

| Process | Private memory | Notes |
|---|---|---|
| `agi.exe` | 64 MB | the launcher: hotkey, tray, apps, settings |
| WebView2 (its 5 processes) | about 210 MB | the search bar and Settings pages; Windows trims their working set to about 30 MB when idle |
| `python.exe` (memory engine) | 457 MB | store and search; the vector file is memory-mapped |
| `llama-server.exe` | 1.77 GB committed, about 100 MB resident | the commit is the graphics driver's reservation for video memory, not RAM in use (flags that change it were tried, `eval/2026-10-20-indexing-throughput.md`) |
| Video memory | 1.06 GB of 6 GB | only the model; storage and search never use the GPU |

CPU over a 10-second idle window: 0.000 s for the launcher, 0.03 s for the engine and for the llama server. The watcher costs
nothing when folders are quiet (no syncs in a quiet minute, `eval` 7k). On disk: the launcher is a 13 MB file; file search adds
1.15 GB (engine, Python environment and models, deletable from Settings) and the index of 46,000 chunks is 380 MB.

File search is optional. Without it the launcher is the 270 MB row and nothing else.

## Speed in detail (with the graphics card)

| What | Result | Where measured |
|---|---|---|
| Hotkey to painted input (panelbench, real app) | p50 31.0 to 36.2 ms, p95 42 to 46 ms; limit is 100 ms | `eval/2026-10-18-m2-release.md`, `STATUS.md` 9g |
| With eight pinned rows on screen | p50 33.3 to 34.6 ms | `eval/2026-10-19-pins.md` |
| App list query | 0.23 ms; 176 apps indexed, icons cached 45 s after first start | `eval/2026-10-17-app-index.md` |
| File search, steady state | p50 16 ms, p95 21 ms on 44,552 chunks; first query after start 50 ms, then about 20 ms | `eval/2026-10-16-query-latency.md` |
| File search while indexing at Fast | p50 93 ms; at Balanced 41 ms; at Gentle 36 ms | `eval/2026-10-18-pace.md` |
| Search at 100,000 chunks | p50 about 26 ms with the machine idle | `eval/perf.md` |
| Engine ready after start | 6.3 s (the bar works while it loads) | `eval/2026-10-18-llamacpp-embedder.md` |
| Turn file search on: button to working search | 2 min 2 s to 2 min 6 s, about 1 GB download | `eval/2026-10-18-engine-install.md` |
| A new file appears, then is searchable | about 3.5 s; a deleted file is gone after 4.5 s | `STATUS.md` 7k |
| A file moved or renamed | 0.3 s, no model call (identical content is recognised) | `STATUS.md` 7j |
| The sample folder, from empty | 16.5 s (defaults), 15.9 s (Fast, after the throughput pass), 8.0 s (cheapest) | `eval/2026-10-20-indexing-choices.md` |
| The whole real index from empty | about 80 minutes before the `auto` PDF rule, 21 to 31 minutes less after it | `eval/2026-10-20-indexing-throughput.md` |

Indexing speed is a dial, not a cost you cannot control. Balanced takes 1.9 times as long as Fast and keeps the graphics card
busy 27 % of the time instead of 65 %; Gentle takes 5.9 times as long at 16 % (`eval/2026-10-18-pace.md`).

## A computer without a graphics card

If no discrete graphics card can run the model through Vulkan, the engine runs it on the processor. It still works, produces the same
search results, and starts in 3 s; only indexing is slow, because every picture and page is a vision-model pass.
Settings says so on the File search page when it happens.

The run below is the sample folder indexed twice from empty with the model forced onto the CPU (the same laptop, GPU unused):

| Choices | Time | Chunks | Query p50 (max) |
|---|---|---|---|
| Defaults (`pdf_pages = auto`, `photos = content`, `videos = name`) | 466 s | 422 | 44 ms (57 ms) |
| Cheapest (`text`, `name`, `name`) | 292 s | 411 | 40 ms (59 ms) |
| *Same folder on the graphics card, for scale* | 16.5 s and 8.0 s | | 6 to 14 ms |

Per item: a picture or PDF page takes 4.9 s, text 1.4 chunks/s (about 0.7 s each). An integrated Intel GPU (UHD) measured about the same
for pictures (4.8 s) and twice the text rate (2.6 chunks/s); the engine does not use integrated Intel GPUs and runs on the CPU
(a stronger integrated GPU, Iris Xe or Radeon, is untested).

What that means in practice:

- **Text-only folders are fine.** A project of 5,000 text chunks takes about an hour once on a CPU and under two minutes on a graphics
  card; after that only changes are read.
- **Pictures are what makes the first index slow.** 1,000 photos is about 80 minutes on a CPU. On a computer without a graphics card choose
  *Words only* for PDFs and *Their names* for photos; the page recommends this and applies nothing for you.
- **Searching stays quick** (about 40 ms), and works while the index is still building.
- **Choose Gentle** if the computer is also in use: it leaves the processor free.

## Quality (the reason the speed is worth having)

92 hand-written queries over the sample folder (77 with an answer, 15 with none), on the real model, same results on the GPU and on the CPU:

| top-1 | recall@3 | recall@10 | MRR | false alarms | missed "no answer" |
|---|---|---|---|---|---|
| 0.896 | 1.000 | 1.000 | 0.946 | 0.000 | 0.000 |

The set is small and written by one author; it gates changes (`docs/eval/baseline.json`) rather than proving general accuracy.

## How these were taken, and what they do not say

- Latency numbers use the project's own tools: `cmd/panelbench -real` for the hotkey path, a kept-alive HTTP client for search, and
  `nvidia-smi` sampled every 0.2 s for the GPU. Reports with the method are in `docs/eval/`.
- The launcher's own engine was often running beside a benchmark, so absolute numbers carry its noise; where it mattered it is stated.
- The CPU rows were taken on 2026-10-24 with `sidecar` code at this commit, the model forced onto the CPU, nothing else heavy running. One
  machine, one run each: treat them as the right order of magnitude (minutes, not seconds, and not hours for a small folder), not as a benchmark.
- Not measured: a slower or older CPU, a 4-core laptop, 8 GB of RAM, an AMD or Intel Arc card, a hard disk instead of an SSD.
  Those are tracked in the stability phase of `STATUS.md`.
