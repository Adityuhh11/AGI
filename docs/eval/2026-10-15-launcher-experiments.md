# Launcher experiments (hotkey panel, sidecar latency, laptop inventory)

Date: 2026-10-15. Machine: RTX 4050 laptop, Windows 11 (build 26200), WebView2 155, Go 1.27.1, Wails v3.0.0-beta.29.
Throwaway spike programs (not committed): a Wails app with one hidden frameless always-on-top window and a real
`RegisterHotKey` global shortcut; the spike presses the hotkey itself (`keybd_event`) 15 times and measures from the hotkey
callback to JS events in the page. A separate "other app" (a console window started by the spike, killed by PID afterwards)
holds the keyboard focus, as in real use. Medians over 14 runs (the first warms the webview).

## 1. Hotkey to a focused, painted input

| approach | window call returns | input focused | painted | max painted |
|---|---|---|---|---|
| Wails `Show()` + `Focus()`, `Hide()` between (no other app holding focus) | 69 ms | 73 ms | 75 ms | 100 ms |
| the same with DWM transitions disabled | no change | 76 ms | 76 ms | 99 ms |
| the same with a real other app holding focus | 68 ms | 72 ms | 73 ms | 96 ms |
| raw `ShowWindow` + `SetForegroundWindow` | 7 ms | 49 ms | never (WebView2 not told it is visible) | not usable |
| **DWM cloak/uncloak** + Wails `Focus()`, window never hidden (other app holding focus) | 41 ms | 41 ms | 42 ms | 58 ms |
| the same with no other app holding focus | 10 ms | 13 ms | 17 ms | 21 ms |

Reading: Wails' own `Show()` costs about 70 ms (it tells WebView2 about visibility and repositions). Keeping the window
shown and toggling the DWM cloak (`DwmSetWindowAttribute(hwnd, DWMWA_CLOAK, ...)`) keeps WebView2 rendering, so the frame
is already painted when the window is uncloaked. Taking keyboard focus across processes is the remaining cost.
Verified on all 15 runs: after the hotkey the window is foreground, visible and not cloaked; after cloaking, **the
invisible window stays the foreground window** until `SetForegroundWindow(previous)` is called, so the launcher must
remember the previously active window and hand focus back (verified: the previous app is foreground again after every
hide). The raw Win32 foreground call alone does not give WebView2 keyboard focus; Wails `Focus()` does.

## 2. Hotkeys that are free on this machine (`RegisterHotKey` probe)

Taken: `Alt+Space`, `Win+Space`, `Ctrl+Alt+Space` (very likely another app's quick entry), `Win+Shift+Space`,
`Win+Alt+Space`, `Ctrl+Win+Space`. Free: `Ctrl+Space`, `Ctrl+Shift+Space`, `Alt+Shift+Space`, `Ctrl+Alt+K`, `Ctrl+Alt+J`,
`Ctrl+Shift+K`, `` Ctrl+Alt+` ``, `` Ctrl+Shift+` ``, `` Win+` ``, `` Alt+` ``, `Ctrl+Alt+;`. A global hotkey also steals that
combination from every application (Ctrl+Space and Ctrl+Shift+Space are editor shortcuts), so the default must be rare.

## 3. Search latency through the real sidecar (HTTP, GPU query embedding)

`python -m omni_memory` started cold: port open after 1.6 s, model ready after 32.5 s (torch import and weights; 10-15 s
warm). Sample Data indexed in 17 s (96 files, 431 chunks).

| | p50 | p95 | max |
|---|---|---|---|
| `POST /search`, k = 20, idle sidecar (60 requests) | 74 ms | 90 ms | 112 ms |
| `POST /search` while an index job runs (42 requests) | 226 ms | n/a | **1568 ms** |

Search is fast enough to run on every keystroke (after a short debounce). During indexing a query waits for the
embedding batch in flight (an image batch is the slow one), up to 1.5 s: acceptable for a one-off folder add, not for the
hour-long first index of a whole laptop. The engine needs query priority over indexing before the launcher indexes in
the background.

### 3b. Query priority over indexing (engine change A3)

`SerializedEmbedder` now cuts bulk calls into slices (4 texts, 1 image), takes the lock per slice and lets queued queries
run before the next slice. Sample folder re-indexed under a fresh source id while a loop searched (`POST /search/files`):

| | job time, no searches | search during the job (p50 / p95 / max) | job time with searches |
|---|---|---|---|
| before: whole bulk call holds the lock | 15.8 s | 220 / 1120 / 1340 ms (44 searches) | 16.5 s |
| slices of 8 texts, 1 image | 17.4 s (+10%) | 231 / 310 / 340 ms (101 searches) | 24.6 s |
| **slices of 4 texts, 1 image (adopted)** | 18.3 s (+16%) | 150-165 / 250-307 / 320-420 ms (about 150 searches) | 28-29 s |

Idle search stays 77-88 ms median. A query now waits for one slice (26 ms in the unit test against 1,051 ms with one big
slice). The tail near 300 ms is one image embedding plus the search itself. The job takes longer with searches running
because each search takes the GPU between slices; with nobody searching the cost is the 10-16% above.

## 4. What a laptop holds (counts and sizes only; `node_modules`, `.git`, venvs, `AppData`, build output skipped)

| root | files | by kind |
|---|---|---|
| Documents, Desktop, Videos | 26 | nothing indexable |
| Downloads | 864 | 399 PDFs (1.26 GB), 213 images, 36 Office, 36 text, 11 code, 13 videos; 153 other (9.2 GB of installers) |
| OneDrive | 1,315 | 602 images, 123 PDFs, 142 code, 108 text, 28 Office |
| `C:\dev` | 15,634 | 1,447 code, 1,039 text, 339 images, 12 PDFs, 189 audio; 12,606 other (13.8 GB) |

About 1,000 PDFs (every page is embedded as text and as an image), 1,200 images and 3,000 code and text files. At the
measured embedding rates (about 60 text chunks/s, about 4 images or PDF pages/s) the first full index is roughly an hour of
GPU time. Office files (about 70) are not supported by the engine yet. The scan itself is instant (under a second per root).

## 5. The real panel (block D, `cmd/panelbench`)

`go run ./cmd/panelbench` runs the real `internal/panel` against a stand-in page: 50 runs, a separate console window
holding the keyboard, the hotkey pressed with `keybd_event`, a wait until the page reports that its input has focus and has
painted, `Hide()`, and a check that the keyboard went back to the other app and that the bar has a sane size and place.
Six consecutive invocations: input focused and painted at a median of about 46 ms, 95th percentile 52-66 ms, maximum
64-68 ms, no misbehaving run. Under the 100 ms rule with a 1.5x margin.

Bugs this and an end-to-end check against the real app found (all fixed, each is a trap for the next Windows window code):

- `MonitorFromPoint` takes its POINT by value; on x64 the two halves travel in one register. Passing X and Y as two
  arguments made the monitor lookup fail and Windows shrank the window to its 40x38 minimum. The first version of the
  harness only checked focus and paint events, so it passed; it now checks the window's size and position.
- Wails creates a hidden window's native window only when it is first shown, and a shown-then-hidden web view stops
  rendering: the window is created (off screen) at start and cloaked, never hidden.
- A new window grabs the keyboard when it is created and again when the web view finishes initialising, so the launcher
  returned the keyboard once and still held it invisibly. It now watches for the first 6 seconds and gives the keyboard
  back each time it finds the closed panel holding it.
- A console build (`-Console`) opens a console window that takes focus: use the windowless build for focus checks.
- `io.MultiWriter` stops at the first write error and stderr is invalid in a windowless process, so the log file stayed
  empty; the console write is now best effort and the file comes first (`%LOCALAPPDATA%\Omni\logs\omni.log`).
- The harness itself raced once: the page reports "focus" and "painted" as two concurrent requests.
