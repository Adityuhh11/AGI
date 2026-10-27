# Packaging Implementation Plan (blocks)

**Spec:** `docs/superpowers/specs/2026-10-18-packaging-design.md`. **Branch:** `feat/packaging`, cut from `dev`.
**Goal:** a zip the owner's friend can run on a clean Windows account: the launcher works at once, file search is a button that
installs a llama.cpp engine, folders and speed are chosen in a settings window, and every setting applies live.

Built in blocks, one commit (or a few) per block; each leaves the repo green and demonstrable. Conventional commits, no attribution
trailers, no push without the owner's word. Spikes (marked S) run first, record what they measured in `docs/eval/`, and may change
the block after them. Two milestones: **M1** after block C (the launcher alone goes to the friend) and **M2** after block G (file search too).

## Global constraints

- Go from the repo root: `go vet ./...`, `go test ./...`, `gofmt` clean. Stdlib first; a new dependency is named in its block and justified.
  Frontend: `npm run check` and `npm test` clean. Python from `sidecar`: `uv run ruff check`, `uv run pyright`, `uv run pytest` clean.
- The hotkey path must not move: `cmd/panelbench -real` before and after each block that touches the page, the panel or `main.go`
  (baseline 41 ms hotkey to focused input; limit 100 ms). Opening the settings window must not change it.
- Nothing leaves the machine except the installer's downloads, which the user starts and whose hosts are listed before they start.
- No test or tool may touch `%LOCALAPPDATA%\Omni\memory` of the owner: tests use `OMNI_MEMORY_DIR` and temp folders.
- Docs in the same commit as the behaviour; `docs/STATUS.md` gets a row per block that ships something; `docs/ARCHITECTURE.md` and
  `docs/api.md` change with the engine.
- Read `DESIGN.md` and `PRODUCT.md` before block B; the settings window is built to them.

## Block S: spikes

**S1. Wails runtime.** A throwaway program (or a temporary tray item) that, at run time: opens a second framed window on `#settings`
from the tray and closes it again; calls `GlobalShortcut.Unregister` and `Register` for a new accelerator, and for one another
program owns; opens the native folder picker (`Dialog.OpenFile().CanChooseDirectories(true)`). Record what works and what does
not, and whether opening the second window changes the hotkey-to-input time. Output `docs/eval/2026-10-18-wails-runtime.md`.

**S2. Clean install.** In an empty temp folder, by hand: download the pinned uv zip; regenerate `uv.lock` without torch and run
`uv sync --frozen --no-dev` with `UV_PROJECT_ENVIRONMENT` and `UV_PYTHON_INSTALL_DIR` set; record the time, the bytes downloaded and the
size on disk; download the llama.cpp Vulkan zip and the two GGUF files at the Hugging Face commit chosen for pinning with `Range` requests
and compute their SHA-256. Output `docs/eval/2026-10-18-clean-install.md`, which is also where the pins in block E come from.

## Block A: settings that apply live, and an engine that may be off

- **A1. `internal/config`.** `[sidecar]` becomes ignored on load and absent on save; delete `defaults.go`'s repo discovery (default
  roots are empty); add `[search] enabled = false` and `pace = "balanced"` (validated: fast, balanced, gentle). Table tests: the owner's
  current file loads, a file with a typo still fails, a round trip loses nothing.
- **A2. `internal/settings`.** `Store` with `Get() Config`, `Update(func(*Config)) error` (validates, writes through a temp file and rename,
  then notifies), `Subscribe(func(old, new Config))`. Tests: invalid change leaves file and memory untouched; a crash between the temp
  write and the rename leaves the old file; subscribers see each change once, in order.
- **A3. Supervisor `off`.** `StateOff` and `Options.Installed func() bool`; when false, `Run` sets `off` and waits for `Start()`/`Stop()`
  calls instead of starting a process. `statusLine` says "File search is off. Open Settings to turn it on." Tests in `supervisor_test.go`.
- **A4. Live consumers.** `web.Web.Update`, `files` and `search` take a `roots func() []string`, `indexing.Manager.SetRoots/SetExclude`
  (new roots queue, removed roots are forgotten from the engine on the next cycle: the code that does that exists in `syncAll`),
  `watcher` restarts when the roots change, `panel.Panel.Rebind(accel)` (register new, then unregister old, keep the old on failure).
  `main.go` wires the store to all of them. Tests for each `Update`/`Set*`; the rebind against a fake registrar.
- Commit(s): `refactor(config): ...`, `feat(settings): ...`, `feat(sidecar): off state`, `feat(launcher): apply settings live`.

## Block B: the settings window

- **B1. Window and plumbing.** `internal/settingsui` (Go service bound to the frontend): `Get`, `Save(section, values) []FieldError`, `PickFolder`,
  `Browsers` (installed browsers from App Paths, reusing `web`), `Diagnostics`. The second window is created on first open; entries in the tray and as a palette
  command. `main.ts` mounts `Settings.svelte` on `#settings`. Binding regeneration is already in `build.ps1`.
- **B2. General and Quicklinks.** The hotkey capture field (a pure function turns a keydown into an accelerator string, unit-tested), browser picker,
  search address, quicklink table with inline validation messages from the Go side. Vitest for the pure parts.
- **B3. File search (off) and About.** Folders list with the native picker, per-folder state from `indexing`, **Turn on** disabled with a note until block E
  (the button exists, the install does not), About with version, folders, log, Copy diagnostics.
- Check by hand (the owner): change the hotkey, add a quicklink and use it from the bar, add and remove a folder, with no restart.

## Block C: release M1

- `scripts/release.ps1` (build, version stamp, zip with `README.txt`); `main.version`; the README text per the spec. A clean-account run: the owner
  unzips to a new folder, deletes `%APPDATA%\Omni` and `%LOCALAPPDATA%\Omni` first (after copying them), and confirms: hotkey works, apps, web,
  tags, the settings window, the tray says file search is off, `omni.log` is written. Measure `panelbench -real`.
- Output: `docs/eval/<date>-m1-release.md`; `STATUS.md` row. **The friend gets this zip.** Questions to send with it go in the README.

## Block D: the engine on llama.cpp (Python)

- **D1. `LlamaEmbedder`.** Promote the spike's embedder into `embedding/llama.py` and `embedding/llama_server.py` (process start, device choice from
  `--list-devices`, health wait, CPU fallback, one restart). Python tests against a fake HTTP server (truncation, token lists, keep-alive reuse,
  device parsing of real `--list-devices` output saved as a fixture, restart once, fail twice).
- **D2. Wire and drop torch.** `__main__.py` builds `LlamaEmbedder` from `OMNI_LLAMA_SERVER` and `OMNI_MODEL_DIR`; delete `gemma.py`, `query_graph.py`,
  `test_gemma_embedder.py`; remove torch and the PyTorch index from `pyproject.toml`, regenerate `uv.lock`; `eval/__main__.py` uses the new embedder.
  `device` values become `vulkan` and `cpu` (`docs/api.md`, the supervisor's reading of `/health`).
- **D3. Gates.** With the spike's llama dir and models, on the owner's machine: the golden eval (every metric within 0.02), `pytest`, a real index job over
  `Sample Data`, then the engine started by the launcher pointing at the same paths through the two environment variables. The owner's real index must
  open without a rebuild (copy it first). Delete `sidecar/spikes/`.
- Docs: `ARCHITECTURE.md` (module table, invariant 4 and 5, on-disk notes), `api.md`, amend ADR 0004 (model id kept), `STATUS.md`.

## Block E: turn file search on from the app

- **E1. `internal/enginesetup`.** `Plan` (the steps and their pins from S2), `Download` (resumable range requests into `.part`, SHA-256, rename), `Install(ctx, progress)`,
  `Installed()`, `Remove()`, `Update()`; the engine source embedded through `go:embed` (a root-level `enginesrc.go`, beside `assets.go`, because embed cannot reach
  outside its folder). Tests with `httptest`: resume after a cut connection, a wrong checksum is refused and deleted, cancel leaves resumable parts, `engine.json`
  is written last, a second run does nothing.
- **E2. Wiring and UI.** The File search section: the host list and size before **Turn on**, a step list with bytes and Cancel, then the on state; **Remove file search**
  and **Update engine**. On success `[search] enabled = true`, the supervisor `Start()`s with `OMNI_LLAMA_SERVER` and `OMNI_MODEL_DIR` pointing into the engine folder.
- **E3. Acceptance.** On the owner's machine from a clean `engine\` folder: time and bytes (into `docs/eval/`), then remove and turn on again; kill the network in
  the middle of the model download and resume.

## Block F: indexing control

- **F1. `pace.py` and the API.** `Pacer` (pause flag, duty cycle with an injectable clock and sleep), called by `Indexer` after every embedding call; `/pace`, `/pause`,
  `/resume`; `paused` and `duty` in `/health` and job bodies. Tests with a fake clock; a pause during a sync of `Sample Data` stops embedding calls within one batch and
  resuming finishes the same result.
- **F2. Measure the presets.** Index the owner's `Downloads` copy at duty 1.0, 0.5 and 0.2; record GPU utilization (`nvidia-smi`), seconds, and the hotkey-to-input time while
  indexing. If 0.5 and 0.2 do not lower GPU use in proportion, change the model of the knob before naming it in the UI. Output `docs/eval/<date>-pace.md`.
- **F3. Launcher and UI.** `indexing.Manager.Pause/Resume/SetPace` and per-root status; the speed picker, pause button and the folder states in the File search section;
  tray item "Pause indexing".

## Block G: release M2

- README gains the file-search steps; the zip is rebuilt; a clean-account run on the friend's laptop with the owner (turn on, add one folder, watch the index grow, search
  while it builds, pause, change speed, remove). Collect his answers and the log in `docs/eval/<date>-m2-friend-session.md`; open the follow-ups as the next `STATUS.md` list.
- Merge path: `feat/packaging` into `dev`, then the owner's pull request to `main` (never a direct push).
