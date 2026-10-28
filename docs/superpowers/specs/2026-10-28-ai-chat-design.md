# Omni AI: a local chat window

Date: 2026-10-28. Status: **owner's answers folded in (section 15); ready for approval.** Nothing is built. Branch when approved: `feat/ai-chat`, cut from `dev`.

The owner wants an optional local language model in Omni, first as a chat. A person types `ask` in the bar (or presses a
hotkey), and a resizable window that stays on top opens and answers questions. The model is Liquid AI's LFM3-1.7B, quantized,
served by llama.cpp. Tools, agents, other models and the person's own API come later; this spec only keeps the door open for
them (section 11).

## 1. What was checked, and what was not

From the model pages (2026-10-28):

- LFM3-1.7B: text only, 1.73 B parameters, 24 layers of which only 6 are global attention (13 are short convolutions, 5
  sliding-window), so its attention memory stays small as the context grows. 256K context, 16 languages, tool calling and
  structured output. ChatML-style template; the assistant turn opens a `<think>` block, so **reasoning is on by default**.
- Recommended sampling for chat, tools and agents: temperature 0.1, top_k 50, repetition penalty 1.1.
- `LiquidAI/LFM3-1.7B-GGUF`: Q4_0 1.05 GB, Q4_K_M 1.1 GB, Q5_K_M 1.26 GB, Q6_K 1.43 GB, Q8_0 1.85 GB, F16 3.47 GB, plus
  `LFM3-1.7B-QAD-Q4_0.gguf` (Q4_0 trained to recover what 4-bit loses). The page says about 1 GB of memory for the 4-bit one.
- **Needs llama.cpp b11960 or newer. The engine is pinned to b11719** (`internal/enginesetup/pins.go`). See section 3.
- Licence: "LFM Open License v1.0" (`lfm1.0`). The model is downloaded by the person's own choice, never bundled in
  Omni's zip or installer. The owner has read the licence and cleared it for Omni's use (2026-10-28).
- Liquid says it is not meant for agentic coding or long multi-step reasoning. A 1.7 B model is for quick questions, rewrites,
  explanations, translation and summaries. The chat's empty state says so in one line.

Not checked, and therefore **section 12 is a measured spike that comes before any code**: that b11960 Vulkan runs the model
on the RTX 4050, how much VRAM it takes next to the embedder, tokens per second, load time, which quantization to ship, and
how `llama-server` should be told to turn thinking off. Numbers in this spec marked *(to measure)* are placeholders.

## 2. Principles that decide the rest

1. **Optional and off by default.** Not downloaded, not started, not mentioned in the bar until the person turns it on.
2. **The hotkey to bar path never waits on the model.** Nothing in this feature runs when the bar opens. The chat window
   opens at once; the model loads behind it.
3. **Independent of file search.** Chat works with no Python engine, and the engine works with no chat. They share the GPU, so
   they must fit together in 6 GB, but they do not share a process, a folder or a version.
4. **Local.** After the one download, no request leaves the machine. No telemetry. The only server on this computer is a
   `llama-server` on a random loopback port behind a random API key, the same arrangement as the engine's.
5. **The person's data stays plain.** Conversations are files in a folder they can open, like notes.

## 3. Architecture

```
chat window (Svelte)  <-- Wails events (tokens) --  internal/chat  --OpenAI-style HTTP-->  llama-server (LFM3)
        |  bound methods (send, stop, list, delete)        |  conversations, context window        ^
        v                                                  v                                       |
   internal/chat.Service ------------------------>  internal/llm  (install, start, idle unload, GPU choice)
```

Two new packages, each with one job:

- **`internal/llm`**: the model as a process. Installs it (llama.cpp zip and the GGUF, pinned by size and SHA-256, resumable,
  through the existing `enginesetup.Downloader` and `Unzip`), picks the device, starts `llama-server`, waits for `/health`,
  restarts it once if it dies, stops it after the idle time, and exposes `Ensure(ctx)` (start if needed, return the base URL
  and key) and `State()` (`off`, `unloaded`, `loading`, `ready`, `failed`). Nothing in it knows about conversations.
- **`internal/chat`**: conversations. The store, the context window, the streaming call, and the `Service` the page calls.
  It talks to any OpenAI-compatible endpoint (`/v1/chat/completions`, streamed), which today is the `llm` server.

**Why Go owns the server and not the Python engine.** The engine is optional and a 0.9 GB install (uv, Python, libraries). Making
chat depend on it would make a 1 GB model need a 1 GB engine. A Go process supervisor is about 150 lines; the engine's
pattern (job object so it dies with Omni, taskkill of the tree, output to the log) already exists in `internal/sidecar` and is
reused, not copied: `sidecar.execStarter` and `KillWithLauncher` are the pieces to export or lift.

**Why its own llama.cpp, not the engine's.** LFM3 needs b11960+; moving the engine from b11719 would mean re-running the golden
eval and the embedding comparisons for a change that has nothing to do with search. So chat pins its own build in its own
folder. The cost is a second 32 MB zip for people who turn both on. When the engine is next moved on purpose, the two pins can
become one; that is a clean-up, not a design.

**Device choice.** Same rule as the engine's (`choose_device` in `sidecar/.../llama_server.py`): the discrete GPU by name from
`--list-devices`, never "the largest memory" (an integrated GPU reports shared memory), the CPU if it will not start. That is a
port of about 15 lines to Go, tested on the same sample listings. A graphics card that runs out of memory moves the model to the
CPU for the session, as the engine's embedder does (`sidecar.Explain` already words the advice).

**One slot.** `llama-server -np 1`: parallel slots split the context between them, and there is one window and one answer at a time.

**Files.**

| What | Where |
|---|---|
| llama.cpp and the model | `%LOCALAPPDATA%\Omni\chat\` (`llama\`, `models\`, `chat.json` manifest written last); under `[search] data_dir` when the person chose one, like the engine |
| Conversations | `%APPDATA%\Omni\chats\<id>.json` (next to `notes\`) |
| Settings | `[chat]` in `config.toml` |

## 4. The window

Same host as Notes (`settingsui.Window`, a new `chatSpec`): route `chat`, 460 x 640, minimum 340 x 320, resizable, **always on
top** with the pin button Notes has (it turns that off and on). Built the first time it is asked for. A second request brings
the open one forward. It follows the existing look: black plate, Geist, the capsule/card language of the bar, `theme.css` tokens,
sharp at 125% (no scaled text, no opacity fades on text, as in the 8h sharpness pass).

```
┌──────────────────────────────────────────┐
│ ● Omni AI              [Think] [+] [≡] [📌]│  status dot, thinking, new chat, history, pin
├──────────────────────────────────────────┤
│                                          │
│                    how do I undo a git…  │  user: right, soft fill
│                                          │
│  You can undo the last commit with …     │  assistant: plain text, markdown
│  ┌────────────────────────────┐ [copy]   │  code blocks have their own copy
│  │ git reset --soft HEAD~1    │          │
│  └────────────────────────────┘          │
│  ── earlier messages left the model's ── │  only when the context was trimmed
│     memory                               │
├──────────────────────────────────────────┤
│ Ask anything…                       [ ↑ ]│  Enter sends, Shift+Enter newline
│ runs on this computer · 3.1k / 8k        │  context meter, quiet
└──────────────────────────────────────────┘
```

- **Streaming.** Go reads the server's stream and emits a Wails event per batch of tokens (batched to one per frame, not per
  token). The page appends to the message and re-renders its markdown once per animation frame. `lib/markdown.ts` is reused as
  is: it escapes everything first, so a reply can never run script. Tables are not supported by it and are not added now.
- **Keys.** Enter sends. Shift+Enter is a new line. Esc stops a running answer (the partial text is kept, marked "stopped");
  with nothing running it closes the window. Ctrl+N starts a new chat. Ctrl+H opens history. Up arrow in an empty box
  recalls the last message to edit.
- **While the model loads** (first use, or after it unloaded) the box is usable: the message is queued and the status dot pulses with
  "Starting the model…". It sends itself when ready. Nothing blocks on the load.
- **Copy** on each answer and on each code block. Regenerate on the last answer.
- **Thinking toggle.** A "Think" button in the header (lit when on) flips whether the model reasons before it answers, from the
  next message on, without restarting the model. It is the same switch as `[chat] thinking` in Settings: pressing it changes the
  setting, so the two never disagree. When on, the reasoning is shown as a collapsed "Thought for 4 s" line above the answer,
  closed by default; the status line reads "Thinking…" while it runs, and Esc stops it like any answer. This needs the server to
  take the choice **per request**; if it only takes it at launch, the button would mean a restart, and the spike (section 12)
  decides what to do instead.
- **Empty state:** one line of what it is ("Runs on this computer. It is a small model: check anything that matters."), no
  suggestion chips.
- **History** (≡): a panel over the messages listing conversations, newest first, with a search box, delete, and "Clear all".
- **Errors** are advice, not raw text, in the style of `sidecar.Explain`: "The graphics card ran out of memory; Omni switched the
  model to the processor, which is slower." The raw text goes to `omni.log`.
- **Closing the window** stops a running answer (the partial text is kept) and starts the idle clock; it does not unload at once
  (section 7).
- Accessibility basics: the log is an `aria-live="polite"` region, every button has a name, the focus order is box → send → header,
  and reduced motion removes the dot pulse and the smooth scroll.

## 5. Ways in

1. **The bar.** Typing `ask` (or `ai`, or `chat`) finds the command "Ask Omni AI" (matched like the other commands: the start of a
   word). `ask: words` is a mode like `note:`: the first row is **Ask “words”** (Enter opens the window and sends it), then
   the conversations whose text contains the words, newest first (Enter reopens one). Plain `ask` with nothing after shows
   "New chat" first and the last five conversations. Tab does not cycle into it, like Clipboard, Snippets and Notes. If chat is
   off, `ask` shows one row, "Turn on Omni AI…", which opens Settings on its page; it never starts a download from the bar.
2. **A hotkey.** `[chat] hotkey` (empty by default), set in the same capture field as the clipboard's. Pressed, it shows the chat
   window with the box focused; pressed again while the window is focused, it hides it. It does not go through the bar.
3. **The tray**: "Omni AI…" next to Notes, when chat is on.

**Warm-up on intent.** Entering the `ask` mode, or pressing the chat hotkey, starts loading the model in the background
(`llm.Ensure` on a goroutine). By the time a question is typed and Enter is pressed it is often ready. The bar's own path
only fires the goroutine; it does not wait. If the person leaves the mode, the load still finishes and the idle timer decides
when it goes.

## 6. Do we keep the history? Yes, locally, with limits.

**Decision (owner, 2026-10-28): keep conversations, on by default once chat is on, deletable in one click, with a per-chat "don't save".**
A "Clear history" button is in the history panel and in Settings, each asking once.

For keeping:
- A launcher's chat is used in short bursts all day. People return to "that command it gave me" or the regex it wrote; without
  history that answer is gone the moment the window closes, and the reflex becomes copying everything out, which is worse for
  privacy than a file.
- A follow-up question ("and on Windows?") needs the earlier turns anyway, so the conversation exists in memory regardless.
  Persisting it is writing a small file.
- The bar can list and reopen past chats (`ask: regex`), so chat joins the launcher instead of being a separate app.
- It costs nothing: a long conversation is tens of kilobytes of text.

Against, and how each is answered:
- **Privacy on a shared computer, or pasted secrets.** Everything stays on the computer, in plain files the person can see and
  delete, like notes. Controls: `keep_history` off (nothing is written; closing the window forgets the chat), a "Don't save
  this chat" toggle in the header for one chat, `history_days` auto-delete (default 90, 0 = never), "Clear all" in Settings and in
  the history panel. Unlike the clipboard history (off by default because it records what the person did *not* decide to
  write down), chat records only what they typed to Omni on purpose, so on by default is consistent with Notes.
- **Not encrypted.** Same as notes and `state.json`. Said in Settings next to the switch: "Saved as plain files in
  %APPDATA%\Omni\chats".
- **Clutter.** Search, the 90-day default and one-click delete.

Format: one JSON file per conversation, written atomically after each finished answer (the same temp file and rename as
`notes.Store`): `{id, title, created, modified, messages: [{role, text, thinking?, tokens, stopped?}]}`. The title is the first
line of the first message cut at 60 characters (no model call to name chats; a 1.7 B model titling is slow, often wrong, and
needs a second generation). Rename is possible in the history panel. The conversations are not indexed by the file engine.

## 7. Context: what the model sees

The window shows the whole conversation. The model sees a **context window** built from it for each request, because the context is a
fixed number of tokens set when the server starts (`-c`), and memory and speed both grow with it.

**Size.** `[chat] context_tokens`, default 8192 *(to measure: VRAM and speed at 4k, 8k, 16k, 32k)*, choices 4096, 8192, 16384, 32768.
The model can do 256K, but a launcher chat does not need it, a 1.7 B model gets worse as the prompt gets long, and a larger
`-c` costs VRAM that file search and games also want. Changing it restarts the model at the next use.

**What goes in, in order:**
1. A short fixed system prompt: who it is, to be concise, to answer in the language the person writes in, today's date (a small
   model does not know it), and nothing about abilities it does not have. It never changes mid-conversation, so the server's
   prompt cache keeps matching.
2. The conversation's messages, oldest to newest, as many as fit.
3. A reserve for the reply: `max_tokens` = min(2048, a quarter of the context).

**Counting.** Exactly, not by guess: the server's `usage` on each response gives the tokens of the prompt and of the reply, which
are stored on the messages; a new message is counted with the server's `/tokenize` before sending. No character heuristics.

**Trimming.** When the prompt would pass 85% of the room, drop the oldest *whole turns* (a question and its answer) from the
front until it is down to 60%, never splitting a turn, never dropping the newest message. Dropping in steps, not one turn at
a time, matters for speed: llama-server keeps the processed prompt in its KV cache and only computes what changed after the
shared start. Trimming the head invalidates that cache, so trimming a little every turn would reprocess the whole prompt
every turn; trimming in steps does it once every few turns. *(to measure: prompt processing time with and without the step.)*

**Visible, not silent.** The window draws a line where the model's memory begins ("earlier messages left the model's memory") and
the footer shows used/total tokens. A single message longer than the whole room is refused with a sentence saying so; it is
never cut in half.

**Thinking tokens are not resent.** A reasoning block belongs to the answer it came with; it is kept for display and left out of
later requests. It would otherwise eat the context.

**Left out: summarising dropped turns.** A summary from a 1.7 B model is lossy and costs a generation. Not now; the trim line is
honest about what was lost. Later, if long chats turn out to matter, it is the one extension that fits here.

## 8. Loading: when the model is in memory

The model is a 1 GB file the GPU must hold while it answers. Keeping it there all day wastes VRAM the embedder, indexing and games
use; starting it for every question costs seconds. So the person chooses, and the default is on demand.

| Setting | Values | Default |
|---|---|---|
| `[chat] load` | `when_asked`: start at the first question (or the warm-up on intent); `at_start`: start with Omni | `when_asked` |
| `[chat] unload_after_minutes` | 1, 5, 10, 30, 60, 0 (never) | 10 |

Minutes count from the **end of the last answer**, not from the window closing, so a person who steps away from an open window also
frees the GPU. Unloading is ending the `llama-server` process: it returns all of its VRAM, and starting again is the load
again. (llama.cpp has its own sleep mode; ending the process is simpler, portable across builds and also drops the GPU context.)
`at_start` starts the model 15 s after Omni is up, never on the path to the first bar.

Settings shows the state live ("Unloaded", "Loading…", "Ready, 1.4 GB in use, unloads in 7 min") and a button to unload now.
Cold load, warm load (file in the OS cache) and first-token times are measured in the spike and written here when known.

Other settings:

| Key | Meaning | Default |
|---|---|---|
| `enabled` | chat is on (downloaded) | false |
| `hotkey` | opens the chat window | empty |
| `context_tokens` | section 7 | 8192 |
| `keep_history`, `history_days` | section 6 | true, 90 |
| `thinking` | let the model reason before it answers | false |

`thinking` is off by default: it multiplies the tokens before the first visible word, and the point of this model is speed. It
is switched from the header button (section 4) or here. When on, the thinking is collapsed. How to turn it off in `llama-server`
for this model, and whether that can be chosen per request, is part of the spike.

Not settings, because they never vary: the sampling values (0.1 / 50 / 1.1 from Liquid's card), the port and key, the slot count.

## 9. Settings, install and removal

A new Settings page **Omni AI**, laid out like File search (cards):

1. **Switch and install.** Off: a card says what turning it on downloads (llama.cpp ≈ 32 MB, the model ≈ 1.1 GB), from where
   (`github.com`, `huggingface.co`), the licence with a link, and a button. Progress with cancel, resumable, checked by SHA-256.
   `enginesetup.Hosts` gains a chat list for the same "before you agree" display.
2. **Behaviour:** the load choice, idle time, context size, thinking, hotkey.
3. **History:** keep, days, "Clear all", where the files are.
4. **Status and remove:** live state, unload now, "Remove Omni AI" (stops the server, deletes `chat\`; conversations are asked
   about separately, as the uninstaller asks about the index).

The first-run guide does **not** offer it. File search is the headline feature; this is found in Settings and by typing `ask`.
`omni.exe --remove-data` and the installer's uninstall learn the `chat` folder (it only deletes folders named Omni).

**Privacy docs.** The rule in `AGENTS.md` ("nothing leaves the machine except … requests the user starts or switches on")
already covers this; the README's privacy table gains one row: turning on Omni AI downloads from github.com and huggingface.co once,
and nothing after that.

## 10. Hardware and performance rules

- Both models on the GPU: the embedder takes about 1.1 GB; the chat model about 1.05 GB plus its context memory *(to measure)*.
  Together they must leave headroom on 6 GB, with the Windows desktop and a browser running. If they do not, the chat model
  falls back to the CPU and says so in the window ("running on the processor").
- Indexing and chat both want the GPU. Chat is never paused for indexing; the indexer's pace setting already lets the person
  slow indexing. The spike measures an answer during a fast sync.
- Acceptance: the hotkey → bar p50 does not move (`panelbench`, with chat on and off); the chat window paints in under 150 ms of
  the request *(to confirm)*, independent of the model's state.
- **Check VRAM before trusting any timing.** The owner's LM Studio also runs a `llama-server` on this GPU (see memory note);
  the spike records free VRAM before each run.

## 11. What this leaves room for (and does not build)

- **Another model, or the person's own API.** `chat` speaks the OpenAI-style API to a base URL and key. Today that is the local
  server; a remote provider, LM Studio or Ollama is the same calls with another URL, with the key kept in Windows Credential
  Manager (never `config.toml`). No provider interface is written now: there is one implementation, and the HTTP API *is* the
  interface. Changing the local model later is `llm`'s pinned artifact becoming a setting.
- **Tools and an agent loop.** The API already carries `tools`, and with `--jinja` the server turns the model's `<tool_call>` text
  into structured calls. A later `internal/chat` addition runs the loop: model asks → Omni runs a tool it owns (search files,
  open an app, make a note, set a timer: each already a Go service) → result goes back. Tools that change anything ask the
  person first. None of that is designed here; only the choice of server flags (`--jinja`) keeps it possible.
- **Files in the answer** ("what did I write about X?"): the engine's search as a tool. Later, and it needs the engine.

## 12. The spike (first step after approval, before any code)

Everything marked *(to measure)*, on the real GPU, written up in `docs/eval/<date>-chat-model.md`:

1. llama.cpp b11960 or newer (Vulkan) runs `LFM3-1.7B`; record the exact build and zip SHA-256 to pin. Does `--jinja` give structured
   `tool_calls`? What turns thinking off, can it be chosen per request (the header button needs that), and where does
   `reasoning_content` arrive when it is on?
2. Quantization: QAD Q4_0 against Q4_K_M and Q8_0: tokens/s (prompt and generation), VRAM at 4k/8k/16k/32k context, and ~30
   prompts of the kind a launcher gets (a quick fact, a rewrite, a regex, an explanation, a translation, a summary), answers
   judged blind by the owner. The smallest file that does not lose in that comparison ships. Pin its size and SHA-256.
3. Cold start to ready, warm start to ready, first token after ready, and time to free the VRAM on unload.
4. The embedder and the chat model together, then with a sync running; and the chat model on the CPU alone.
5. Prompt-cache behaviour of the trim step (section 7): follow-up latency with a cache hit against after a trim.
6. The 85% / 60% thresholds and the 2048 reply reserve against what the chats actually look like.

The spike's results can change this spec; if so the spec is amended before the plan is written. An ADR (`0008`) records the
runtime choice: chat has its own llama.cpp pin and server process, separate from the engine's.

## 13. Build order (becomes the plan)

1. Spike (section 12) and the ADR.
2. `internal/llm`: pins, layout, install/remove, device choice, process, idle unload. Tests: device choice on sample listings, idle
   timer on a fake clock, a stand-in server script (as `sidecar` does) for start, restart and failure.
3. `internal/chat`: store (table tests: atomic write, ids cannot escape the folder, expiry), context builder (table tests: trim
   steps, newest never dropped, oversize refused), streaming client against a fake SSE server, `Service`.
4. Config keys and validation (`[chat]`), Settings page, install progress (the File search page's pattern).
5. The window, markdown reuse, streaming render, history panel; checked by building the app and opening it at 125%.
6. The bar's `ask` mode and rows, the hotkey, the tray item, warm-up on intent.
7. **Trial, then decide:** the hotkey starting a question from the clipboard's text (the clipboard, not the selection: reading a
   selection means sending Ctrl+C to another program, which is fragile). Built small behind the hotkey, tried by the owner for a
   few days, kept or deleted. It is not part of the acceptance of this feature.
8. Measure the acceptance items in section 10; docs (`STATUS.md`, `ARCHITECTURE.md`, README privacy row, `docs/README.md` index);
   then the owner's review, then into PR 39.

## 14. Deliberately not here

Voice, images in and out, attachments, a system-prompt editor, per-chat model choice, summarising old turns, titling chats by
the model, syncing or exporting conversations, a second chat window, streaming to anything but the chat window, and any use of
the chat model by other features. Each is possible later; none is needed to ask a question.

## 15. The owner's answers (2026-10-28)

1. **History:** on by default, with a Clear history button (section 6).
2. **Names:** "Omni AI", found by `ask`, `ai` and `chat`, for now; easy to change later.
3. **Hotkey with text:** undecided. Tried as a small trial after the main build (section 13, step 7), then kept or dropped.
4. **Licence:** read and cleared by the owner.
5. **Thinking toggle:** a button in the chat header, same switch as the setting (section 4).
