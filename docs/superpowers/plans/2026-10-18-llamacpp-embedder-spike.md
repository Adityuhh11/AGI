# llama.cpp embedder spike (plan)

**Question.** Can the memory engine embed with llama.cpp and a quantized GGUF model instead of torch, with the same search
quality, fast enough, and a download small enough to ship? The answer decides the shape of the file-search install (packaging
phase P4): torch through `uv` (several GB) or llama.cpp (target: about 1 GB with the model).

**Why now.** Torch with CUDA is the one thing that makes the engine too heavy to hand to other people. Everything else in
packaging (engine optional, settings page, indexing control) works either way, so this is the experiment that gates P4.

**Output.** `docs/eval/2026-10-18-llamacpp-embedder.md` (numbers) and, when the spike ends, `docs/decisions/0004-embedding-runtime.md`
(go or no-go and why). Spike code lives in `sidecar/spikes/` and is deleted or promoted by the decision, never left half-way.

## Rules

- Measure on the real data (`Sample Data/`) with the real golden set. No result is "close enough" by feel: the gate is
  `docs/eval/baseline.json` (any metric drops more than 0.02, or a `forbid` query surfaces, is a fail).
- Only models use the GPU, and 6 GB VRAM is shared: stop the launcher's engine before every GPU measurement.
- Downloads come from public GitHub releases and Hugging Face only; record each URL, file and size in the report.
- Same text format as today, byte for byte: `title: {name} | text: {text}` for documents, the model's `SearchQuery` prompt
  for queries, Matryoshka truncation to 768 then L2 normalisation (see `embedding/gemma.py`). A difference in the prompt
  would look like a difference in the quantization.

## Steps

**S0. Is it possible at all?** Find the EmbeddingGemma 2 GGUF files (text, and the vision projector for images) and a
llama.cpp release binary that loads them. Record: repo, file names, quantization options and sizes, the llama.cpp build
number, and whether `llama-server` can return an embedding for an image and for a text, and how (endpoint, request shape).
If images cannot be embedded through llama.cpp, stop here and write the decision: image search is core, so a text-only
llama.cpp is no-go for the whole engine (a hybrid with torch only for images keeps the torch download and is not worth it).

**S1. Text fidelity.** Take every text chunk the chunkers produce from `Sample Data/` (the real ones, with their titles).
Embed them with the current torch bf16 path and with llama.cpp at Q8_0, plus F16 as the control. Report the cosine between
each pair: minimum, 5th percentile, mean. Same for 50 queries from the golden set. Expectation to check, not assume:
the Q8_0 vs bf16 gap is of the same order as the measured bf16 vs fp32 gap (cosine >= 0.99995 on the 4-sentence test).

**S2. Image and PDF-page fidelity.** The same comparison for every image in `Sample Data/` and for the rendered PDF
pages. Images have no keyword fallback, so a larger gap here hurts more than for text.

**S3. Search quality.** Write `sidecar/spikes/llama_embedder.py`: an `Embedder` (the contract in `embedding/base.py`) that
starts `llama-server` as a child process and calls it over localhost. Build the full index with it and run the golden
queries through the same `evaluate` and `summarize` the eval harness uses. Report top1, recall@3, recall@10, MRR, false
alarms, missed nones and forbidden hits next to `baseline.json`. Re-run the confidence calibration (`search/calibration.py`)
on the new index, since the noise ceiling is measured per index.

**S4. Speed.** On the dev laptop (RTX 4050, 6 GB), for each backend that works (CUDA build, Vulkan build, CPU):
indexing throughput on `Sample Data/` in chunks per second, image throughput, and query latency p50 and p95 for a
single query (the launcher path today is about 7 ms for the embedding and 16 ms p50 for the whole search; the bar must
not get slower than the hotkey budget allows). Include time from process start to first embedding (the torch engine takes
10 to 35 s to be ready).

**S5. Footprint.** Sizes on disk and in VRAM, side by side with torch: `llama-server` and the DLLs each backend needs,
the GGUF files, resident and peak VRAM during a batch; and, for the torch side, the size of `sidecar\.venv` and of the
torch CUDA wheels that `uv sync` downloads. This is the table that decides the download size we promise.

**S6. Decision.** Write ADR 0004. Go only if all hold:

| Gate | Threshold |
|---|---|
| Golden metrics | every metric within 0.02 of `baseline.json`; no forbidden hit; no new false alarms |
| Images | all of `Sample Data/` embeds, ranking of the image queries does not drop below the baseline |
| Indexing speed | at least half the torch throughput on the same GPU |
| Query latency | p95 of one embedding under 30 ms |
| Download | engine plus model under about 1.5 GB |

If Q8_0 fails on quality and F16 passes, take F16 and report its size. If both fail, no-go: P4 installs the torch engine
with `uv` and the ADR records why.

## After a go

The index is tied to its model: the quantization goes into `model_id` (for example `...-q8_0`), so changing it means a
rebuild, which the store already enforces. `GemmaEmbedder` stays until the new embedder has run on the owner's real index
for a while. The engine's other modules do not change; the swap is behind the `Embedder` contract.
