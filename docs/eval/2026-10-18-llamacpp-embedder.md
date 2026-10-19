# llama.cpp as the embedding runtime (spike, 2026-10-18)

Plan: `superpowers/plans/2026-10-18-llamacpp-embedder-spike.md`. Decision: `decisions/0004-embedding-runtime.md`.
Machine: RTX 4050 Laptop (6 GB), Intel UHD as second Vulkan device, launcher stopped, nothing else on the GPU.
Spike code: `sidecar/spikes/` (an `Embedder` over `llama-server`, and the scripts named below).

## What was run

| Piece | Version / source |
|---|---|
| llama.cpp | release b11719; `llama-server` from the Windows Vulkan zip (32 MB) and the CUDA 13.4 zip (155 MB) plus its `cudart` zip (423 MB) |
| model | `ggml-org/embeddinggemma-2-GGUF` (Apache-2.0, not gated): `embeddinggemma-2-Q8_0.gguf` 310 MB and `mmproj-embeddinggemma-2-Q8_0.gguf` 555 MB; BF16 files (558 MB, 982 MB) as control |
| reference | the current path: `GemmaEmbedder`, torch bf16 on the GPU |

`llama-server --embedding -ngl 99 -c 8192 -ub 1024 -b 1024 --image-max-tokens 280` (plus `--device Vulkan1` for the
discrete card on the Vulkan build). Texts are sent as `/v1/embeddings` inputs; images as `image_url` data URIs.

## S0: is it possible

Yes. `llama-server` returns 768-d unit vectors for text and for images from the same model, and its tokenizer produces
the same ids as the Hugging Face one (checked on a mixed ASCII/accented string). Two things the client must do:

- **Truncate to 512 tokens itself.** A longer input is a 500 ("input is too large ... increase the physical batch size").
  The spike tokenizes with `/tokenize` only for texts over 400 characters, cuts to 511 tokens and keeps the closing token,
  which is what the HF tokenizer's truncation yields; token lists are accepted as `input`.
- **Give images the size the model's own processor gives them.** The HF processor scales an image so it has about 2520
  patches of 16 px (280 soft tokens), keeps the aspect ratio and floors each side to a multiple of 48 (a 285x438 image
  becomes 624x960). Before that fix, images differed from torch (below). llama.cpp also caps the image token budget by the
  physical batch: at the default `-ub 512` it caps at 256 tokens, so `-ub 1024 --image-max-tokens 280` is needed.

## S1/S2: fidelity against torch bf16 (cosine of the same input)

On the 92-query golden set's corpus (`Sample Data/`: 385 text chunks, 46 images and PDF pages, 92 queries):

| input | Q8_0, as sent by default | Q8_0, images fitted, `-ub 1024`, 280 tokens | BF16 weights, same |
|---|---|---|---|
| text chunks (mean / min) | 0.99988 / 0.99968 | same | 0.99996 / 0.99988 |
| queries (mean / min) | 0.99992 / 0.99971 | same | 0.99997 / 0.99994 |
| images and PDF pages (mean / min) | 0.974 / 0.933 | **0.9991 / 0.9962** | 0.967 / 0.925 (before the fix; not run after it) |
| top-10 overlap of cosine rankings over the whole corpus | 0.938 | **0.973** | |
| same top-1 | 0.902 | **0.989** | |

Quantization is not what separated images from torch: BF16 weights gave the same image cosines (0.9675) as Q8_0 before
the fix. Preprocessing was the cause.

On the owner's real index (a copy; 760 random stored chunks re-embedded and compared with the vectors torch stored):

| kind | n | mean | min |
|---|---|---|---|
| code | 150 | 0.99988 | 0.99976 |
| markdown | 150 | 0.99986 | 0.99967 |
| text | 100 | 0.99986 | 0.99970 |
| pdf_text | 150 | 0.99987 | 0.99965 |
| image | 160 | 0.99957 | 0.99799 |
| pdf_page | 200 | 0.99892 | 0.99414 |

(`s5_real.py`. Before the image fix the same sample gave images 0.981 mean / 0.933 min and PDF pages 0.964 / 0.890.)

## S3: golden eval (`python -m agi_memory.eval` flow, 92 queries, with confidence calibration)

| embedder | top1 | recall@3 | recall@10 | MRR | forbidden | false alarms | missed nones |
|---|---|---|---|---|---|---|---|
| baseline (torch bf16, CUDA) | 0.896 | 1.000 | 1.000 | 0.946 | 0 | 0.000 | 0.000 |
| llama.cpp Q8_0, Vulkan, images fitted | 0.896 | 1.000 | 1.000 | 0.946 | 0 | 0.000 | 0.000 |
| llama.cpp Q8_0, CUDA, images fitted | 0.896 | 1.000 | 1.000 | 0.946 | 0 | 0.000 | 0.000 |

Reports: `2026-10-18-llamacpp-q8-vulkan-fitted.md`, `2026-10-18-llamacpp-q8-cuda-fitted.md`. The golden set is saturated
(recall is already 1.000), so it cannot show small losses; the cosine and top-10 numbers above are the finer evidence.

## S4: speed

Small set (`Sample Data/`, 385 chunks, 46 images, 92 queries, default `-ub 512`, one keep-alive connection):

| | chunks/s | images/s | query p50 | query p95 |
|---|---|---|---|---|
| torch bf16 (queries through CUDA graphs) | 61 | 6.7 | 6.4 ms | 9.3 ms |
| llama.cpp Vulkan Q8_0 | 60 | 5.7 | 6.6 ms | 7.0 ms |
| llama.cpp CUDA Q8_0 | 66 | 7.4 | 10.0 ms | 14.5 ms |

Real chunks (2,000 random text chunks, 100 images, 150 rendered PDF pages from the real index; the final configuration):

| | chunks/s | images/s | PDF pages/s |
|---|---|---|---|
| torch bf16 | 43.0 | 5.6 | 5.7 |
| llama.cpp Vulkan Q8_0 | 40.3 (94%) | 3.6 (64%) | 3.9 (68%) |
| llama.cpp CUDA Q8_0 | 50.0 (116%) | 6.1 (109%) | 4.9 (86%) |

Time from process start to `/health` ok: 3.7 s (Vulkan), 2.7 s (CUDA), files in the OS cache; the torch engine takes
10 to 35 s. An early run measured 30 ms per query; that was `urllib` opening a new connection per request. With one
keep-alive `http.client` connection it is 6.6 ms (the server itself spends 4 to 11 ms). The embedder must keep its
connection open.

## S5: footprint

| | torch engine now | llama.cpp, Vulkan | llama.cpp, CUDA |
|---|---|---|---|
| runtime | `sidecar\.venv` 4.52 GB (torch alone 4.39 GB; the rest 0.13 GB) | build 32 MB zip, 88 MB unpacked, plus the same 0.13 GB without torch | build 155 MB + cudart 423 MB zip, 714 MB unpacked (`cublasLt` 470 MB, `ggml-cuda` 143 MB) |
| model | bf16 safetensors, about 1.5 GB | Q8_0 text 310 MB + vision projector 555 MB = 865 MB | same |
| download to get file search | about 6 GB | about 1.1 GB | about 1.7 GB |
| VRAM, resident (nvidia-smi, nothing else running) | 0.8 GiB resident, 1.3 peak (from `gemma.py`, not re-measured) | about 1.1 GB | about 1.15 GB |

Torch is imported only in `embedding/gemma.py` and `embedding/query_graph.py`.

## Not measured

- The friend's RTX 50-series machine. The CUDA build lists architectures 1200 and 1210 (Blackwell) and the Vulkan build
  needs only the driver, but neither has run on one.
- A full re-index of the 56k-chunk index through llama.cpp (the sample comparison above stands in for it) and a long soak
  of the server for leaks.
- JPEG instead of PNG for images (PNG was used for fidelity; JPEG may be faster and lossy).
- Several requests in flight at once, which may raise throughput (the server runs four slots).
- With no `--device`, the Vulkan build picked the NVIDIA card on this laptop (an Intel iGPU is also listed); the
  supervisor should not rely on that and has to choose the discrete device from `--list-devices`.

## After the swap (packaging block D, 2026-10-18)

The production code (`LlamaEmbedder`, `LlamaServer`) replaced the spike code; these were measured through it.

| Check | Result |
|---|---|
| Golden eval, GPU (Vulkan, the discrete card chosen by name) | top1 0.896, recall@3 1.000, recall@10 1.000, MRR 0.946, forbidden 0, false alarms 0.000, missed nones 0.000: identical to the torch baseline, so `baseline.json` was re-recorded with `device: vulkan` and differs only there (`2026-10-18-packaging-d3-*.md`) |
| Golden eval, CPU only (`--cpu`, `--device none -ngl 0`) | the same numbers (`2026-10-18-packaging-d3-cpu.md`): the fallback costs no quality |
| The owner's torch-built 56k-chunk index (a copy), golden queries against it | torch query vectors and llama.cpp query vectors give identical metrics: top1 0.675, recall@3 0.883, recall@10 0.961, MRR 0.787, forbidden 0, false alarms 0.013, missed nones 0.267 (lower than on the sample folder because 56k chunks hold far more distractors). An index built before the swap stays valid |
| Engine start to ready, through the launcher's real-engine test | 6.3 s (torch: 10 to 35 s); the whole start, index of `Sample Data`, search and stop test: 24 s |
| A fresh engine folder built by hand in the installer's layout | `uv sync --frozen --no-dev --python 3.12` 23 s with a warm cache; the engine imports and runs |
| Python tests | 588 pass; ruff and pyright clean; torch, transformers, sentence-transformers, scipy and scikit-learn are gone from the project (the dev environment is 157 MB instead of 4.5 GB) |

Two failures found by tests that killed the engine, both fixed:

- The supervisor ended only the process it started. A virtual environment's `python.exe` is a launcher that runs the real interpreter
  as a child, so the engine and its `llama-server` stayed alive on their port. `execProcess.Kill` now ends the whole tree
  (`taskkill /T /F`); `exec_windows_test.go` checks it with a parent and a child.
- A `llama-server` outlived an engine that was terminated without running exit handlers (the way the launcher stops it), holding
  about 1 GB of RAM and of VRAM. The server is now placed in a Windows job that closes with the engine
  (`test_the_server_dies_when_the_engine_is_killed_without_a_chance_to_clean_up`).
