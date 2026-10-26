# AGI

A fast, private launcher for Windows. Press a hotkey, type, and open an app, search the web, paste a snippet, or find a
file by what it *says*, not just what it is called. Everything runs on your computer.

> **Status: early (0.5.x), Windows 11 only.** It is used daily by its author and a few friends. Expect rough edges;
> please open an issue when you meet one. macOS and Linux are planned, not started.

- **Instant.** The bar is painted about 33 ms after the hotkey (median, measured), and never waits on a model.
- **Light.** Idle, the launcher uses no CPU and about 270 MB of memory including its web view. File search is optional;
  when on it adds a small model (about 1 GB of graphics memory) and a Python process (about 450 MB with a 46,000-chunk index).
- **Private.** No telemetry, no account. File search runs a local model; your files and your index never leave the machine.
  The few things that touch the network are listed [below](#privacy-and-the-network).

## What it does

| | |
|---|---|
| **Apps** | Type a few letters (`vsc`, `chr`), the first letters of each word, or part of a name. Frequently used apps rise. An app that is already open is brought to the front instead of starting twice. |
| **Files, by meaning** *(optional)* | Describe what you want: "the diagram with the three boxes", "how I hash passwords", "t distribution table". Finds code, PDF pages, notes, photos (by what they show) and videos (by name, or by eight frames from inside them). Results show where in the file the match is. |
| **Web and quicklinks** | `?` or `web:` searches in the browser you choose. `gh rust lifetimes` opens that search on GitHub; add your own quicklinks. |
| **Clipboard history** *(off until you switch it on)* | `clip:` lists what you copied, newest first. Enter pastes it into the program you came from. Skips copies marked private, password managers and programs you list. |
| **Snippets** | Text you paste often, with `{date}`, `{time}` and `{clipboard}` filled in. `snip:` or a keyword. |
| **Notes** | `note:` lists your notes; `note: buy milk` makes one. They are markdown files in `%APPDATA%\AGI\notes`, opened in a small window that stays above other windows (and can be resized, searched and deleted). |
| **Timer and pomodoro** | `timer 25`, `timer 90s`, `timer 1h30 tea` or `pomodoro`: a small timer sits at the edge of the screen and keeps going when you close it. Lengths are in Settings > Timer. |
| **Calculator and conversions** | `12*7+3`, `15% of 80`, `5 km in miles`, `72 f in c`: the answer is the first row; Enter copies it. |
| **System** | `lock`, `sleep`, `task manager`, `recycle bin`, `windows settings`. Shut down and restart are left out on purpose. |
| **Tags and pins** | `Ctrl+T` tags an app or file, `#work` lists everything tagged. `Ctrl+P` pins up to eight rows that show the moment the bar opens. |

Keys: `Tab` switches between All / Apps / Files / Web, `Ctrl+K` shows what you can do with the selected row,
`Ctrl+Enter` reveals a file in Explorer, `Alt+Enter` searches the web from any row, `Ctrl+,` opens Settings (so does typing `settings`, or `settings file` for one page), `Esc` closes.
In file search, `code:` `img:` `pdf:` `doc:` `vid:` before a query limit it to one kind of file.

## Install

**Requirements:** Windows 11 (64-bit). WebView2 is part of Windows 11. For fast file search, a graphics card with a
current driver (NVIDIA, AMD or Intel Arc, through Vulkan); without one it still works, only slower
([numbers](docs/PERFORMANCE.md#a-computer-without-a-graphics-card)). About 1.1 GB of disk and a one-time download of about 1 GB
if you turn file search on; the launcher alone is a 13 MB file.

1. Download `agi-<version>-windows-x64-setup.exe` from the [latest release](https://github.com/Adityuhh11/AGI/releases/latest) and run it.
   It installs for you alone (no administrator rights) into `%LOCALAPPDATA%\Programs\AGI`, or the folder you choose, and adds a Start menu
   entry. Prefer no installer? `agi-<version>-windows-x64.zip` holds the same `agi.exe`: unzip it anywhere you can write to.
2. Windows may say **"Windows protected your PC"**, because the program is not code-signed yet (see [signing](docs/DISTRIBUTION.md)):
   click **More info**, then **Run anyway**. You can check the download against the SHA-256 shown next to it on the release page, or
   with `gh attestation verify <file> --repo Adityuhh11/AGI`, which proves it was built from this repository by its release workflow.
3. There is no window of its own: AGI lives in the notification area (the arrow by the clock). The first start opens a short setup guide:
   what AGI is, your hotkey and the keys, and the optional file search with its folders and options (the tray menu shows it again).
4. Press **Alt+Shift+Space**. Change the hotkey in Settings > General; it tells you if another program already owns it.
   "Start when I sign in" is there too.

To update, open Settings > About and press **Update** (it checks the release's SHA-256 first), or run the newer installer over it.
To remove it: Settings > Apps > Installed apps > AGI > Uninstall. It asks whether to delete what AGI saved too (the engine and models,
the index, clipboard history and settings, up to about 1.2 GB); your own files are never touched. For the zip, quit from the tray
icon and delete the folder and the folders under [Where things are](#where-things-are).

### Turning on file search

File search is the part that needs real resources, so it is a choice you make in **Settings > File search**.

1. **Download and turn on.** The page says what it will download and from where before you press the button: a search
   engine (llama.cpp), its Python libraries (pinned, through `uv`) and the embedding model, about 1 GB from `github.com`,
   `pypi.org`, `files.pythonhosted.org` and `huggingface.co`. Every file is checked against a pinned SHA-256. It took about
   two minutes on a fast connection. Cancelling keeps what was downloaded.
2. **Add folders and choose what to index** (PDF pages, photos, videos) and **how hard it works** (Fast, Balanced, Gentle).
   Nothing is read until you press **Start indexing**, so you choose first. Each folder shows how many files and how much it holds, and each
   option says how long indexing will take on this computer (within about a quarter; the time left takes over once it runs). The defaults are sensible; on a computer without a
   graphics card choose *Words only* for PDFs and *names* for photos.
3. **Later changes are saved, not instant.** Changing a choice on an index that exists shows what it will do and waits for
   **Save and re-index**. Afterwards only changes are processed: a new file is searchable a few seconds after it appears, and a
   moved file is not computed again.

"Turn off" stops the engine and keeps everything. "Delete downloaded files" removes the engine and models and keeps your index.

## How it works

```
 hotkey ──▶ [ Go launcher (Wails v3) ] ──▶ Svelte page in WebView2 (the bar, Settings)
                  │  apps · web · tags · pins · clipboard · snippets: answered in-process, microseconds
                  │
                  │  authenticated HTTP on 127.0.0.1                         (only when file search is on)
                  ▼
        [ Python memory engine ] ──▶ llama.cpp server ──▶ EmbeddingGemma 2 (Q8_0), on the GPU or the CPU
                  │
                  └── index: SQLite (metadata, keyword + path search) + one memory-mapped vector file, searched on the CPU
```

- **Instant things are answered by Go itself** from memory (the Start Menu list, your pins and tags), so the bar never waits.
- **File search is hybrid.** A file is split into chunks (code by lines, Markdown by section, PDFs by page, photos and
  video frames as pictures). Each chunk becomes a 768-number vector from a small embedding model, and a query is turned into one the
  same way. Results blend three signals into one score: vector similarity, keyword (BM25) matches on the text and identifiers,
  and the query appearing in the file's path. The engine also says when nothing is a *confident* match, using a noise ceiling it measures from
  its own index, so a query with no real answer returns "closest files" rather than a confident wrong one.
- **Only the model uses the graphics card.** Storage and search run on the processor; a search takes about 16 ms on a 45,000-chunk index.
- **It stays current by itself.** A watcher follows your folders and the engine re-reads only what changed.

More: [`docs/ARCHITECTURE.md`](docs/ARCHITECTURE.md) (modules, data flow, on-disk format, invariants),
[`docs/api.md`](docs/api.md) (the engine's HTTP contract), [`docs/decisions/`](docs/decisions/) (why each big choice was made, with the numbers).

## Performance

Measured on a laptop with an RTX 4050 (6 GB) and a real index of 46,000 chunks; method, no-GPU numbers and caveats are in
[`docs/PERFORMANCE.md`](docs/PERFORMANCE.md).

| | |
|---|---|
| Hotkey to a painted, focused input | **31 to 36 ms** median, under 50 ms p95 |
| App search, per keystroke | **0.2 ms** |
| File search query | **16 ms** median, 21 ms p95 (index of 45,000 chunks) |
| New file searchable | about **3.5 s** after it appears |
| Idle, launcher | **0 % CPU**, about **270 MB** private memory with its web view; file search adds about 450 MB and 1.06 GB of video memory |
| Indexing speed | text about 53 chunks/s; pictures and PDF pages about 6/s on the GPU, about 0.2/s on the CPU |
| **No graphics card** | the sample folder (96 files) indexes in **8 minutes** instead of 16 seconds, or 5 minutes with the cheapest choices; a search still takes about **40 ms** |
| Quality (92 hand-written queries) | right file first for 90 %, in the top three for 100 %, no false alarms |

## Privacy and the network

No telemetry. No account. Your files are read only from folders you add, and secrets (`.env*`, keys and similar) are never
opened. The program touches the network in exactly these cases, all started or switched on by you:

| When | Where | What is sent |
|---|---|---|
| You press **Download and turn on** (or **Update**) in File search | `github.com`, `pypi.org`, `files.pythonhosted.org`, `huggingface.co` | ordinary downloads of pinned files |
| You open a web search or link | your browser | what you typed after the trigger |
| You press **Check for updates**, or turned on the daily check (off by default) | `api.github.com` | a request for the latest release number, nothing about you |
| You press **Update to X** | `github.com` | a download of that release, checked against its published SHA-256 |

The engine listens on `127.0.0.1` only, behind a random per-run token, and rejects foreign `Host` headers.
Clipboard history is off by default and stored only on your machine. See [`docs/decisions/0005-update-check.md`](docs/decisions/0005-update-check.md).

## Settings and where things are

Everything is in Settings (`Ctrl+,`, or the tray icon). The file behind it is `%APPDATA%\AGI\config.toml`; Settings edits it
and takes effect without a restart, and a mistyped key is reported rather than ignored.

```toml
hotkey  = 'Alt+Shift+Space'
roots   = ['C:\Users\you\Documents', 'C:\dev']      # folders file search indexes
exclude = ['C:\dev\big-generated-folder']           # inside a root, left out
browser = 'firefox'                                 # empty = the Windows default

[[quicklink]]
trigger = 'gh'
title   = 'GitHub'
url     = 'https://github.com/search?q={q}'

[search]
enabled  = true
pace     = 'balanced'    # fast | balanced | gentle
pdf_pages = 'auto'       # auto | all | text
photos   = 'content'     # content | name
videos   = 'name'        # name | content
```

| What | Where |
|---|---|
| Settings | `%APPDATA%\AGI\config.toml` |
| Log (Settings > About opens it) | `%LOCALAPPDATA%\AGI\logs\agi.log` |
| Search engine and models | `%LOCALAPPDATA%\AGI\engine` (only after you turn file search on); the setup guide can put it and the index on another drive |
| The index | `%LOCALAPPDATA%\AGI\memory` |
| Clipboard history | `%LOCALAPPDATA%\AGI\clipboard.json` (only if switched on) |
| Notes | `%APPDATA%\AGI\notes\*.md`, one file per note |
| What you launched, tags, pins | `%APPDATA%\AGI\state.json` |

## Build from source

Windows 11, PowerShell, and:

- [Go](https://go.dev/dl/) 1.27 or newer
- [Node.js](https://nodejs.org/) 24 (any current LTS should do) and npm
- the [Wails v3 CLI](https://v3alpha.wails.io/) (`go install github.com/wailsapp/wails/v3/cmd/wails3@latest`; the project is on `v3.0.0-beta.29`)
- [uv](https://docs.astral.sh/uv/) for the Python engine's tests (not needed to build or run the launcher: the engine's source is embedded in `agi.exe`)

```powershell
git clone https://github.com/Adityuhh11/AGI.git
cd AGI
.\scripts\build.ps1            # bindings, frontend, then bin\agi.exe  (-Console keeps the log window)
.\bin\agi.exe
```

Tests and checks:

```powershell
go vet ./... ; go test ./...                                  # launcher
cd frontend ; npm test ; npm run check                        # page
cd ..\sidecar ; uv run pytest ; uv run ruff check             # engine (add -m model to use the real model)
uv run python -m agi_memory.eval                              # retrieval quality against docs/eval/baseline.json
```

`.\scripts\release.ps1 -Version X.Y.Z` makes the release zip; the full procedure is in [`docs/RELEASING.md`](docs/RELEASING.md).

## Repository map

| Path | What |
|---|---|
| `cmd/agi` | the launcher's `main`: wires everything together |
| `cmd/panelbench` | measures hotkey-to-painted latency on the real app |
| `internal/panel`, `settingsui`, `tray` | the bar window, the Settings window, the tray icon |
| `internal/apps`, `rank`, `palette`, `state`, `web`, `snippets`, `clipboard` | what the bar can answer instantly |
| `internal/search`, `indexing`, `watcher`, `sidecar`, `enginesetup` | file search from the launcher's side: querying, syncing folders, running and installing the engine |
| `internal/config`, `settings`, `update`, `autostart`, `files` | settings file and live store, update check and install, start at sign-in, opening results |
| `frontend/` | the Svelte 5 + TypeScript page (the bar and Settings) |
| `sidecar/` | the Python memory engine (`agi_memory`): chunking, embedding, store, search, eval |
| `docs/` | architecture, status, API, decision records, specs, plans, measurements |

## Contributing

Not open to pull requests yet while the project is being made ready for others; issues and bug reports are welcome. How it will work is
in [`CONTRIBUTING.md`](CONTRIBUTING.md), the working rules (small focused commits, measure before deciding, no speculative code) in
[`AGENTS.md`](AGENTS.md), and vulnerabilities go through [`SECURITY.md`](SECURITY.md).
Where the project stands and what is next: [`docs/STATUS.md`](docs/STATUS.md).

## License

No license has been chosen yet; until one is added to this repository, all rights are reserved.
