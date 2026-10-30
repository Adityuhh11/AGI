# 0008: Omni AI runs its own llama.cpp server, owned by the launcher

Date: 2026-10-28. Status: accepted. Evidence: `eval/2026-10-28-chat-model.md`.

## Decision

The chat model (LFM3-1.7B, QAD Q4_0 GGUF) is served by a `llama-server` that the Go launcher starts and stops itself
(`internal/llm`). It uses its own llama.cpp build (b11970, Vulkan), kept in `%LOCALAPPDATA%\Omni\chat\`, separate from the
search engine's.

## Why

- **Not the Python engine.** The engine is an optional 0.9 GB install; chat should not need it. The launcher already
  supervises one child process (`internal/sidecar`), so a second is cheap.
- **Not the engine's llama.cpp.** The model needs b11960 or newer; the engine is pinned to b11719 and its embeddings were
  validated on that build (ADR 0004). Moving it would mean re-running the golden eval for a change unrelated to search. The price
  is a second 37 MB zip for people who use both. The two pins become one when the engine is next moved on purpose.
- **Not a library inside Omni.** llama.cpp's Go bindings need cgo and a compiler toolchain in the build, tie the GPU backend to
  the launcher's build and put a model crash inside the launcher. A process can be killed to free every byte of VRAM, which is
  how "unload" works.
- **OpenAI-style HTTP.** The launcher talks `/v1/chat/completions` to a base URL and key. A remote provider, LM Studio or a
  different local model later is another URL, not another code path. Tool calls use the same API (`--jinja` gives structured
  `tool_calls`).

## Alternatives not taken

Ollama or LM Studio as a dependency (a second installer the person must run, not pinned, not ours to start and stop); the
engine's Python process serving chat (above); Q4_K_M and Q8_0 (larger, no better on the sample; Q8_0 half the speed).

## Consequences

- Two GPU residents (embedder about 1.1 GB, chat about 1.2 GB at 8k context); the CPU is the fallback for the chat model (34 t/s).
- The chat server flags and the per-request `enable_thinking` / `stop: ["</think>"]` pair are part of the contract with this model; a
  different model is a different set (spike again).
