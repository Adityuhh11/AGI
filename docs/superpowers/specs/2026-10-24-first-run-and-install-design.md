# First run and installation: from "unzip it" to "it found my file"

Date: 2026-10-24. Status: approved by the owner on 2026-10-24 with the recommendations in section 7. Roadmap phase U in `STATUS.md`.

## 1. Why

The product is stable enough for strangers; the first ten minutes are not. A person today gets a zip, unzips it somewhere, runs an
unsigned exe past a SmartScreen warning, and sees **Settings on the General page**. Nothing says what AGI is, what the hotkey is for, or that
file search exists. If they find File search, they download 0.9 GB, wait two minutes for a bar, add folders one picker at a time without
knowing what each will cost, and watch "41 of 96 files". Each step works; the journey does not teach.

Friction in the journey as built (v0.5 plus the S2 and S3 work), in order:

| # | Step | Friction |
|---|---|---|
| 1 | Get it | A zip. No install location, Start menu entry, shortcut or uninstall entry. "Start when I sign in" stores the exe's path, so moving the folder breaks it. Updates need a writable folder. |
| 2 | First start | Opens Settings on General. Nothing explains the product, the hotkey's purpose, or the keys. The tray icon hides in the overflow. |
| 3 | Find file search | A settings page among six; its value is not explained anywhere before the download. |
| 4 | Download | Two minutes of waiting on a page; the person cannot usefully do anything else. |
| 5 | Choose folders | One "Add folder" picker at a time. No suggestions (Documents, Pictures, Desktop...), no sizes, no way to leave a subfolder out in the UI (`exclude` is config-only). |
| 6 | Choose options | Three choices with sentences, but no number: what will *my* folders cost *this* computer? |
| 7 | Indexing | "N of M files" and a bar for one folder; no time left, no word that search already works; files are read alphabetically, so the newest documents may be last. |
| 8 | After | No "you're set" moment; nothing says closing the window does not stop indexing, or that the tray shows progress. |

## 2. What DigUp does (same model, same category, 5.7k stars)

Screens in `Digup Userflow/` (not committed), plus its repository:

1. **Welcome.** Name, tagline, three examples per file type ("that payment error", "the clause about pets in the lease"), then a privacy
   sentence: what is downloaded (865 MB, once), that it works offline afterwards, and what happens next.
2. **What should DigUp search?** Known folders (Documents, Pictures, Videos, Music, Desktop, Downloads) as checkboxes, Downloads
   unchecked, a "..." on each to leave subfolders out, "Add Folder...", and "only reads files: never changes, moves or uploads them".
   The footer shows "Downloading the model: 88% of 865 MB" and "Looking at your folders...": the download and the scan run **while the person decides**.
3. **Search your code too?** Code projects found by the scan, each with file count, size and time ("under a second, 13 KB"); off by default.
4. **Search from anywhere.** The hotkey as key caps, "Change Shortcut...", and a six-line cheat sheet of keys.
5. **You're all set.** A speed check of this computer, then "Reading your files: 56 of 922, about 10 min left; you can search right away, results
   fill in newest first", and "lives in the tray; closing this window doesn't stop it".

Beyond the screens: a per-user NSIS installer (no admin; uninstall offers "delete the application data" including the model and index), an
in-app one-click update, `CONTRIBUTING.md` (issue first, measure before and after), PR template, issue forms that say where the logs are and warn
they may hold file names, CI on every PR, a release workflow, SHA-256-pinned vendored dependencies, MIT licence, Security tab for vulnerabilities.

## 3. What we take, adapt, or leave

| DigUp does | Us |
|---|---|
| A first-run guide that says what the product is, with concrete examples | **Take**, but the product is a launcher first: the guide leads with apps, the web, the hotkey and keys, and presents file search as an optional step. DigUp has one feature; a stranger can use ours in 30 seconds with no download. |
| Download while the person decides | **Take.** Our install is 2 minutes, mostly waiting; the folder and option steps fill it. |
| Known-folder suggestions, Downloads off, leave-out per folder, "only reads" promise | **Take**, with the OneDrive-moved folders resolved (we already have `knownfolder_windows.go`) and the same promise. |
| Time per folder, measured on this computer | **Adapt, and go further.** We show time per *choice* too: the number a person needs is "what do Words only / Every page cost on my folders", and it changes as they click. A scan counts files by kind in Go while the engine downloads; the time appears when the device is known. |
| Benchmark step ("checking this computer's speed") | **Leave**: our engine already knows its device (`vulkan`, `cuda`, `cpu`) and we have measured rates for each; a live ETA corrects the estimate within the first minute. Add a real benchmark only if estimates prove wrong. |
| Newest files first | **Take.** The engine scans by name today; sort by modified time so recent files are searchable first. Small, and it makes "search works right away" true. |
| Live "N of M, about X min left", "search works now", tray tells progress | **Take.** |
| Separate code step | **Leave for now.** Our file search reads code with everything else (`code:` narrows it), and dev folders are what people add by hand. Revisit if code indexing needs its own time/space warning. |
| Per-user installer, uninstall that offers to delete data | **Take** (section 5), as Inno Setup. |
| CONTRIBUTING, templates, CI, release workflow, pinned vendor hashes | **Take** (section 6). Our installer already pins every download by SHA-256. |
| Cross-platform shell | Not now; Windows only. The guide is written in Svelte, so it carries over. |

Where we can be **better** than DigUp, not just equal:

- **The launcher works at once.** The guide never blocks on the download; "Not now" on file search ends the guide in four screens.
- **Choices with prices.** Folder and option costs update live; DigUp shows a per-folder time only.
- **Nothing starts until Start** (S2, already built), and the same screen later offers "Save and re-index".
- **Honest about hardware.** On a computer without a graphics card the guide says so before the download, preselects the cheap options, and shows the cost of the dear ones (measured: 8 minutes against 16 seconds for the sample folder).
- **Where files go.** The file-search step names the folder (about 1.1 GB engine and models, plus the index) and offers to put it on another drive; DigUp's installer asks for the application folder only.

## 4. The setup guide

A window like Settings (`#setup`, own chunk, own Svelte file), opened on first run instead of Settings, from the tray ("Set up AGI..."), and
from Settings > About ("Run the guide again"). It is a front end over the services Settings already uses (`settingsui`), so there is one source
of truth and the Settings pages stay the place to change things later. Existing installs (a `config.toml` already exists) do not see it unless asked.

Steps, each one screen with Back and Continue, progress shown in a footer (download, scan):

1. **Welcome.** "Press a hotkey, type, open anything." Three rows: apps and the web at once; your files, by meaning (optional); clipboard,
   snippets, pins. A privacy paragraph in the README's words (no telemetry; the only downloads are the ones you press). Continue.
2. **Your hotkey and the bar.** The hotkey as key caps with Change (the existing `HotkeyField`), "Start when I sign in", a "try it now" line,
   and the cheat sheet: `vsc` Enter opens an app, `?` the web, `gh rust` a quicklink, `clip:` the clipboard, `snip:` snippets, `Ctrl+K` actions,
   `Ctrl+,` or typing `settings`.
3. **Search your files too?** What it does in one example per kind; what it costs (0.9 GB download once, then offline; about 1.1 GB on disk;
   the card it will use); where it is saved and **Change folder...**; the hosts. Two buttons: **Download and continue** (starts the install at
   once, the footer shows progress) and **Not now** (skips to step 6; nothing is downloaded).
4. **What should AGI search?** Known folders as checkboxes (Documents, Pictures, Videos, Desktop, plus Downloads off), "Add folder...", and a
   "Leave out..." on each (a subfolder picker; writes `exclude`). Each row shows file counts by kind and size from the scan, and, once the
   engine is ready, a time. "AGI only reads files: it never changes, moves or uploads them."
5. **How thorough?** PDF pages, photos, videos, and how hard to work, with the total time for the chosen folders next to each option and
   the device sentence ("on your graphics card" / "on the processor: pictures are slow here"). Defaults follow the device (processor:
   Words only, photo names). This is the S2 choices block, reused.
6. **You're set.** **Start indexing** (or, if skipped, "Open Settings > File search any time"). Then progress with time left, "you can search
   right away, results fill in newest first", "AGI lives in the tray, closing this window does not stop it". **Done** closes the guide.

State of the guide is written to `config.toml` as the person goes (folders, choices, hotkey), so closing it half-way loses nothing; the only new
key is `setup_done` (set at step 6 or when the guide is dismissed, so it does not open again).

## 5. Installing it

Today: zip, run from anywhere. Wanted: **`agi-X.Y.Z-windows-x64-setup.exe`** built with Inno Setup (free, scriptable in CI, the
format winget handles best):

- per user, no administrator rights, default `%LOCALAPPDATA%\Programs\AGI` (writable, so the in-app update keeps working), folder page to change it;
- Start menu shortcut, optional Desktop shortcut, "Launch AGI" on the last page;
- closes a running AGI before upgrading;
- an Apps and features entry and an uninstaller with a question, "Also delete the search engine, models and index (about 1.2 GB)?", off by default,
  never asked during an update;
- the zip stays as the portable download.

The installer does not duplicate the guide: it decides where the program lives and how it is reached; the guide decides how it behaves
(hotkey, start at sign-in, file search). "Start when I sign in" therefore stays in the guide and Settings, and keeps using the Run key.
The updater is unchanged for installed copies (it replaces `agi.exe` in a writable folder); the Apps and features version number would
go stale, so the updater also writes it, or later the updater downloads the installer and runs it silently (decide in the plan).

The engine folder (`%LOCALAPPDATA%\AGI\engine`) and the index (`...\memory`) can move: `[search] data_dir` in `config.toml`, applied only before
a download (or by moving the two folders and restarting); the launcher already reads `AGI_ENGINE_DIR` and the engine `AGI_MEMORY_DIR`, so this
is one setting that sets both.

## 6. Making the repository ready for strangers (roadmap phase S4)

From DigUp, adapted: `CONTRIBUTING.md` (open an issue first; measure before and after for speed or quality changes; nothing that sends data
elsewhere), `SECURITY.md` (private vulnerability reporting), issue forms (bug: what happened, Windows version, AGI version, computer and
graphics card, and the log path with a warning that logs hold file names; feature), a PR template (issue link, tests run, evals if search
results changed), and **CI on every PR**: `go vet`, `go test ./...`, `npm test` and `svelte-check`, `uv run ruff check` and `uv run pytest` on
a Windows runner. Release workflow on a tag: build the zip and installer, attach SHA-256s and a build-provenance attestation. The licence,
signing and the project's name stay parked at the owner's request.

## 7. Decisions (owner, 2026-10-24)

1. **Installer:** Inno Setup, per user. The zip stays as the portable download.
2. **Where files go:** the guide shows the path and offers "Change folder..." (`[search] data_dir`).
3. **Order:** CI first (U0), then newest-first and time left (U1), then the guide (U2, U3), then the installer (U4).

## 8. Phases

| | Scope | Notes |
|---|---|---|
| U0 | CI workflow, `CONTRIBUTING.md`, `SECURITY.md`, issue forms, PR template | Small, no product risk; part of S4 |
| U1 | Newest-first scan in the engine; live "time left" from throughput in the Indexing block and the tray; "search works while it indexes" line | Measure on `Sample Data` and the real index |
| U2 | The guide's shell and steps 1, 2, 3, 6 (no estimates): first-run opens it, tray and About reopen it, `setup_done`, known-folder checklist and leave-out in step 4 | Reuses `settingsui` |
| U3 | `internal/survey` (files and bytes by kind, same skip rules as the engine) and `estimate` (rates per device and pace, calibrated on `Sample Data` and the owner's index); folder rows and step 5 show times | Calibration is measured, in `docs/eval/` |
| U4 | Installer (Inno Setup) and the release workflow; `data_dir` | Needs decision 1 and 2 |

## 9. Tests

`survey` against `Sample Data` (counts match what the engine indexed: 96 files, 8 denied) and a temp tree with skipped folders; `estimate` as a table
test; the engine's newest-first order (a test with files of different mtimes); guide state (what each step writes to `config.toml`; dismissing
sets `setup_done`; an existing config never opens it); frontend: pure helpers (`keycaps`, `formatEta`) with `node --test`. By hand in a clean profile,
including a run with the model forced to the CPU, and the installer on a clean Windows user.
