# Chat model spike: LFM3-1.7B on llama.cpp

Date: 2026-10-28. Spec: `superpowers/specs/2026-10-28-ai-chat-design.md` section 12. Machine: the owner's laptop, RTX 4050 (6 GB), Vulkan.
The Omni engine's own `llama-server` (the embedder, 1,111 MiB) was running throughout, so every VRAM figure is *next to* the embedder.
Scripts were throwaway Python in the session scratchpad (a server started with the flags below, requests over HTTP); the numbers are
what they printed.

## Build and files

| What | Value |
|---|---|
| llama.cpp | `b11970`, `llama-b11970-bin-win-vulkan-x64.zip`, 37,346,727 bytes, sha256 `2d043e717386c03caa1f150fbcf6c37b513d9872ed44f5d41706b2fecc14617b`. `llama-server --version`: 0.8.0-dev (build 11970). The model needs b11960 or newer; the engine's b11719 cannot run it |
| Model | `LiquidAI/LFM3-1.7B-GGUF` at revision `4b423c332fd97961416f2e8b69467758fd27370d`, file `LFM3-1.7B-QAD-Q4_0.gguf`, 1,049,615,680 bytes, sha256 `43a7d09a0e9cba91b63eb7052488b9564569bddfd65754d2e426465e572a7452` |
| Others tried | `Q4_K_M` 1,099,627,808 bytes; `Q8_0` 1,846,238,496 bytes |
| Flags | `--jinja -c 8192 -np 1 -ngl 99 --device Vulkan1 --api-key <k> --no-webui` |

The release tag `v0.8.0` on the llama.cpp repository (what GitHub calls "latest") has no Windows assets; the `bNNNNN` tags do.

## Findings that change the spec

1. **Thinking can be chosen per request.** `chat_template_kwargs: {"enable_thinking": false}` in the request body. The template
   (`/props`) defaults it to true. Off, the prompt ends with a prefilled `<think></think>`; on, with `<think>`. The header button needs
   no restart. With thinking on, the reasoning arrives in `reasoning_content` (streamed), the answer in `content`.
2. **With thinking off, the model sometimes closes a block it never opened.** 3 of 12 answers on QAD Q4_0 (1 of 12 on Q4_K_M, 0 on Q8_0)
   ended the real answer, wrote `</think>` and then answered again. In each case the text before the tag was a complete answer.
   Sending `stop: ["</think>"]` when thinking is off removes it: 0 of 12 leaks, all 12 finished with `stop`.
3. **Thinking is long.** On six prompts with thinking on: 51, 208, 334, 380, 529 and 2,011 completion tokens (0.4 to 18.7 s at about 105
   tokens/s). A reply reserve of 2,048 is not enough with thinking on; the reserve is 4,096 then. (A first try with `max_tokens` 1,500 on
   a git question ran out inside the thinking and produced no answer.)
4. **Tool calls arrive structured.** With `--jinja` and a `tools` array, "Set a 25 minute timer for tea" streamed a `tool_calls` delta,
   `set_timer({"minutes":25,"label":"tea"})`. `chat_template_caps` reports tools, parallel tool calls and preserved reasoning.
5. **The sampling defaults are Liquid's.** `/props` reports temperature 0.1, top_k 50, repeat penalty 1.1 without being asked.
6. **Prompt cache matters little.** Consecutive turns reuse only a short prefix (`cache_n` 15 to 266), because the template renders earlier answers with a
   `<think></think>` prefix that the generation did not have. It costs nothing visible: prompt processing runs at 6,500 to 6,850 tokens/s
   from 770 tokens up (0.15 s for 772, 0.26 s for 1,477, 0.46 s for 2,910, 0.79 s for 5,112). A full 8k re-read is about 1.2 s. So the spec's
   "trim in steps so the cache stays valid" is dropped: oldest whole turns are removed until the prompt fits.

## Speed (llama-bench, 512-token prompt, 128 generated, GPU, embedder loaded)

| Quantization | File | Prompt | Generation |
|---|---|---|---|
| QAD Q4_0 | 993 MiB | 6,583 t/s | 130 t/s |
| Q4_K_M | 1.02 GiB | 6,809 t/s | 140 t/s |
| Q8_0 | 1.71 GiB | 5,933 t/s | 85 t/s |

CPU only (`-ngl 0 --device none`) on QAD Q4_0: prompt 164 t/s, generation 34.6 t/s. Slow to read a long prompt (a 2,000-token prompt is
12 s) but a short chat reads at a comfortable speed.

Through the server, a short answer (about 50 tokens) takes 0.4 to 0.6 s end to end; first content token about 0.6 s on a cold prompt.

## Which quantization

Twelve launcher-style prompts (arithmetic, regex, translation, polite rewrite, definition, summary, fact, PowerShell, two capitals,
names, a word problem, a unit conversion), answers read by me, not blind:

- QAD Q4_0 gave the correct regex (`\(\d{3}\) \d{3}-\d{4}|\d{3}-\d{3}-\d{4}`); Q4_K_M's was looser; Q8_0's was wrong (it does not match `(555)`
  as asked). Q4_K_M put the English word "railway" in a French sentence. Arithmetic, facts, summary, PowerShell and the unit
  conversion were right on all three.
- Q4_0 is also the smallest and, with Q4_K_M, the fastest.

**Ship QAD Q4_0.** The sample is small and was read by one person; the owner can compare in the real chat. Raw answers are not
kept in the repository.

## Memory and loading

| Context | Chat server VRAM (next to the embedder) |
|---|---|
| 4,096 | +1,125 MiB |
| 8,192 | +1,177 MiB |
| 16,384 | +1,281 MiB |
| 32,768 | +1,489 MiB |

Embedder 1,111 MiB + chat at 8,192 = about 2,290 MiB of 6,141 MiB, with the Windows desktop on it. Context costs only about 13 MiB per
1,000 tokens (the model has 6 global-attention layers), so a larger context is a cheap choice.

- Start to ready: **11.3 s** the first time after the download (file not yet in the OS cache, driver caches cold), **1.6 s** on every
  later start. A reboot is likely to bring the longer one back; the window must say "Starting the model…" and queue the message.
- Ending the process returns the VRAM at once (1,111 MiB again, 0.78 s to end).

## Not measured, and one unexplained thing

- **An answer during a fast sync, and the chat model with the sync running** (spec 12.4) were not measured: no sync was run. The
  figures above are with the embedder loaded and idle. This goes in the acceptance step of the build, with Omni's real indexing running.
- **Two stalls.** In the first two multi-turn runs, 3 of about 20 turns spent 8 to 11 s in prompt processing (about 75 t/s instead of
  about 4,000) and then generated normally. Nothing was running from Omni then that I know of, GPU use was 0% idle, and in six later
  runs (about 60 turns) it never happened, on fresh and on reused servers. It is not a shader-compile cost (prompts of 8 to 5,112
  tokens were fast on a fresh process). Cause unknown; most likely another program using the GPU at that moment. The window must show
  progress ("Reading…" until the first token) so a stall looks like work, and the acceptance step looks for it again.
- The quality comparison is not the blind 30-prompt test the spec asked for; see above.
