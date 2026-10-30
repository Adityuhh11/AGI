# Stability sweep, 2026-10-30

Two rounds of trying to break Omni before it is opened to other people: round 1 finds and fixes, round 2 reviews the fixed branch from scratch
and tests again. Branch `test/stability-sweep`, cut from `dev`.

## What could and could not be run

| Where | What ran |
|---|---|
| Linux container (development) | Python engine: `ruff`, `pyright`, `pytest` (737 tests, three runs, no flake), hostile-file indexing, 1,563 hostile search queries over the API. Page: `svelte-check`, 114 tests, a markdown timing run on pathological input. Go: `go vet` for Windows, the portable packages (and, through throw-away Linux stand-ins for the Windows and Mac calls that were never committed, most of the rest) under `-race`, repeated and shuffled; fuzzing of the calculator, timer, settings file, window text, snippets, notes and page extractor |
| GitHub runners, `.github/workflows/stability.yml` (on every push to a `test/**` branch) | The whole Go suite on Windows and on macOS: `gofmt` (Mac), `go vet`, `go test -race -shuffle=on -count=3`, the page tests and types, and the engine suite with `ruff` and `pyright` |
| Needs a person on a real machine | The hotkey, DPI at 125% and 150%, the tray, drag of the bar, the permission cards, the installer UI, a different graphics card, sleep and wake. See "For a person" below |

## Round 1: found, and what was done

| # | Severity | Finding | Where | Result |
|---|---|---|---|---|
| 1 | Medium | A tar archive with a link that points back at the folder (`p -> .`) made a later link look inside the folder by its name while it really sat higher up; a file written through that link left the folder. Reproduced with a test first. (Archives are pinned by SHA-256, so this needs a wrong pin or a compromised download, but the check claimed to refuse it.) | `internal/enginesetup/untar.go` | Fixed: where an entry lands is judged after the links on the way are followed; a file no longer opens through a link of the same name |
| 2 | Medium | The Windows folder watcher gave up on a folder at its first error or when it was missing at start (a USB drive unplugged, a network share dropped): changes there were never indexed until Omni was restarted | `internal/watcher/watch_windows.go` | Fixed: retried every 30 s, and the folder is reported as changed when it comes back, since what happened meanwhile is unknown. Test on the Windows runner |
| 3 | Low | `Watch` could block forever if its context ended before the operating-system wait began: the single cancel found nothing to cancel | `internal/watcher/watch_windows.go` | Fixed: the cancel is repeated until the read stops; the handle is closed after the last cancel. Test: 50 starts with an already-ended context |
| 4 | Medium | Settings, notes, chats, state, clipboard history and the app list were written aside and renamed without being flushed to disk. After a power cut the new name can point at an empty file (the settings are then set aside and reset; a chat or note is lost) | six places | Fixed: `internal/atomicfile` flushes before the rename and is used by all six, replacing six copies of the same code |
| 5 | Medium | A search holding a NUL character made the file-name search fail with `sqlite3.OperationalError: unterminated string`, which the API answers as a server error. Found by 1,563 generated queries; it was the only failure | `sidecar/src/omni_memory/store/keyword.py` | Fixed with a test |
| 6 | Low | 21 Go files were not `gofmt`-clean; nothing in CI checked | repository | Formatted; the Mac CI job now fails a pull request that is not clean |
| 7 | Low | A data race in a test's fake engine (not in the product) showed under `-race` | `internal/search/search_test.go` | Fixed |

### Found and not changed (decisions for the owner)

| Severity | Finding | Why it was left |
|---|---|---|
| **Blocker for open source** | There is no `LICENSE` file; the README says none is chosen | Your decision |
| Medium | No third-party notices are shipped. The binary embeds OFL-licensed fonts (Geist, Geist Mono, Sora) and Apache-2.0 shaders; the Gemma model is downloaded under its own terms; the engine pulls MPL-2.0 `pathspec` | Needs the licence choice first; then a `THIRD_PARTY_NOTICES` file and an "About" line |
| Medium | Pictures of 90 to 179 megapixels are decoded whole and spike memory to about 1.7 GB (Pillow only warns until twice its limit). JPEG draft decoding would cut both memory and time, but changes the pixels the model sees slightly | Quality-affecting: needs the golden eval before and after, per `CONTRIBUTING.md` |
| Low | The engine keeps one port for the life of the launcher. If another program takes it while the engine is down (an ephemeral port can be handed out), file search stays down until Omni restarts | Rare; the client holds the address, so picking a new port on restart is a small design change |
| Low | Excluding a folder whose name holds `:` or `\` is refused by the API (`_relative_folders`); legal on a Mac | Rare; needs a decision on what a folder name may hold |
| Low | The history search in Omni AI reads and parses every conversation file on each keystroke | Not measured with a large history yet |
| Low | The update is checked by the SHA-256 GitHub publishes with the release (integrity, not authenticity); nothing is signed | Known and documented (`docs/DISTRIBUTION.md`) |
| Low | `npm audit`: two low advisories in `deepmerge`, reached only through the build tool `@sveltejs/vite-plugin-svelte`; no fix exists | Build-time only, not shipped |
| Info | `govulncheck` could not run here (it was built with an older Go than the project needs). Worth adding to CI | |

### Looked at and sound

Markdown renderer (escapes first, links limited to http, https and mailto; 200,000 lines in 0.5 s); the settings store (serialised updates, a bad file is set
aside, nothing changes unless it validates); the update download and swap (probe, digest, one named file, rollback); `Unzip`; the asset server's `/thumb`
(inside the indexed folders only, no links followed); the engine's API (loopback only, host check, constant-time token, unknown fields refused, path rules);
calculator and timer parsing (fuzzed, depth and length capped); the Windows clipboard code (thread-bound, closed on every path); hostile files (empty,
garbage, truncated, 300 MB document bomb, 144-megapixel picture, a sparse 50 GB file, link loops, 60-deep folders, odd names): every one was skipped or
reported, none crashed the run.

## For a person on a real machine

- Windows: press the hotkey 20 times quickly; open the bar at 100%, 125% and 150% scale; sleep and wake with file search on; unplug a USB drive that is an indexed folder, plug it back, and change a file on it (it should be indexed within a minute: finding 2); run the installer on a clean user.
- Mac: the same on a Retina display; deny and then allow the folder permission and Accessibility; the Mac hotkey fix and dragging the bar, which `STATUS.md` lists as not yet confirmed.
