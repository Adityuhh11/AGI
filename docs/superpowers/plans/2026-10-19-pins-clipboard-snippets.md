# Pins, clipboard history and snippets: plan

Spec: `docs/superpowers/specs/2026-10-19-pins-clipboard-snippets-design.md` (approved by the owner on 2026-10-19, with its
seven decisions). Branch `feat/pins-clipboard`, cut from `dev`. One commit per task, docs in the same commit as the code
they describe, `go vet`, `go test ./...`, `svelte-check` and `npm test` green before each commit. No AI attribution.

P0 is written out; P1 to P3 are outlined and each starts with the spike the spec asks for, so they are detailed when the
spike has answered.

## P0: Pins

**P0.1 `internal/state`: pins.** `file.Pins []string` (`json:"pins"`), `Pins()` (a copy, in the order pinned),
`TogglePin(id) (pinned bool, err error)`. At most `maxPins = 8`: pinning a ninth is an error that says so. A missing field
loads as no pins; the file version stays 1. Tests: toggle on and off, order kept, survives a restart, the ninth is
refused and the first eight stay, an old file without the field loads.

**P0.2 `internal/palette`: pins as rows.** `Pinned() []Item` (resolved through `itemFor`, which learns quicklinks:
`quicklink:<trigger>:` resolves through `Browser.Find` to a row with an empty text; rows whose target is gone are left
out), `PinIDs() []string`, `TogglePin(id) (bool, error)` which accepts `app:`, `file:` and `quicklink:` ids only and cuts the
text off a quicklink id (a pin is the trigger, not what was typed after it). Tests: each kind pins and resolves, a gone app
and a deleted file are dropped from `Pinned` but stay pinned, a quicklink pinned with text keeps only the trigger, a web
row or a tag is refused.

**P0.3 Page: the empty state.** `flatten(rows, expanded, entries, keepOrder)`: with `keepOrder` the entries stay in the order
given (pins must not be regrouped as quicklinks first). `pinIdOf(item)` in `flatten.ts` (app, file, file result, quicklink).
In `App.svelte`: `pins` (the set of pinned ids) and `pinned` (the resolved rows) loaded at start and after every toggle; when
the text is empty `entries` are the pinned rows; `onBlur` clears the text and the results, so the next opening starts from the
pins; a "Pinned" section label; `Ctrl+P` toggles the pin of the selected row; "Pin" / "Unpin" in the `Ctrl+K` panel
(`actionsFor` takes `{taggable, pinnable, pinned}`); a pin mark on a pinned row (`Icon` gains `pin`); Enter on a pinned
quicklink puts `<trigger> ` in the input; the footer hint shows `Ctrl+P pin` for rows that can be pinned. Tests (node): the
order is kept, `pinIdOf`, the actions with and without pin, and Escape behaviour unchanged.

**P0.4 Measure.** `cmd/panelbench -real` with eight pinned rows in the state the page reads: p50 and p95 against today's
36.2 / 42.4 ms (limit for this feature: p50 45 ms). If it is over, the fallback in the spec (pinned rows a frame after the
capsule) is tried and measured. The numbers go in `docs/eval/2026-10-19-pins.md`.

**P0.5 Docs.** `docs/STATUS.md` row 9b; the README's "what to try" gains a line on pins.

## P1: Clipboard (outline)

1. **Spike first** (`docs/eval/2026-10-19-paste.md`): put text on the clipboard, give the keyboard back to the previous
   window and send `Ctrl+V` from Go, into Notepad, VS Code, Zen, Chrome, Word, Windows Terminal and an elevated Notepad;
   how long to wait for the window to be foreground; what an elevated target does. The result decides `Paste` before
   anything is built around it.
2. `internal/clipboard`: `Source` interface (sequence number, text, formats, owner program; real one in `_windows.go`, a
   fake for tests), the privacy filters as a table test, `Store` (the JSON file in `%LOCALAPPDATA%\AGI`, limits, dedupe,
   search by whole words), `Watch` (4 Hz, plus on opening), `Paste`.
3. `config`: `[clipboard] enabled`, `ignore`; the settings switch, the ignore list and "Clear history"; the
   `cmd:clipboard` row and the `clip:` mode in `palette`; the page's Clipboard mode, rows and actions.
4. Measure: 1000 entries searched in under 2 ms, idle cost of the watcher, a real marked copy honoured.

## P2: Snippets (outline)

`config.Snippet` with validation, `internal/snippets.Expand` (`{date}`, `{time}`, `{clipboard}`, `{{`), the `snip:` mode and
the keyword row in All mode, the Settings page cloned from Quicklinks, the `cmd:snippets` row, the one-time seeding of pins
with `cmd:clipboard` and `cmd:snippets` (`state.SeedPins`, only when the file has never had a `pins` field).

## P3: Hotkey and release (outline)

`[clipboard] hotkey`, registered and rebound with the main one, `Panel.Show(mode)` and the page event; the README says what is
kept and where; `release.ps1 -Version 0.3.0`.
