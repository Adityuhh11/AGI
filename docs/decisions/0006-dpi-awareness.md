# 0006: The launcher declares per-monitor DPI awareness in its manifest, and every build is checked

Status: accepted, 2026-10-27.

## Context

On 2026-10-24 `build/windows/agi.manifest` was embedded in `agi.exe` (commit `24ca3eb`, the installer work) with the note
"deliberately no DPI setting: the launcher sets its own", because Wails calls `SetProcessDpiAwarenessContext` at startup.
From then on every window (the bar, Settings, the setup guide, Notes, the timer) looked soft at the owner's 125% scale. It
was noticed with the notes and timer windows, and two CSS rounds (`98131c1` and the notes fixes) chased the symptom without
touching the cause.

## What was measured (125% display, Windows 11 build 26300)

| Build | Process DPI awareness | Sharpness of the same text (Laplacian variance, share of hard edges) |
|---|---|---|
| v0.7.0 (`29091e3`), HEAD, the owner's `bin\agi.exe` (manifest, no DPI setting) | 0, unaware | about 1120, 18% |
| HEAD with the manifest taken out | 2, per-monitor | 3099, 48% |
| HEAD with `dpiAware true/pm` and `dpiAwareness PerMonitorV2` in the manifest | 2, per-monitor | 3099, 48% |

A DPI-unaware process is drawn by Windows at 100% and stretched to 125% as a picture, so no CSS can make it sharp. With the
manifest present and silent about DPI, Wails' own call did not take effect (the process stayed at 0); without a manifest it
did. The cause is the combination, found by experiment, not by reading the Windows documentation.

## Decision

- The manifest declares per-monitor v2 awareness itself. Wails sees an aware process and leaves it alone.
- `scripts/check-dpi.ps1` starts the built exe in an empty profile and asks Windows for its DPI awareness; it fails unless
  it is per-monitor. `scripts/build.ps1` runs it last, so a build that would be soft does not come out of the build.
- CSS tricks for sharpness (`--line-width`, `non-scaling-stroke`, colour fades instead of opacity) stay: they are right
  for a process that is aware, and they are the reason rows and icons stay crisp when an element is composited.

## Rules that follow

- A change to the manifest, to how the process starts (anything that creates a window or sets DPI state before Wails), or to
  the Wails version is checked with `scripts/check-dpi.ps1` and by looking at a window at 125% scale.
- A rendering bug is first measured at the process level (awareness, window DPI against monitor DPI), then in the page.
