# Omni on macOS: the same launcher, the same features

Date: 2026-10-29. Status: **approved by the owner on 2026-10-29** with the changes below. Roadmap phase M in `STATUS.md`.

## 0. Owner's decisions (they override the sections they touch)

1. **No "cloak" on the Mac.** The panel is shown and hidden with the window's ordinary show and hide (candidate A of 5.1). The cloak
   candidates B and C are dropped. If the first frame turns out stale or blank on a real Mac, that is a bug to fix then, not a design to build now.
2. **No in-app update on the Mac for now** (5.10 is deferred until the app is signed and notarized). Until then a Mac user deletes the app and downloads the new one.
   The opt-in update *check* still works and offers the release page, with no install button.
3. **The bundle id is `io.github.adityuhh11.omni`**, and `install.sh` is published.
4. **Windows and Mac code are kept visibly apart** so a bug can be triaged to one system at a glance: shared logic lives in `x.go`, Win32 code
   only in `x_windows.go`, Cocoa/`open`/`pmset` code only in `x_darwin.go`, and a file that is pure Go but only makes sense on one system is also
   suffixed (`parse_windows.go` for the Start Menu JSON). `docs/ARCHITECTURE.md` lists every such pair (section 13).
5. The stable self-signed identity and the smoke build after M1 are decided later ("we will see").

## 1. Goal and non-goals

A Mac user downloads Omni, opens it, presses the hotkey and gets what a Windows user gets: apps, the web, the clipboard history and
snippets, notes, the timer, file search, Omni AI, settings, the first-run guide, autostart and the one-click update. Not a reduced
"preview": parity is the exit criterion (section 11).

| In | Out (for now) |
|---|---|
| Apple Silicon (arm64), macOS 13 Ventura and later (13 is where `SMAppService`, the login-item API, starts) | Intel Macs. Apple and Homebrew are ending Intel support, and `macos-latest` runners are arm64. |
| Distribution without a paid Apple account (ADR 0010) | Developer ID signing, notarization, the Mac App Store, a Homebrew cask: all need the paid account or a signed app. They are added later without redesign. |
| Same Go core, same Svelte pages, same Python engine | A second UI, Linux. |

## 2. What is different about this port

1. **Nobody here owns a Mac.** The owner develops on Windows, and a friend has the only Mac. The macOS code uses cgo (Objective-C),
   which cannot be compiled or type-checked from Windows. So: (a) a `macos-latest` job in CI is the compiler, test runner and
   bundle builder from the first commit (section 9); (b) everything that can be plain Go is plain Go and is tested on Windows with
   fakes, as the clipboard `Paster` and `Source` already are; (c) cgo is kept to a few small files that each do one OS call; (d) the friend
   is the only real test, so he gets a numbered checklist (section 10), not "tell me if it works".
2. **A Mac asks the person for permission** where Windows does not (typing into other apps, reading Documents or Desktop). Each is a
   designed screen in the first-run guide, never a surprise (section 8).
3. **No file-extension convention for "not Windows".** Go builds `x_darwin.go` only on macOS. Where a package has only `x_windows.go` today, the
   shared code stays in `x.go` and a `x_darwin.go` is added; a `x_other.go` (`!windows && !darwin`) is **not** added, because
   the product supports two systems and a stub that pretends to support a third is slop.

## 3. What Wails already gives us (read in `wails/v3@beta.29`, not assumed)

| Need | Wails on macOS |
|---|---|
| Global hotkey | `GlobalShortcut` on Carbon `RegisterEventHotKey`: no Accessibility permission, works while another app is frontmost. `Alt` means Option, `Super` means Command. |
| Panel window | `MacWindowClassPanel` (an `NSPanel`) with `PanelPreferences{FloatingPanel, NonActivating, BecomesKeyOnlyIfNeeded}`, `Mac.WindowLevel`, `Mac.CollectionBehavior`. Its window class already answers `canBecomeKeyWindow = YES`, so a non-activating panel still takes typing. |
| Tray | `systray` (status item), menu, template icon. |
| Login item | `autostart_darwin_smappservice.go`: `SMAppService.mainAppService` register and unregister. |
| Single instance | `single_instance_darwin.go`. |
| Clipboard text | `app.Clipboard.SetText/Text`. No change counter, no owner: ours (below). |
| Screens | `app.Screen` with work area and scale factor. |
| No Dock icon | `Mac.ActivationPolicy: Accessory`. Changing it at run time is `[NSApp setActivationPolicy:]`, a one-line cgo call of ours. |

This removes the hardest items (hotkey, panel class, autostart) from "write natively" to "configure and measure".

## 4. The map: every Windows-only seam and its Mac equivalent

Effort: S under 100 lines, M a day, L several days. "cgo" means a small Objective-C file. The right column is the section that explains it.

| Package (Windows file) | What Windows does | What the Mac does | Effort | § |
|---|---|---|---|---|
| `panel` (`win32_windows.go`) | DWM cloak, `SetWindowPos`, `GetForegroundWindow`, per-monitor DPI | Non-activating `NSPanel`, alpha/ordering "cloak", `NSScreen.visibleFrame` under the mouse, frontmost app via `NSWorkspace` | L (cgo) | 5.1 |
| `hotkey`, `panel` hotkeys | Wails `GlobalShortcut` | Same code, Carbon underneath | S | 5.2 |
| `clipboard` (`win32_windows.go`) | `GetClipboardSequenceNumber`, owner window, private formats, `SendInput` Ctrl+V, elevation check | `NSPasteboard.changeCount`, frontmost app as owner, `org.nspasteboard.*` private types, `CGEventPost` ⌘V, Accessibility and Secure Input checks | L (cgo) | 5.3 |
| `apps` (`list_`, `launch_`, `focus_`, `icon_windows.go`) | `Get-StartApps` via PowerShell, `ShellExecute`, window enumeration, `IShellItemImageFactory` icons | Scan the application folders for `.app`, `open`, `NSRunningApplication.activate`, `NSWorkspace.icon(forFile:)` to PNG | L (cgo for icons) | 5.4 |
| `files` (`shell_`, `players_windows.go`) | `ShellExecute`, `explorer /select`, registry App Paths, VS Code path | `open`, `open -R`, `open -a`, `/Applications` lookups, VS Code CLI | M | 5.5 |
| `web` (`apppath_windows.go`) | Registry App Paths, `ShellExecute` | `.app` lookup, `open` | S | 5.5 |
| `watcher` (`watch_windows.go`) | `ReadDirectoryChangesW` | FSEvents stream (cgo) | M | 5.6 |
| `autostart` | `HKCU\...\Run` | Wails `SMAppService` | S | 5.7 |
| `sidecar` (`exec_windows.go`) | `taskkill /T`, Job Object kill-on-close | Process groups plus a reaper process, ending the group (no job object exists) | M | 5.8 |
| `enginesetup`, `llm` (`.exe` paths, zip pins, Vulkan) | `uv.exe`, `llama-server.exe`, `Scripts\python.exe`, `.zip`, Vulkan | `uv`, `llama-server`, `bin/python`, `.tar.gz` or `.zip`, Metal | M | 5.9 |
| `update` (`restart_`, `swap.go`, `fetch.go`) | Rename the running `omni.exe`, breakaway from the job, `OpenProcess` wait | Replace `Omni.app`, `kill(pid, 0)` wait | M | 5.10 |
| `config`, `cmd/omni`, Python `config.py` | `%APPDATA%`, `%LOCALAPPDATA%` | `~/Library/Application Support/Omni`, `~/Library/Logs/Omni` | S | 5.11 |
| `system` (`system_windows.go`) | Lock, sleep, Task Manager, Recycle Bin, Settings, beep | `pmset displaysleepnow`, `pmset sleepnow`, Activity Monitor, empty Trash, System Settings, `afplay` | S | 5.12 |
| `power` | `GetSystemPowerStatus` | `IOPSCopyPowerSourcesInfo` (cgo) or `pmset -g batt` | S | 5.12 |
| `settingsui` (`knownfolder_`, `keymenu_windows.go`) | Known Folder API, `WM_SYSCOMMAND` filter for Alt+Space | `~/Documents` etc. with iCloud awareness; no filter needed | S | 5.13 |
| `tray`, windows (`settingsui/window.go`) | Notification-area icon | Status item (template icon) and the Dock icon policy | M | 5.14 |
| Frontend (`keycaps`, `accelerator`, `Hints`, copy) | "Ctrl", "Win", "Explorer", "Start menu" | ⌘ ⌥ ⇧ ⌃ glyphs, "Finder", "Applications" | M | 5.15 |
| Build (`build.ps1`, `release.ps1`, Inno) | `omni.exe`, zip, installer | `Omni.app`, zip, `install.sh` | M | 6 |

Everything not in this table (search, ranking, palette, state, notes, timer, chat, snippets, web search, the Python engine's logic) is
already portable: the grep for Windows code found no other seam.

## 5. Designs

### 5.1 The panel (the part that decides whether it feels like a native Mac launcher)

**What Spotlight, Alfred and Raycast do**, and what Apple's own engineers tell developers to do (links in section 12): an
*accessory* app (no Dock icon, no menu bar of its own) showing a *non-activating* `NSPanel` at a floating level, with collection
behavior that lets it appear on every Space and over full-screen apps. A normal `NSWindow` activates the app when it opens, which
yanks the person out of a full-screen app's Space and makes the app they were in lose focus. A non-activating panel takes the
keyboard for typing **without** making Omni the active app, so the app underneath stays active and gets focus back by itself when the panel closes.

```
Window class       MacWindowClassPanel, Frameless, transparent background, DisableShadow (the page draws its own)
Panel preferences  FloatingPanel, NonActivating
Level              floating (raise to "status" only if a measurement shows full-screen apps cover it)
CollectionBehavior CanJoinAllSpaces | FullScreenAuxiliary | IgnoresCycle
Activation policy  Accessory
```

**Consequences for the code that exists today** (and how little of it changes):

| Today (Windows) | Mac |
|---|---|
| `previous uintptr`: the window that had the keyboard, remembered at `Show`, given back at `Hide` | The panel never took activation, so there is nothing to give back. `Previous()` is "the frontmost application now" (`NSWorkspace.frontmostApplication`, its pid as `uintptr`), read when the paste happens. Hiding the panel is enough. |
| `cloak(hwnd)`: DWM keeps the page painted so showing is instant (41 ms vs 72 ms for `Show()`) | The same idea needs a Mac measurement, below. |
| `place(work, dpi, height)` in physical pixels | Reused unchanged by passing the work area **in points** and `dpi = 96` (scale 1): the widths are already in device-independent pixels, and a point is one. Cocoa's bottom-left origin is flipped to top-left inside the cgo function so `placement.go` never learns about it. |
| `cursorMonitor()`: the monitor under the mouse | `NSEvent.mouseLocation` and the `NSScreen` that contains it; `visibleFrame` excludes the menu bar and the Dock, as the work area excludes the taskbar. |
| `Resize(heightDIP)`: `SetWindowPos`, top-left fixed | `setFrame:display:` with the top edge held (Cocoa's origin is the bottom, so the origin is recomputed). |
| `WindowLostFocus` hides the panel | `windowDidResignKey` through the same Wails event (to verify it fires for a panel). |

**The cloak question is a measurement, not a decision** (AGENTS.md: measure, don't guess). Three ways to make the second and later
openings instant, tried in this order against the budget "hotkey to painted, focused input ≤ 50 ms p95, the Windows figure":

- A. `orderOut`/`orderFront` (what Wails' `Hide()`/`Show()` do). Simplest. Risk: WebKit stops rendering a window that is not on screen, so the
  first frame after showing is stale or blank (the same effect that cost 72 ms on Windows).
- B. Keep the window ordered in with `alphaValue = 0` and `ignoresMouseEvents = YES`, then set alpha 1. The page keeps rendering. Risk: a fully
  transparent window may count as occluded and be throttled by WebKit (to measure), and it stays in Mission Control unless `IgnoresCycle` and
  `Stationary` are set.
- C. Keep it ordered in but moved far off screen (the Windows start state). Risk: AppKit clamps a window to the screens unless
  `constrainFrameRect:toScreen:` is overridden, which means a class of our own.

Also required: **App Nap off.** An accessory app with no visible window is a candidate for App Nap, which delays timers and the
clipboard poll. `ProcessInfo.beginActivity(options: .userInitiated)` for the life of the launcher (cgo, 5 lines) keeps the hotkey
and the history prompt. The benchmark (`cmd/panelbench` exists for Windows) gets a Mac sibling, run by the friend on the real machine, and its numbers go in `docs/eval/`.

### 5.2 Hotkeys

Wails does it (section 3). Our work: (a) `config.DefaultHotkey` per OS. The Windows default `Alt+Shift+Space` is kept on Mac as
Option+Shift+Space, which is free (Spotlight is ⌘Space, Alfred ⌥Space, Raycast ⌥Space), so no second default is needed *(verify
that the shortcut registers while the friend's input sources are English and a second one)*; (b) the settings hotkey field records ⌘ as
`Super` and shows glyphs (5.15); (c) a combination that macOS reserves (⌘Space with Spotlight on) fails inside `Register`, and the existing
`HotkeyError()` path already tells the person in the tray and in Settings. Carbon hotkeys fire on the main thread, so `Toggle` stays cheap as it is.

### 5.3 Clipboard history and paste

The shared part (store, filters, ignore list, `Paster` order of events) is already behind interfaces and tests; only the leaves are new.

| `Source` method / `Paster` field | Mac implementation |
|---|---|
| `Sequence()` | `NSPasteboard.general.changeCount`. The same cheap counter, polled the same way (`bar.OnShow = clips.Poll` plus the timer). |
| `Read().Text` | `string(forType: .string)`; over `MaxText`, `ErrTooLong`. |
| `Read().Owner` | macOS does not say who copied. The best available truth is the frontmost app at the moment the count changed, which is right for a copy by hand and wrong for a background tool. It feeds the ignore list (password managers). Documented as a limit. |
| `Read().Private` | The community convention at nspasteboard.org: types `org.nspasteboard.ConcealedType` (password managers set it: 1Password, Bitwarden, Keychain-style), `TransientType`, `AutoGeneratedType`. Any present means private. Plus the existing `DefaultIgnore` names matched against bundle name. |
| `Previous` | frontmost app's pid, as in 5.1. |
| `Foreground` | frontmost app's pid; with a non-activating panel it equals `Previous` immediately, so the existing wait loop returns at once. |
| `Elevated` → **secure input** | `IsSecureEventInputEnabled()`: a password field somewhere holds it and no app may synthesize keys. Returns the Mac message "Copied. A password field is active; press ⌘V yourself." (replaces `ErrElevated`'s wording by OS, the struct field is reused). |
| `SendPaste` | `CGEventCreateKeyboardEvent` for virtual key 9 (V) with `kCGEventFlagMaskCommand`, down and up, posted to the session event tap. **Needs the Accessibility permission** (`AXIsProcessTrusted`). Not trusted → the text is on the clipboard already; the error message offers "Open Accessibility settings" (`x-apple.systempreferences:com.apple.preference.security?Privacy_Accessibility`). |
| `SetText` | `app.Clipboard.SetText` (already used). |

The own-copy filter (`strings.TrimSuffix(filepath.Base(self), ".exe")`) becomes the bundle's name on Mac.

### 5.4 Applications: list, launch, focus, icons

- **List.** Walk `/Applications`, `/System/Applications` (+ `/System/Applications/Utilities`), `/Applications/Utilities`, and `~/Applications`, one level deep
  plus the Setapp-style one-level subfolders, for `*.app`. Name from `CFBundleDisplayName`, falling back to `CFBundleName`, falling back to the
  folder name; localized names (`InfoPlist.strings`) are a later refinement. This is a directory read plus a `plist` parse, in the tens of
  milliseconds, so it needs none of Windows' PowerShell cache gymnastics (`apps.json` stays as the cache for the first keystroke). `ID` is the bundle path (stable across refreshes,
  as the Windows ID is). Plist parsing: `howett.net/plist` would be a new dependency; the Info.plist of an app is XML or binary,
  so if binary plists are common among the apps found, the dependency earns its place; otherwise `plutil -convert json` via `exec` costs nothing.
  **Decision deferred to M1 with a count of how many real `.app` files are binary** (they are on most Macs: Apple ships binary plists).
  Likely outcome: `plutil -convert json -o - Info.plist` (a macOS tool, always there).
- **Refresh trigger.** The same as `StartMenuFolders()`: the application folders are watched with the new watcher (5.6) or polled each minute.
- **Launch / focus.** One call does both: `open <bundle>` raises a running app and starts a stopped one (what Launch/focusRunning do in two steps on Windows).
  The "uninstalled since the list was made" check (`os.Stat` of the bundle) stays, with `ErrGone`.
- **Icons.** `NSWorkspace.shared.icon(forFile: path)` rendered into a 64 px `NSBitmapImageRep` and encoded to PNG (cgo, around 40 lines), cached as today in
  `iconDir` by hash of the path. Reading the `.icns` ourselves would need an extra decoder for little gain.
- **"Run as administrator", "Show in Explorer"** → the latter is "Show in Finder" (`open -R`); the former does not exist and is hidden from the Mac action list.

### 5.5 Files, editors, players, browsers

`files.opener` is an interface; add `macShell`: `Default` = `open path`; `Reveal` = `open -R path`; `Editor` = VS Code's `code -g path:line` if found
(`/usr/local/bin/code`, `/opt/homebrew/bin/code`, inside `/Applications/Visual Studio Code.app/Contents/Resources/app/bin/code`), else `open -t path` (the default text editor) — the Notepad fallback
equivalent. Players: the known-player table already has per-player argument shapes; on Mac they are found as `.app` bundles
(`IINA`, `VLC`, `mpv` from Homebrew) and started with `open -a <App> --args ...`. Browsers: `web.Installed()` checks `.app` names in `/Applications` and `~/Applications`
(`Google Chrome`, `Safari`, `Firefox`, `Brave Browser`, `Arc`, `Zen`, `Vivaldi`...); opening an address with one is `open -a "<App>" <url>`;
the default browser is `open <url>`. The `knownBrowsers` list gets a Mac display-name map beside it.

### 5.6 Folder watching

`ReadDirectoryChangesW` watches a whole tree through one handle; the Mac counterpart is **FSEvents** (`FSEventStreamCreate`), one stream
for all roots, file-level events, with the stream's own latency as the first debounce; the existing `debounce.go` and `filter.go` stay.
Rejected: `fsnotify` (a new dependency, and on macOS it is kqueue, which opens one file descriptor per watched file and directory:
the owner's index is 56,252 chunks over thousands of folders, which is the descriptor limit's territory). FSEvents is cgo (`CoreServices`), about
80 lines, with a Go callback that sends paths on a channel. History is not replayed (`kFSEventStreamEventIdSinceNow`): a missed change is caught by the
rescan the manager already does at start.

### 5.7 Starting at login

Wails exposes `SMAppService`. It shows in System Settings → General → Login Items, where the person can switch it off; `Enabled()` reads the service
status (`.enabled`; `.requiresApproval` counts as off, with the same repair message the Windows version gives for "switched off in Task Manager").
Needs the app to be in a bundle, hence not testable with a bare binary: the tests inject the API.

### 5.8 Child processes that must die with the launcher

Windows puts the launcher in a Job Object with kill-on-close, so the engine and the chat server die even if the launcher is killed from
Task Manager. macOS has no such object and no `PR_SET_PDEATHSIG`. What it has, and what we use: **a closed pipe is an exit notice.**

1. At start the launcher re-executes its own binary as `omni --reaper` (a hidden subcommand in `cmd/omni`) holding the read end of a pipe;
   the launcher keeps the write end. The reaper does nothing but read lines.
2. Each child is started with `Setpgid: true` (its own process group; the engine's llama-server inherits it). The launcher writes the
   child's pgid to the pipe.
3. When the launcher dies for any reason, the kernel closes the write end, the reaper reads EOF and sends `SIGKILL` to every registered group, then exits.
   A child that ends normally is removed (`unregister pgid`).
4. `Kill()` for a normal stop is `kill(-pgid, SIGTERM)`, then `SIGKILL` after a grace period: what `taskkill /T /F` does for the tree.

Pure Go (`os/exec`, `syscall`), no dependency, testable on Windows only as logic: a test with a fake `pgid` writer plus one real test gated by an
env var that the CI macOS job sets. The Python engine's own `_die_with_this_process` (a Windows Job Object around llama-server) becomes unnecessary on
Mac because the group covers it; the function already returns early when `os.name != "nt"`.

### 5.9 Installing the engine and Omni AI on a Mac

| Item | Windows pin | Mac pin |
|---|---|---|
| `uv` | `uv-x86_64-pc-windows-msvc.zip` | `uv-aarch64-apple-darwin.tar.gz` (tar, not zip) |
| `llama.cpp` | `...-bin-win-vulkan-x64.zip` (b11719 and b11970) | `...-bin-macos-arm64` (the workflow in llama.cpp's repo builds it with Metal and the shader library embedded; whether the file is `.zip` or `.tar.gz` is read from the release page at pin time) |
| Python | `venv\Scripts\python.exe` | `venv/bin/python` |
| Models | GGUF files | the same files, same SHA-256 |

- `enginesetup.Layout` gets OS-specific names (`Python()`, `LlamaServer()`, `UV()`) in `layout_windows.go` / `layout_darwin.go`; callers do not change.
- `Downloader.Unpack` handles `.tar.gz` besides `.zip` (stdlib `archive/tar`, `compress/gzip`), keeping **file modes** (the executable bit matters
  on a Mac, and `Unzip` must restore it), and refusing path traversal exactly as `Unzip` does.
- The pins are per-OS `Artifact` values chosen at build time (`pins_windows.go`, `pins_darwin.go`), each pinned by size and SHA-256 like today's.
  Per-pin real downloads happen once on a real Mac to take the numbers (docs/eval).
- **Device choice.** `ChooseDevice` / `choose_device` pick by GPU *name* among Vulkan devices. On Apple Silicon the only device is
  Metal (`MTL0` or similar) and the GPU shares the Mac's unified memory, so the 6 GB VRAM rule does not apply: on Mac, **offload everything**
  (`-ngl 99`) and pass no `--device`. The CPU fallback stays for a failed start. Embedding runs on the GPU and Omni AI chat shares it;
  they fit unified memory easily on 8 GB Macs *(measure on the friend's Mac: RAM pressure with both models loaded)*.
- Downloaded files are written by our process, not by a browser, so they carry no quarantine flag and Gatekeeper does not look at them. They must still
  run on Apple Silicon, which requires every executable to carry at least an ad-hoc signature: the llama.cpp and python-build-standalone binaries
  are linker-signed *(verify on first install; if one is not, the installer runs `codesign -s - --force` on it, which is free)*.
- Python wheels: `uv.lock` already lists arm64 macOS wheels for `av`, `numpy`, and the rest (checked), so `uv sync --frozen` needs no change.
  The error hint about "os error 448" is Windows-only and is kept behind the OS.

### 5.10 The one-click update

The release asset is `omni-X.Y.Z-macos-arm64.zip` holding `Omni.app`; the code that finds it (`assetOf`), checks GitHub's digest, and downloads is unchanged
apart from the name. What changes is the install step:

1. `exePath` is `…/Omni.app/Contents/MacOS/omni`; the **bundle** is the unit: its root is three levels up.
2. Download and verify the zip (as today), extract **the bundle** next to the running one (`Omni.app.new`) with the executable bits and the signature files intact. Go's
   `archive/zip` keeps modes; the zip is made on the CI runner with `ditto -c -k --keepParent`, which also keeps symlinks (our bundle has none).
3. `swap`: rename `Omni.app` → `Omni.app.old`, `Omni.app.new` → `Omni.app`. macOS allows replacing a running app's bundle (the running binary stays mapped), so the
   Windows retry loop is kept only for the rename.
4. `StartNew`: `open -n <bundle> --args --wait-for <pid>`; `WaitForExit` polls `kill(pid, 0)`. No job to break away from.
5. The file the app downloads has no quarantine flag, so the new version opens without the Open Anyway steps. The Accessibility grant **is** affected: see 6.2.

### 5.11 Where things live

Windows keeps config in `%APPDATA%\Omni` and everything else in `%LOCALAPPDATA%\Omni`. Mac:

| What | Where |
|---|---|
| `config.toml`, `state.json`, notes, chats | `~/Library/Application Support/Omni/` (what `os.UserConfigDir()` returns) |
| The engine, models, index, Omni AI model, icons, thumbnails, clipboard history | `~/Library/Application Support/Omni/` as well (*not* `~/Library/Caches`: cleaners empty it, and 1 GB of models and an hours-long index are not a cache) |
| Log | `~/Library/Logs/Omni/omni.log` (Console.app shows it) |

One small function, `config.DataDir()` (Windows: the existing cache dir + `Omni`; Mac: Application Support), replaces the seven
`os.UserCacheDir()` call sites listed by `grep` (`cmd/omni/main.go`, `remove.go`, `enginesetup/layout.go`, `llm/layout.go` ...) and the Python engine's `LOCALAPPDATA`
fallback is removed: the launcher always sets `OMNI_MEMORY_DIR`. Windows paths stay byte-for-byte what they are, so existing users are untouched.
`config.Validate` rejects `\\` UNC data dirs and checks "a local drive"; on Mac it checks the path is absolute and not inside the app bundle.

### 5.12 System actions, battery, beep

`sys:lock` → `pmset displaysleepnow` (locks if "require password immediately" is on, which is the macOS default) or the `CGSession -suspend` helper
*(verify which still exists on macOS 15 and 26; if neither locks reliably, drop the action on Mac and say so, rather than ship one that does nothing)*;
`sys:sleep` → `pmset sleepnow`; `sys:taskmgr` → `open -a "Activity Monitor"`; `sys:winsettings` → `open "x-apple.systempreferences:"`;
`sys:recycle` → `osascript -e 'tell application "Finder" to empty trash'` (asks for the Automation permission for Finder the first time; the
person already sees Finder's own confirmation, as Windows asks). Shut down and restart stay out, as on Windows. The ids keep their prefix; the
labels come from the table, so the bar says "Lock screen / Sleep / Activity Monitor / System Settings / Empty Trash" on Mac.
`Beep()` → `afplay /System/Library/Sounds/Glass.aiff` (or `NSSound.beep`). `OnBattery()` → `IOPSCopyPowerSourcesInfo` in cgo, or one `pmset -g batt` parse; the second costs no cgo
and is called once a minute, so it is the choice.

### 5.13 Known folders, the key menu

`knownFolder("Documents")` is `~/Documents`, `Pictures`, `Movies` (shown as "Videos"), `Downloads`, `Desktop`, `Music`. Where Windows resolves OneDrive moves
(`movedTo`), the Mac equivalent is iCloud Drive's "Desktop & Documents": the real files live in `~/Library/Mobile Documents/com~apple~CloudDocs/…` and `~/Documents`
is a link into them; resolving the link (`filepath.EvalSymlinks`) gives the same de-duplication the Windows code gets. `KeyMenuFilter` is Windows-only (it exists because
Windows swallows Alt+Space); on Mac the page receives the key and `application.WindowsOptions` is ignored, so only `keymenu_windows.go` stays and `main.go` builds the option behind a small
`keyOptions()` per OS.

### 5.14 Tray, Dock, menu bar, windows

- **Status item** from the same `tray.go`. A Mac menu-bar icon is a monochrome *template image* (black with alpha; the system tints it for light and dark
  and the selected state), 18 pt, so a template `tray-template.png` (1x and 2x) is added; the Windows colour icon stays. The tray menu content is unchanged ("Open", "Notes", ...).
- **Dock.** `Accessory` by default: no Dock icon, no ⌘Tab entry, which is right while only the bar is open. But Settings, Notes, Chat and the guide are ordinary
  windows, and an accessory app's windows cannot be ⌘Tabbed to and its menu bar never appears. So `settingsui.Window` switches the policy to
  `Regular` while at least one of those windows is open and back to `Accessory` when the last closes (a reference count; one cgo call). Raycast and similar tools do this.
- **Application menu.** With `Regular` active the app needs a menu bar: App (About Omni, Settings ⌘,, Hide, Quit ⌘Q), Edit (undo/cut/copy/paste/select all: without it ⌘C/⌘V do not work in the web
  view's text fields!), Window (minimize, close ⌘W). Wails' default menu provides these; we set it rather than rely on it. **The Edit menu is not optional.**
- **Close ≠ quit.** Closing the last window must not quit the app (`ApplicationShouldTerminateAfterLastWindowClosed: false`); the launcher lives in the menu bar.
- The panel never shows in Mission Control (`IgnoresCycle`, `Stationary`), and it is above the notch/menu bar region only as far as the work area allows.

### 5.15 The pages

The Svelte pages are shared. Their Windows wording and key handling are corrected in one place each:

| Where | Change |
|---|---|
| `keycaps.ts` | Maps `Super`→⌘, `Alt`→⌥, `Shift`→⇧, `Ctrl`→⌃ and the key names (`Space`, `Return`, `Esc`, arrows) to glyphs when the platform is macOS. Platform comes from the Go side once (`runtime.GOOS` through a service), not from `navigator`. |
| `accelerator.ts` | Records `metaKey` as `Super` (it does), says "Hold ⌘, ⌥, ⇧ or ⌃ as well." on Mac; macOS Option changes `e.key` ("å"), so recording uses `e.code` (it must be checked that it already does). |
| `keys.ts` actions | In-page shortcuts that use `Ctrl` (`Ctrl+Enter`, `Ctrl+C` copy path ...) use `⌘` on Mac. One helper `mod(e)` = `e.metaKey` on Mac, `e.ctrlKey` elsewhere; the hint text is generated from the same table, so a hint cannot disagree with a key. |
| Copy ("Explorer", "Start menu", "taskbar", "tray", "Windows key") | A short `os` dictionary: Explorer→Finder, taskbar/tray→menu bar, Start menu→Applications. Where a sentence only makes sense on Windows (SmartScreen, Alt+Space note), it is shown only there. |
| Setup guide "Search from anywhere" | Shows ⌥⇧Space as caps; the permission cards (8) are new there. |

## 6. Packaging and distribution

### 6.1 What a release contains

`omni-X.Y.Z-macos-arm64.zip` (holding `Omni.app`) and `install.sh`, beside the two Windows files, on the same release. The `Release` workflow gets a `macos` job on
`macos-latest` that runs a new `scripts/release-mac.sh`; the Windows job is untouched.

`Omni.app/Contents/`: `MacOS/omni` (the Go binary, `-tags production`, `-trimpath`, same `-ldflags` and `-X main.version`), `Info.plist`, `Resources/omni.icns`.
`Info.plist`: `CFBundleIdentifier` (`io.github.adityuhh11.omni`; the reverse-domain id is the permission identity, so it never changes), `CFBundleShortVersionString` and `CFBundleVersion`
from the release number, `LSMinimumSystemVersion` 13.0, `LSUIElement` true (no Dock icon at launch; matches `Accessory`), `NSHighResolutionCapable` true,
and the usage strings macOS shows in its permission prompts (`NSDesktopFolderUsageDescription`, `NSDocumentsFolderUsageDescription`, `NSDownloadsFolderUsageDescription`, `NSAppleEventsUsageDescription` for the Trash).
`omni.icns` is built from the existing `omni.ico` source art with `iconutil` on the runner. `NSPrincipalClass` and the webview entitlements are Wails' defaults.

### 6.2 Signing without paying (decision in ADR 0010)

Apple Silicon will not run a binary without a signature at all; an *ad-hoc* signature (`codesign --force --deep --sign -`) is free and satisfies the
CPU but gives Gatekeeper nothing to trust. So an unsigned-by-Apple download is always "from an unidentified developer". That is a first-launch hurdle, not a block (section 6.3).

The subtle part is **Accessibility** (needed to paste with ⌘V, 5.3). macOS ties a permission to the app's code identity. An ad-hoc signed app's identity is the hash of
its exact bytes, so *every update is a different app* and macOS silently drops the grant: the toggle still shows as on in System Settings but does nothing
(reported by developers, links in section 12). A **stable self-signed code-signing certificate**, created once, kept as a repository secret and used by every release, gives
the app an identity that survives updates, at no cost. It buys nothing for Gatekeeper (the Open Anyway step remains). Plan: M4 builds with an ad-hoc signature; M5 introduces the self-signed identity and the friend
checks, over two builds, that the Accessibility grant survives the update. If it does not, the fallback is already in 5.3 (copy, then "press ⌘V") and the grant is re-requested after each update with
an explanation. **(verify with the friend; it is the one claim here that no document settles).**

### 6.3 How a Mac user installs it

| Path | Steps | Warning |
|---|---|---|
| `install.sh` (recommended) | Paste `curl -fsSL https://github.com/Adityuhh11/Omni/releases/latest/download/install.sh \| sh` into Terminal. It resolves the latest release, downloads the zip, **checks its SHA-256 against GitHub's published digest** (the same rule the in-app update uses), unpacks to `~/Applications`, and opens it. | None. A file fetched by `curl` is not quarantined, and Gatekeeper only inspects quarantined files. (Verify on the friend's Mac.) |
| Download the zip in a browser | Unzip, drag `Omni.app` to Applications, double-click → "cannot be opened". System Settings → Privacy & Security → scroll down → **Open Anyway** → confirm with a password. Or `xattr -dr com.apple.quarantine /Applications/Omni.app`. | Yes, once. Since macOS 15 the old right-click → Open shortcut is gone. |
| Build from source | `scripts/build-mac.sh` | None. |

`docs/INSTALL-MAC.md` shows both, with the real wording of each macOS dialog as the friend sees it. The README's install section gets a Mac block. The script is short, readable, and printed by
`curl -fsSL …/install.sh` before anyone runs it: it is a security surface, so it does nothing but download, verify, unpack and open (no `sudo`, no other edits), and it is part of the
attested release.

### 6.4 Provenance

The same `actions/attest-build-provenance` step covers the zip; `gh attestation verify omni-X.Y.Z-macos-arm64.zip --repo Adityuhh11/Omni` works for Mac users who care.

## 7. Build and CI

- `scripts/build-mac.sh` mirrors `build.ps1`: strip `__pycache__`, `npm install`, `wails3 generate bindings`, `npm run build`, `go build` (with cgo; needs Xcode command-line tools, present on the runners), assemble the bundle, ad-hoc sign, verify with
  `codesign --verify --deep --strict`. `scripts/release-mac.sh` adds the zip and the checksum.
- `ci.yml` gets a `launcher-macos` job (every PR, `macos-latest`): the build above, `go vet ./...`, `go test ./...`, page tests, a bundle check
  (`plutil -lint`, `codesign --verify`, the executable is arm64: `lipo -archs`), and a **launch smoke test**: start the app with a throwaway profile, wait for the log line "omni starting", confirm the process
  stays alive for 5 s and exits cleanly on SIGTERM. A runner has a GUI session but no GPU, so hotkey latency is not measured there.
- `GOOS=darwin go vet` from Windows is *not* part of the plan; cgo makes it fail. The `macos-latest` job is the gate.
- Real-hardware tests follow the repository pattern: the `*_real_test.go` files are switched on by an env var; Mac ones are `*_real_darwin_test.go`.
- Windows CI is unchanged and still gates the PR.

## 8. First-run permissions (a designed screen each, in the guide and in Settings → About)

| Permission | Why | When asked | If refused |
|---|---|---|---|
| Accessibility | Paste into the app you came from (⌘V), clipboard history "paste" | Lazily: the first time someone pastes, a card explains it and opens Settings; the guide offers it early. | Everything works except typing for you: Omni copies and says "press ⌘V" (the Windows `ErrNoTarget` experience). |
| Files and Folders (Desktop, Documents, Downloads, removable volumes) | Indexing those folders | macOS asks once per folder the first time Omni (or the engine) reads it. The guide adds folders **one at a time and reads one file from each as it is added**, so each prompt appears with its folder's name next to it rather than a burst during indexing. | That folder is skipped and shown as "needs permission" with a button to the right Settings pane. |
| Full Disk Access (optional) | Whole-disk indexing, `~/Library`, mail | Never asked automatically; offered as an option for the person who wants everything searched. | Nothing changes. |
| Automation → Finder | Empty Trash action | First use of the action. | The action says why it did nothing. |
| Login Items | Start at login | Switching the setting on; macOS also posts a notification. | The setting shows "switched off in System Settings". |

The engine and its llama-server are children of Omni, so macOS attributes their file reads to Omni's identity ("responsible process"), and a grant to Omni covers them *(verify with the friend: index a folder under Documents and see only one prompt)*.

## 9. What is tested, and where

| Layer | Where it runs | How |
|---|---|---|
| Logic (ranking, store, placement, debounce, paste order, pin parsing, plist-to-app, tar.gz unpack with modes, reaper protocol, accelerator mapping) | Windows CI and Mac CI | Plain Go tests with fakes. New tests for each new seam: a table test per mapping. |
| Objective-C leaves (panel ops, pasteboard, icons, FSEvents, Accessibility) | Mac CI only | `go test` with a build tag that runs only on darwin, using real AppKit calls that need no window server (pasteboard, `NSWorkspace.icon`, FSEvents on a temp dir). |
| Bundle, signature, launch | Mac CI | the smoke test in 7. |
| Feel: hotkey latency, focus, full-screen apps, Spaces, multi-monitor, Retina, paste into real apps | The friend's Mac | The checklist below. |

## 10. The friend's checklist (sent with the M5 build; a shorter one after M1)

1. Install by `install.sh` (note any dialog macOS shows, with a photo). Repeat on a second Mac or user account if possible, with the browser download path.
2. Press the hotkey in: Finder, Safari, a full-screen app, a second Space, a second monitor. The bar appears each time, on the screen with the mouse, and Escape returns him to where he was.
3. Type an app name: result, Enter: it opens or is raised. Check Slack/Chrome with several windows.
4. Copy three things in different apps, open the clipboard mode, paste each back into TextEdit, Chrome, Terminal, and a password field (expecting the "press ⌘V" message).
5. Snippets, notes, timer, chat window: open, ⌘C/⌘V in text fields, ⌘W, ⌘Q, ⌘Tab.
6. File search: turn it on, add Documents, Desktop and an external drive; see the permission prompts; search; open a result; reveal in Finder; add a file and see it found within seconds.
7. Restart the Mac, confirm start at login; quit from the menu bar, confirm no `llama-server`/`python` left in Activity Monitor; `kill -9` Omni from Terminal, confirm the same (the reaper).
8. (Deferred with the in-app update.) Delete the app, download the next release, and see whether Accessibility has to be granted again.
9. Run the Mac panel benchmark and send its output file.
10. Light and dark mode; 8 GB RAM behavior with chat and file search loaded together.

## 11. Milestones and exit criteria

| | Scope | Done when |
|---|---|---|
| **M0 Compiles** | `config.DataDir`, per-OS files for every seam in section 4 that is just a path or a name; honest "not available on this system" errors where nothing is ported yet; the `launcher-macos` CI job; pure-Go parts with tests. Nothing visible yet. | `go build`, `go vet`, `go test` green on macOS CI. The Windows build and tests are byte-identical in behaviour. |
| **M1 Core launcher** | Panel (5.1) with the cloak benchmark, hotkey, app list/launch/focus/icons, tray, Dock policy and menu, web/browsers, files open/reveal, system actions, battery, settings and notes and timer windows. | The friend runs the short checklist (items 2, 3, 5, 10 minus engine) and the benchmark meets or explains its number. |
| **M2 Clipboard** | Pasteboard source, private types, Accessibility flow, snippets and paste. | Checklist item 4. |
| **M3 Engine** | `.tar.gz` unpack, Mac pins, layout, Metal device handling, reaper, watcher (FSEvents), permission cards for folders. File search and Omni AI install and run. | Items 6 and 7 (engine part). |
| **M4 Packaging** | `Omni.app`, icon, plist, ad-hoc signature, zip, `install.sh`, release workflow job, `INSTALL-MAC.md`, README. | A tagged pre-release produces the files; item 1. |
| **M5 Login and polish** | `SMAppService`, the update notice (release page, no installer; section 0), final pass on copy and keys. The stable self-signed identity is decided later. | Item 7; the friend gets the full checklist and this is the build that goes to him. |

Each milestone merges to `dev` by its own small PRs (a panel PR, a clipboard PR ...), each with its tests and the doc updates, per AGENTS.md. A plan file
(`docs/superpowers/plans/2026-10-29-macos.md`) breaks M0 and M1 into tasks once this spec is approved.

## 12. Risks and open questions

| Risk | Handling |
|---|---|
| I write cgo I cannot run. | The CI loop (7) compiles and smoke-tests every push; each cgo file is small and does one thing; Windows-testable logic stays out of cgo. Expect several red CI runs on M1. |
| The panel cloak (5.1) turns out slow or blank on first frame. | Three candidates and a measurement are specified; the benchmark decides. |
| Accessibility grant is lost on each update (6.2). | Self-signed identity (M5); the fallback is the copy-and-tell path. |
| `lock` has no reliable no-sudo command on current macOS (5.12). | Drop the action on Mac rather than ship a broken one. |
| Wails v3 is beta (`beta.29`) and its Mac panel class is recent. | The cgo we add never depends on Wails internals beyond `NativeWindow()`; if the panel class misbehaves, the fallback is our own `NSPanel` subclass in cgo, which is more work but contained. |
| macOS 26 (Tahoe) specifics: Liquid Glass window chrome, new notifications permission. | The panel draws its own surface on a clear window, so nothing is inherited; checked by the friend's screenshots. |
| 8 GB Macs running embedding + chat + the browser. | Measured in item 10; the chat's idle unload (`UnloadAfterMinutes`) already exists. |
| Memory attribution of Python/llama children for file prompts (8). | Verified in M3 with the friend. |

Open questions for the owner: (1) is a stable bundle id `io.github.adityuhh11.omni` fine, since it is permanent once people grant permissions? (2) should the friend get a smoke build after M1 (recommended), or only at M5?
(3) do you want the repository to publish `install.sh`, or only the zip until a Mac-using stranger exists?

## 13. Docs touched by this work

`docs/STATUS.md` (phase M), `docs/ARCHITECTURE.md` (the per-OS file convention and the reaper), `docs/DISTRIBUTION.md` (macOS section), `docs/RELEASING.md` (the Mac files, the Mac smoke test), new `docs/INSTALL-MAC.md`, `README.md`
(requirements and the table of network calls: unchanged, the same hosts), `CONTRIBUTING.md` (building on a Mac), `AGENTS.md` ("Windows 11 only" becomes "Windows 11 and macOS on Apple Silicon" when M5 lands), `docs/decisions/0010`.

## Sources

- Apple DTS on overlay windows above full-screen apps (accessory policy plus a non-activating panel): https://developer.apple.com/forums/thread/826308
- Simulating panels, Spotlight versus Alfred window styles: https://github.com/mitchchn/FunWithPanels
- Gatekeeper, quarantine, and why `curl` downloads are not inspected: https://developer.apple.com/forums/thread/666452, https://unit42.paloaltonetworks.com/gatekeeper-bypass-macos
- "Open Anyway" and the removal of right-click Open: https://macos-tahoe.com/blog/app-is-damaged-cant-be-opened-macos-tahoe-gatekeeper-quarantine-fix-2026/
- Ad-hoc signed apps lose Accessibility on every update; a stable certificate keeps it: https://peep14.mataroa.blog/blog/langcheck-on-macos-fixing-accessibility-permission-that-wouldnt-stick/, https://git.bdeshi.space/bdeshi/shannoncoat/src/tag/0.0.8
- Homebrew ending support for casks that fail Gatekeeper (1 Sept 2026): https://brew.sh/2025/11/12/homebrew-5.0.0/
- llama.cpp's release workflow (macOS arm64 build with Metal): https://github.com/ggml-org/llama.cpp/blob/master/.github/workflows/release.yml
- Pasteboard privacy types: https://nspasteboard.org
- Wails v3 `beta.29` source (`pkg/application/*_darwin.go`), read locally.
