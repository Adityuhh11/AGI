# Packaging: a launcher anyone can run, an optional file-search engine, a settings window

Date: 2026-10-18. Status: approved to build by the owner on 2026-10-18 (review continues while it is built). Branch `feat/packaging`. Builds on the launcher
(`2026-10-15-launcher-design.md`), the palette (`2026-10-17-palette-design.md`) and the memory engine
(`2026-10-12-memory-engine-design.md`, `docs/api.md`). Measurements behind the engine choice: `docs/eval/2026-10-18-llamacpp-embedder.md`,
ADR `docs/decisions/0004-embedding-runtime.md`.

## Goal

A friend (Windows 11, NVIDIA GPU, a Raycast power user) gets one zip, unpacks it, runs `agi.exe`, and has a working launcher: apps,
web search, quicklinks, tags, the hotkey he chooses. File search by meaning is something he turns on when he wants it: a button
downloads about 1.1 GB, he picks folders one at a time, and the index builds slowly in the background at a speed he sets. Nothing
about the machine it was built on is assumed. He sends back what works and what does not.

The same build is the first public build: nothing here is a shortcut we would redo for open source.

## Non-goals

- macOS and Linux (the OS-specific packages are already separate; a port is its own spec).
- An installer (MSI/MSIX), code signing, auto-update. v1 is a zip; SmartScreen's "Run anyway" is documented in the README.
- CUDA builds of llama.cpp (Vulkan measured 94% of torch on text; a CUDA pack is added if feedback says images are too slow).
- Audio, the decision model and the LLM layer (their specs come later).
- A plugin system. Settings edit what `config.toml` already holds, plus the few new keys below.

## What changes, in one table

| Today | After |
|---|---|
| The engine is torch in `sidecar\.venv` in the repo, found by walking up from the exe (`config/defaults.go`) | The engine is installed from the app into `%LOCALAPPDATA%\AGI\engine\` and runs without torch |
| A missing engine is a permanent failure in the tray | No engine is a normal state: "File search is off" |
| Settings are a file the user edits; the launcher reads them once at start | A settings window edits the same file; every setting applies without a restart |
| First run indexes `Sample Data` from the repo | First run indexes nothing; folders are chosen by the user |
| Indexing speed is whatever the GPU gives | Pause, resume, and a speed (Fast, Balanced, Gentle) |
| `build.ps1` makes `bin\agi.exe` | `release.ps1` makes `dist\agi-<version>-windows-x64.zip` |

## Design

### 1. Where things live

| What | Where | Removed by |
|---|---|---|
| Settings | `%APPDATA%\AGI\config.toml` (unchanged), `state.json` beside it | user |
| Index | `%LOCALAPPDATA%\AGI\memory\` (unchanged) | "Delete the index" in settings |
| Logs, icon and thumbnail caches | `%LOCALAPPDATA%\AGI\logs\`, `icons\`, `thumbs\`, `apps.json` (unchanged) | user |
| Engine (new) | `%LOCALAPPDATA%\AGI\engine\`: `engine.json` (what is installed), `tools\uv.exe`, `python\` and `venv\` (made by uv), `app\` (the engine's source, copied from the exe), `llama\` (llama-server and its DLLs), `models\` (the two GGUF files) | "Remove file search" in settings: one folder delete, the index stays |

`config.toml` loses `[sidecar] dir` and the repo discovery behind it. A file that still has `[sidecar]` loads (the table is ignored)
and the next save drops it; every other unknown key is still an error.

### 2. The launcher without an engine

`sidecar.Supervisor` gets a state `off` (`StateOff`: no engine installed, nothing is started, nothing is retried). The tray says
"File search is off. Open Settings to turn it on." and the bar's file rows are simply absent. Everything the hotkey path needs
(apps, web, tags, the bar itself) never depended on the engine and stays that way. Enabling file search installs the engine and starts the
supervisor in the same session; removing it stops the supervisor and goes back to `off`.

### 3. Settings that apply live

New package `internal/settings`: one `Store` that owns the `Config`, loads and saves `config.toml` (the current `config` package
keeps parsing and validation), and tells subscribers when it changed. Consumers that captured `cfg.*` at start in `main.go` take
what they need from the store instead:

| Setting | Consumer | How it applies |
|---|---|---|
| `hotkey` | `panel.Panel` | `Rebind(accel)`: register the new one first, unregister the old one after it works; on failure keep the old one and show why in the form |
| `roots`, `exclude` | `indexing.Manager`, `watcher`, `files` (opener and thumbnails), `search` | the manager syncs the new set (new roots queue, removed roots are dropped from the index after a confirmation); the watcher restarts with the new roots; the others ask the store when they need the roots |
| `browser`, `search_url`, `[[quicklink]]` | `web.Web` | `Update(...)`; palette rows read it per query |
| `[search] enabled`, `pace` (new) | supervisor, indexing | start or stop the engine; set the speed |

The store validates a change with the existing `Config.Validate` before it writes, and writes atomically (temp file and rename), so
a crash never leaves half a file. Comments the user wrote in `config.toml` are lost on the first save from the window; the
window says so once. A file edited by hand while the app runs is not watched (open Settings again to reload).

### 4. The settings window

A second Wails window (framed, resizable, about 760 x 560, created when first opened, so the hotkey path pays nothing), opened from
the tray ("Settings"), from a palette command ("Settings") and from the bar's footer when file search is off. The same
frontend build serves it: `main.ts` mounts `Settings.svelte` when the URL hash is `#settings`, `App.svelte` otherwise; no router.
Styling follows `DESIGN.md` and `theme.css`.

Sections, each saving on change (no global Save button) and showing the error from `Validate` next to the field:

| Section | Holds |
|---|---|
| General | hotkey (a capture field: press the combination; a taken one is reported at once), browser (a name from the installed browsers Windows lists, or "default"), web-search address |
| Quicklinks | a table: trigger, title, address with `{q}`; add, edit, remove; the existing rules (one word, `{q}` once) shown as inline errors |
| File search | off: what it is, what will be downloaded and from where, the size, **Turn on**. Installing: a step list with bytes and a Cancel. On: folders, speed, pause, status (below), **Remove file search** |
| About | version, log folder, config folder, "Copy diagnostics" (version, Windows build, GPU names, engine state, last log lines; nothing else) |

Folders (in File search, available while it is off too: they are saved and start indexing when search is turned on): a list of the
chosen folders, each with its state (waiting, indexing 40 / 96, up to date with a file count), **Add folder** (the native folder
picker), remove (asks first: it drops that folder's index). Excluded subfolders stay a `config.toml` feature for now.

### 5. The engine on llama.cpp (Python side)

`GemmaEmbedder` and `query_graph.py` are replaced by `LlamaEmbedder` (promoted from `sidecar/spikes/llama_embedder.py`); torch,
transformers, sentence-transformers, scipy and scikit-learn leave `pyproject.toml` and the PyTorch index entry goes with them. The engine
owns the llama-server process: it starts it in the embedder's constructor on a free loopback port, with
`-ngl 99 -c 8192 -ub 1024 -b 1024 --image-max-tokens 280 --embedding`, and the process dies with the engine (the launcher's job object
already covers children, and the embedder also stops it on exit). Behaviour fixed by the spike and kept:

- one keep-alive connection; texts over 400 characters are tokenized by the server and cut to 511 tokens plus the closing token;
- images are resized to the model processor's size (about 2520 patches of 16 px, aspect kept, sides multiples of 48, bicubic) and sent as PNG;
- `model_id` and `dim` stay `google/embeddinggemma-2` and 768: the vectors agree with torch's to cosine 0.9999 (text) and 0.994 or better (images, pages),
  so an index built before this change stays valid and is not rebuilt. ADR 0004 is amended to say so.
- The device is chosen from `llama-server --list-devices`: a name with NVIDIA first, then AMD (Radeon), then anything else but Intel integrated
  graphics, else the CPU (`-ngl 0`), with the choice logged and shown in About. If the server is not healthy in 60 s on the chosen device it is
  started again on the CPU and `/health` says `device: cpu` so the settings window can say "running on the CPU (slow)". Choosing by memory size is wrong:
  the Intel iGPU on the dev laptop reports more than the NVIDIA card.
- If the server dies mid-run it is started again once; a second death fails the request (the supervisor restarts the engine as it does today).

`/health` keeps its contract; `device` becomes `vulkan`, `cpu` (was `cuda`, `cpu`).

### 6. The installer (`internal/enginesetup`, Go)

Turning on file search runs these steps, each resumable and verified, with progress events to the window:

| Step | Source | Size |
|---|---|---|
| 1. uv | `astral-sh/uv` release zip on GitHub, pinned version, SHA-256 | about 20 MB |
| 2. engine source | embedded in `agi.exe` (`//go:embed` of `sidecar/pyproject.toml`, `uv.lock`, `src/`), written to `app\` | 0 |
| 3. Python and libraries | `uv sync --frozen --no-dev` with `UV_PROJECT_ENVIRONMENT=venv` and `UV_PYTHON_INSTALL_DIR=python`: uv fetches its managed Python 3.12 (GitHub) and the wheels (PyPI) | about 150 MB |
| 4. llama.cpp | `ggml-org/llama.cpp` release `b11719`, `llama-b11719-bin-win-vulkan-x64.zip`, SHA-256 | 32 MB |
| 5. models | Hugging Face `ggml-org/embeddinggemma-2-GGUF` at a pinned commit: `embeddinggemma-2-Q8_0.gguf`, `mmproj-embeddinggemma-2-Q8_0.gguf`, SHA-256 | 865 MB |
| 6. check | start the engine once and run one text and one image embedding | |

Rules: every download is a resumable range request into `*.part`, checked against a SHA-256 compiled into the exe, then renamed; a
failed or cancelled install leaves `engine.json` unwritten and the next attempt continues from the `.part` files; the hosts
(github.com, pypi.org, files.pythonhosted.org, huggingface.co and the CDNs they redirect to) are listed in the window before the
user presses **Turn on**, because the project's privacy rule says nothing leaves the machine except what the user asked for. After the
install the app makes no network call. Updating the engine is the same function: it compares `engine.json` with what the exe
embeds and redoes only the steps whose pinned versions differ. A developer changes engine code, runs `build.ps1`, and presses
**Update engine**; there is no second way to run the engine.

### 7. Indexing control

Engine side (`sidecar/src/agi_memory/pace.py`): a `Pacer` the `Indexer` calls after every embedding call.

- **Pause/resume:** a flag; the indexer blocks in `checkpoint()` until resumed, so a pause takes effect within one batch (under a second).
  A paused job stays `running` with `paused: true`; stopping the engine while paused loses nothing (a file is recorded last; an unfinished one is redone).
- **Speed** is a duty cycle: after a call that took `t` seconds the indexer sleeps `t * (1/duty - 1)`, so the GPU is busy at most `duty` of the time.
  Presets: Fast 1.0, Balanced 0.5, Gentle 0.2, default Balanced (the owner can go about the day). The numbers are measured
  in the plan (block F) before they are named in the UI.
- API: `GET/PUT /pace {"duty": 0.5}`, `POST /pause`, `POST /resume`; `/health` and job responses report `paused` and `duty`.

Launcher side: `indexing.Manager` exposes `Pause`, `Resume`, `SetPace`, and per-root status; the pace is saved as `[search] pace = "balanced"` and applied
when the engine becomes ready. What the model keeps in VRAM (about 1.1 GB) stays while the engine runs; the speed setting changes how fast indexing
runs, not that. "Pause while on battery" is not in v1 (it is on the Next list of `STATUS.md`).

### 8. Release

`scripts/release.ps1` runs `build.ps1`, stamps the version (`-ldflags "-X main.version=..."`, from `git describe`), and writes
`dist\agi-<version>-windows-x64.zip` holding `agi.exe` and `README.txt` (what it is, the SmartScreen click, the hotkey, where the config and the log are,
what "Turn on file search" downloads, and what to send back: `agi.log` and the answers to a short list of questions). Publishing the
zip anywhere is the owner's step.

## Decisions made here (change them in review, not in code)

| Decision | Why |
|---|---|
| `model_id` stays; existing indexes survive the swap | measured cosine 0.9999 / 0.994+ against torch; saves a 1.5 h rebuild for the owner and every early user |
| Vulkan build only | 32 MB, no CUDA libraries, text at 94% of torch; CUDA adds 580 MB for images at 64% to 109% |
| uv to get Python and the libraries | the lock file already exists; the alternative (embedding a Python and pre-built wheels in the exe) means maintaining our own bundle |
| Settings apply without a restart | a restart needs a second-instance dance with the single-instance lock; live apply is also what a person expects |
| Folders can be chosen before search is turned on | the friend sets it up once, then presses one button |
| The engine source ships inside `agi.exe` | one file to hand over, and the engine always matches the launcher |

## Verification

- Go: table tests for the settings store (validate, atomic write, `[sidecar]` migration, subscribers), the supervisor's `off` state, the installer
  (a local HTTP server with fake artifacts: resume, bad checksum, cancel, host list), hotkey rebind on a fake registrar.
- Python: `LlamaEmbedder` against a tiny fake HTTP server (truncation, token lists, retry once), the Pacer (pause, duty cycle with a fake clock),
  and the `model`-marked tests against a real llama-server. The golden eval gates the swap: every metric within 0.02 of `baseline.json`.
- Measured, written to `docs/eval/`: hotkey to painted input unchanged (`cmd/panelbench -real`, limit 100 ms); GPU load at each speed preset;
  install time and bytes on the owner's machine; engine start to ready.
- By hand, on the friend's laptop, from the zip on a Windows account that has never seen the repo: first run, change the hotkey, add a quicklink,
  turn on file search, add a folder, pause and resume, turn it off, delete the app folder and check nothing but the data folders remain.

## Open risks

| Risk | Mitigation |
|---|---|
| Wails v3 beta: a second window, runtime hotkey rebind | both are public APIs in the pinned version (`GlobalShortcut.Unregister`, `Window.NewWithOptions`); a spike in block B before the window is built |
| Antivirus or SmartScreen flags `agi.exe` or the downloaded `llama-server.exe` | pinned release checksums; document it in the README; revisit signing when public |
| uv or the PyTorch-free lock fails on a clean machine | the installer step 3 prints uv's last lines in the window and the log; the friend's laptop is the test |
| A GPU or driver without Vulkan | the CPU fallback in section 5; the window says so |
| Hugging Face rate limits or an interrupted 865 MB download | range requests resume; the pinned commit makes the URL stable |
