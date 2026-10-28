# 0004: llama.cpp with the 8-bit GGUF model replaces torch as the embedding runtime

Date: 2026-10-18. Status: accepted for the shipped engine (the swap itself is a later phase). Numbers: `eval/2026-10-18-llamacpp-embedder.md`.

## Decision

The file-search engine will embed through `llama-server` (llama.cpp) running `ggml-org/embeddinggemma-2-GGUF` at Q8_0 (text
310 MB, vision projector 555 MB), instead of torch and sentence-transformers. The Python engine stays; only `GemmaEmbedder`
(and `query_graph.py`) are replaced by an `Embedder` that talks to a `llama-server` child process, which the supervisor starts
the way it starts the engine. The default build is Vulkan (32 MB download); the CUDA build is an optional faster pack.

How the client must call it (each of these was measured to matter):

- one keep-alive HTTP connection (a new connection per request cost 20 ms a query);
- texts over 400 characters are tokenized by the server's `/tokenize`, cut to 511 tokens plus the closing token, and sent as
  token lists (longer inputs are a server error);
- images are resized by the client to the model processor's size (about 2520 patches of 16 px, aspect kept, each side a
  multiple of 48, bicubic) before sending, and the server runs with `-ub 1024 --image-max-tokens 280`;
- the discrete GPU is chosen explicitly from `--list-devices` (the laptop also lists an Intel iGPU).

## Why

Torch is 4.39 GB of the 4.52 GB engine environment, because it carries its own CUDA libraries, and it cannot be slimmed
safely. That made the engine undeliverable to anyone else (about 6 GB with the model). The same model through llama.cpp:

| | torch now | llama.cpp Vulkan | llama.cpp CUDA |
|---|---|---|---|
| download for file search | about 6 GB | about 1.1 GB | about 1.7 GB |
| ready after start | 10 to 35 s | 3.7 s | 2.7 s |
| golden set (92 queries) | 0.896 / 1.000 / 1.000 / 0.946 | identical | identical |
| vectors vs torch, 760 real chunks, pages, images (min cosine) | | 0.994 | |
| chunks / images / PDF pages per second, real data | 43 / 5.6 / 5.7 | 40 / 3.6 / 3.9 | 50 / 6.1 / 4.9 |
| query p50 / p95 | 6.4 / 9.3 ms | 6.6 / 7.0 ms | 10.0 / 14.5 ms |

Every gate of the spike plan held: golden metrics equal to the baseline, every image embeds, throughput at least half of torch
(worst case 64%), query p95 far below 30 ms, and Vulkan plus model under 1.5 GB. Q8_0 is enough: BF16 weights moved text
cosines from 0.99988 to 0.99996, and before the image fix they gave the same image cosines as Q8_0.

## Alternatives

| alternative | outcome |
|---|---|
| keep torch, install it on demand with `uv` | works, but 6 GB for every user; kept only as the fallback if the swap fails in the field |
| strip torch's CUDA libraries | fragile (a missing library is a crash on someone else's GPU); not attempted |
| bake torch into the installer | multi-GB installer |
| text-only llama.cpp, torch for images | keeps the torch download, so no gain |
| BF16 GGUF instead of Q8_0 | 558 + 982 MB; no measurable quality gain |

## Consequences

- The `model_id` stays `google/embeddinggemma-2` (amended 2026-10-18, after the swap): the two sets of vectors agree to cosine
  0.9999 for text and 0.994 or better for images and pages, and the owner's 56k-chunk index, built with torch, gives
  identical golden-set results with torch query vectors and with llama.cpp query vectors (top1 0.675, recall@10 0.961,
  MRR 0.787 for both; measured on a copy of the index with a throwaway script). Nobody rebuilds an index because of this change.
  A later change of model, dimension or quantization that the same measurement does not clear still changes the id.
- The engine owns the server's life: the server is placed in a Windows job that dies with the engine (the launcher kills the
  engine without running exit handlers, and an orphaned server holds a gigabyte of RAM and of VRAM), and the supervisor
  kills the engine's whole process tree (a venv's `python.exe` is a launcher that runs the real interpreter as a child).
  Both were found by the tests that killed the engine, not by reading code.
- Cosine rankings are not identical (top-10 overlap 0.973, same top-1 0.989 on the sample set). The golden set cannot show
  the difference because it is saturated; the golden set needs harder queries before another model change is judged by it.
- Images and PDF pages run at 64 to 68% of torch's speed on Vulkan (CUDA: 86 to 109%). The client's PNG encoding and
  resizing are part of that; JPEG and concurrent requests are untested ways to close it.
- Audio was not in scope here. (Corrected 2026-10-28: the projector file does contain the audio encoder, and the server
  embeds audio through it; ADR 0007.)
- Not yet run on the friend's RTX 50-series machine, nor over a full re-index of the real 56k chunks. The packaging phase
  (P4) must repeat the fidelity sample and the golden eval on the final binaries, and test an iGPU-only machine.
- `sidecar/spikes/` holds the throwaway scripts and the first `LlamaEmbedder`; the real embedder is written in the packaging
  phase and the spikes are deleted then.
