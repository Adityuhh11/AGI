# Launcher v1: hotkey, search bar, results from across the laptop

Date: 2026-10-15. Status: approved by the owner, scope narrowed on 2026-10-15 (see Scope). Builds on the memory engine (`docs/api.md`,
`2026-10-12-memory-engine-design.md`, `2026-10-15-search-quality-design.md`). Evidence:
`docs/eval/2026-10-15-launcher-experiments.md`.

## Goal and scope

Press a hotkey, a plain black search bar appears with the cursor in it, type, and see where your files are: code,
PDFs, images and text anywhere on the laptop, ranked by the memory engine, each result showing the part of the file that
matched. Enter opens it. That is the whole of v1.

In scope: global hotkey (configurable), the panel, search-as-you-type, file-level results with the matching chunks,
opening and revealing files, indexing the laptop in the background and keeping it fresh, a tray icon to see status and
quit, starting the Python sidecar and keeping it alive.

**First milestone: it searches the `Sample Data/` folder only.** The owner will build the big index of the whole laptop
later, in one go, plugged in, by hand. So v1 has no automatic scheduling, no power or full-screen policy and no default
list of personal folders: `roots` defaults to the sample folder, and pointing it at real folders later is a config
change. Only code is wanted from repositories: dependency and build folders (`node_modules`, `.git`, venvs, `dist`...)
are already skipped by the engine's scanner.

Out of scope for v1 (each is its own later spec): any LLM feature, UI design beyond a plain black bar, a settings
screen (config is a file), app and command launching, clipboard history, Office files, file watching (built later: `internal/watcher`), packaging and
installers, auto-update, macOS and Linux.

## Constraints (from `AGENTS.md`, measured where it matters)

- Hotkey to a focused, painted input in under 100 ms, never waiting on a model. Measured: a normal Wails show/hide takes
  a median 72 ms (max 96 ms), a DWM cloak/uncloak takes 41 ms (max 58 ms). We use the cloak (see Panel).
- Nothing leaves the machine. The sidecar token exists only in the Go process; the web view never sees it.
- Windows 11 only, 6 GB VRAM shared with the embedding model: no second model in v1.
- Go: stdlib first, `gofmt` and `go vet` clean, one package one job, table tests. Frontend: Svelte and TypeScript.

## Architecture

One background app, `agi.exe` (Go, Wails v3 beta pinned in `go.mod`), with a tray icon and no taskbar entry. It owns the
panel window, supervises the Python sidecar and is the only client of its API.

```
hotkey (RegisterHotKey) ──> internal/panel ──> Svelte UI (WebView2)
                                                  │ Wails bindings (Go methods)
                    ┌─────────────────────────────┴───────────────┐
              internal/search          internal/files        internal/indexing
                    │                 (open, reveal,         (roots → sources,
                    │                  thumbnails)            sync schedule, status)
                    └───────────────┬─────────────────────────────┘
                          internal/sidecar (client + supervisor)
                                    │ HTTP 127.0.0.1, bearer token
                              Python sidecar (memory engine)
```

Repository layout: `cmd/agi/` (entry point), `internal/{config,sidecar,panel,search,files,indexing,tray}/`, `frontend/`
(Svelte), `sidecar/` unchanged. `docs/` as before.

### internal/config

`%APPDATA%\AGI\config.toml`, created with defaults on first run, read at start. Parsed with `pelletier/go-toml/v2` (the
one new dependency here; the stdlib has no TOML).

```toml
hotkey = "Alt+Shift+Space"
roots = ["C:\\dev\\VS_CODE\\AGI\\Sample Data"]   # v1 default; real folders are added later by hand
[sidecar]
dir = "C:\\dev\\VS_CODE\\AGI\\sidecar"   # dev: where `uv run python -m agi_memory` runs; packaging is later
```

Each root becomes a source with a stable id derived from its path (`root-` plus the first 10 hex of its SHA-256). The
engine's own scanner already skips secrets (deny list), `.gitignore`d paths, `node_modules`, links and oversize files.
Personal folders (Documents, Downloads, `C:\dev`...) are added to `roots` by the owner when the big index is built; the
whole `C:\` is deliberately never a default (AppData, Program Files and caches would drown the index).

### internal/sidecar

- **Supervisor**: generates a 32-byte random token per launch, starts `uv run python -m agi_memory` with
  `AGI_SIDECAR_TOKEN`, `AGI_SIDECAR_PORT` (a free port, passed explicitly), `AGI_MEMORY_DIR`; puts the process in a
  Windows Job Object with `KILL_ON_JOB_CLOSE` so it can never outlive the launcher; polls `GET /health` until
  `ready` (cold start measured at 32 s, so the launcher starts the sidecar at launch, not at first search); on exit or
  `error` restarts with backoff (1, 2, 4, ... up to 30 s) and reports state to the UI. Exit status 2 (bad config) and an
  index refused for model, dimension or schema change are not retried: the UI shows the message, and a refused index is
  rebuilt only after the owner confirms (it moves the old index directory aside).
- **Client**: typed methods for every route in `docs/api.md` (`Health`, `Sources`, `AddSource`, `Job`, `DeleteSource`,
  `Search`), a request timeout, and errors that carry the HTTP status and `detail`. Tested against an `httptest` server.

### internal/search

Wails-bound `Search(query string, token int) ([]FileResult, error)`. The UI debounces (120 ms), the service cancels the
previous in-flight request when a newer one arrives, and drops answers whose `token` is stale, so a slow answer never
overwrites a newer one. Maps the sidecar's file groups to display rows: absolute path (root of the source plus uri), file
name, folder, kind, best-chunk snippet, location text (`lines 12-52`, `page 3`, `image`), and for images a thumbnail URL.
While the sidecar is not ready it returns a `warming up` state instead of an error. When `confident` is false the UI
shows "No match" above the (still returned) results in a dim colour rather than presenting noise as an answer.

### internal/files

`Open(path)` (ShellExecute, the default app), `Reveal(path)` (`explorer /select,`), and a thumbnail handler on the asset
server: `/thumb?path=...` decodes PNG, JPEG, GIF and WebP with the standard library and `golang.org/x/image`, resizes to
160 px on the long side, and caches the JPEG under `%LOCALAPPDATA%\AGI\thumbs` keyed by path and mtime. It serves only
paths inside a configured root and refuses everything else. PDFs show a text snippet and the page number, no page image
in v1.

### internal/indexing

On start, and on tray "Rescan now", submits each configured root with `POST /sources` one at a time (the engine allows
one job; a 409 means wait for the running job). A repeat submit is an incremental sync, so unchanged files cost almost
nothing and deleted files disappear. Reports to the tray and UI: `Indexing Sample Data: 40 / 96 files` or `Up to date:
431 chunks`. A failed job shows its error; per-file errors are counted, not shown. Removing a root from the config issues
`DELETE /sources/{id}`. Later, when real folders are configured: periodic rescans, running on mains power only and pausing
under full-screen apps (decided as direction by the owner, not built in v1).

### internal/panel

The window: frameless, always on top, no taskbar button, black, 720 px wide, centred horizontally and placed at about a
fifth of the height of the monitor that holds the cursor, per-monitor DPI aware. It is created once at start, shown once
and then **cloaked**; the hotkey uncloaks it and the Escape key, a click outside or the hotkey again cloaks it. Never
`Hide()`: that costs about 70 ms because WebView2 stops rendering.

Show: remember `GetForegroundWindow()`, move to the right monitor, uncloak, Wails `Focus()`, tell the UI to focus and
select the input (the last query stays, selected, so typing replaces it). Hide: cloak, then `SetForegroundWindow(previous)`
because a cloaked window keeps keyboard focus otherwise (measured). The window height follows the content (input only
at first; grows with results up to 8 rows, then scrolls); resizing is done by Go on a UI request and is checked in the
latency harness.

Hotkey: `app.GlobalShortcut.Register` (Win32 `RegisterHotKey`). If the combination is taken the launcher says so in the
tray tooltip and via a notification and tries nothing else silently; the owner changes `hotkey` in the config. Several
common combinations are already taken on this machine (`Alt+Space`, `Win+Space`, `Ctrl+Alt+Space`).

### Frontend

> The look described here (a plain black bar) was replaced on 2026-10-15; see `2026-10-15-launcher-visual-design.md` and `/DESIGN.md`. The behaviour below (rows, keys) still holds.

One Svelte view. A black bar with an input (placeholder "Search"), no other chrome. Below it, up to 8 rows. Row anatomy,
all plain text on black:

- **file** (code, text, markdown): kind icon (a letter glyph for now), file name, folder path in dim grey, and under it
  the best matching chunk (about 2 lines of the snippet) with its location (`lines 12-52`).
- **pdf**: name, folder, snippet and `page 3`.
- **image**: a 64 px thumbnail on the left, name and folder.
- **more matches**: when a file has further matching chunks, `+2 more` at the row's end; Right arrow expands them in
  place (each with its location), Left collapses. Moving down skips the children unless expanded.

Keys: Up/Down move, Enter opens the file, Ctrl+Enter reveals it in Explorer, Right/Left expand/collapse, Ctrl+C copies the
path, Escape hides. The pointer works too (click opens). How this should look and feel comes later, in its own design
pass; v1 only needs to be readable and fast.

## Engine changes this requires (a short phase 7a, before the launcher code)

The launcher needs four things the API does not have yet:

1. **Files, not chunks.** `SearchHit` gains `source_id`. `POST /search` accepts `group: "file"`: it ranks chunks as today
   (pool of 60), groups them by `(source_id, uri)` with the file's score equal to its best chunk's, and returns up to `k`
   files with up to `per_file` (default 3) chunks each (`locator`, `snippet`, scores). The eval gains a file-level
   variant of its metrics (the golden set is already file-level: it matches on `uri`), and the response keeps `confident`.
2. **`GET /sources`**: id, root, last scan time, file and chunk counts, so the launcher can show status and detect a root
   that was never indexed.
3. **Query priority over indexing.** Measured: a search during indexing took p50 226 ms and up to 1.5 s. Goal: search p95
   under 300 ms while a job runs. The approach is chosen by measurement in the plan (a priority lock so queued queries run
   before the next indexing batch, and a smaller image batch while a query is waiting), and the first full-laptop index must
   not make the panel feel slow.
4. **Document the contract** in `docs/api.md`; add the deviation to `docs/STATUS.md`.

## Testing and acceptance

- Go table tests: config parsing and defaults, source-id derivation, the search service's cancel and stale-token logic, the
  sidecar client against `httptest`, the supervisor's restart policy against a fake process, the indexing scheduler against
  a fake client, the thumbnail handler's path check. Trivial glue has no test.
- A latency harness (`cmd/panelbench`, Windows only) repeats the spike's measurement against the real panel: hotkey to a
  focused, painted input, with another app holding focus, 50 runs, and fails over 100 ms at the 95th percentile. It is run
  by hand before a merge that touches `internal/panel`.
- Manual acceptance on the owner's laptop: fresh start with an empty index finds Documents content after the first root is
  indexed (searching works while the rest indexes), the sample queries from `sidecar/eval/golden.yaml` return the right files
  at the top, a deleted file disappears at the next rescan, a killed sidecar is restarted, quitting from the tray leaves no
  Python process, a taken hotkey is reported.

## Risks

- **Wails v3 is beta** (nightly tags, "API stable" in its notes). Mitigation: pin the exact version, keep Wails behind
  `internal/panel`, `internal/tray` and the bindings so the window layer could be swapped for a plain WebView2 host.
- **Cloak technique** relies on `DwmSetWindowAttribute`; a Windows update could change behaviour. The latency harness is the
  guard; fallback is Wails `Show()`/`Hide()` at 72 ms median (inside the limit, with little margin).
- **Foreground rules**: taking focus from another process is restricted by Windows; it worked in all 15 spike runs because
  the hotkey press is input to our process. The acceptance run covers full-screen apps and elevated windows (an elevated
  app cannot be handed focus back by a non-elevated one: documented limit).
- **Later, the big index**: about an hour of GPU use for the whole laptop (measured inventory in the experiments note); the
  owner runs it once, plugged in. Automatic background indexing and its power policy are deferred.
- **768-d index size**: 100k chunks are 307 MB; the whole laptop is estimated at 20,000 to 60,000 chunks.

## Decisions taken

Default hotkey `Alt+Shift+Space` (configurable). Sample Data is the only default root for now; personal folders and the
background-indexing policy come with the big index. No LLM and no design pass in v1.
