# Pins, clipboard history and snippets

Date: 2026-10-19. Status: draft, waiting for the owner's approval. Branch `feat/pins-clipboard`, release target v0.3.0.
Builds on the palette (`2026-10-17-palette-design.md`, where clipboard was P5 and snippets "later") and the packaging
work (`2026-10-18-packaging-design.md`: settings window, `config.toml` through `internal/settings`).

## Goal

The bar becomes the place you also reach for what you copied and what you type over and over, and the things you use
most are there the moment it opens.

- **Pins.** Anything the bar can show can be pinned. With nothing typed, the bar lists the pinned things, so the hotkey
  alone is already a launcher.
- **Clipboard history.** What you copied, searchable, and Enter pastes it into the app you came from.
- **Snippets.** Text you keep (an address, a reply, a SQL fragment) with a name and an optional keyword, pasted the
  same way, with `{date}`, `{time}` and `{clipboard}` filled in.

It stays inside the rules: the hotkey to painted input stays under 100 ms (today 34 to 39 ms), nothing leaves the machine,
no new dependency, and a capability is a package and a few lines of config, not a plugin system.

## Decisions I made, for the owner to change

| # | Decision | Why |
|---|---|---|
| 1 | Clipboard history is **off until switched on** in Settings, and says what it keeps | It records what you copy, passwords included if an app does not mark them; the privacy rule makes that a choice |
| 2 | **Enter pastes** into the app you came from; `Ctrl+Enter` only copies | What Raycast does; copying alone leaves you to press Ctrl+V yourself |
| 3 | `Tab` still cycles All, Apps, Files, Web, Tags. **Clipboard and Snippets are reached by prefix (`clip:`, `snip:`), by their pinned rows, and by an optional hotkey** | Seven chips to cycle through is too many |
| 4 | **Text only** in the clipboard history for now | Images need storage, thumbnails and a size policy; text is what is searched for |
| 5 | **No expansion as you type** (typing `;addr` anywhere) | It needs a global keyboard hook: antivirus flags those, and it would have to be written again for macOS |
| 6 | On upgrade the pins are **seeded once** with Clipboard history and Snippets; they can be unpinned | Otherwise a new feature is invisible |
| 7 | Clipboard search is **whole words, anywhere in the text**, newest first, not fuzzy | Measured below: the fuzzy matcher is made for app names and is 9.6 ms over 1000 entries |

## Measured on this machine (2026-10-19)

Throwaway benchmarks (not committed). A thousand copied texts, written the way `state.json` is written (the whole file to
a temporary one, then renamed):

| History | Size | Rewrite on each copy | Search by whole word (all entries) |
|---|---|---|---|
| 1000 entries of 300 characters | 0.4 MB | 1.5 ms (p95 2.7) | 12 µs |
| 1000 entries of 2,000 characters | 2.4 MB | 3.3 ms (p95 4.5) | 14 µs |
| 500 entries of 20,000 characters | 14 MB | 12 ms (p95 19) | 7 µs |

Loading the file at start takes 52 to 61 ms, off the hotkey path. `rank.Match` over 1000 texts of 200 characters takes
9.6 ms, against 0.23 ms for the 176 apps it was built for. So: **one JSON file rewritten per copy is enough (no database),
and clipboard rows are matched by whole words, not by the fuzzy matcher.** The rewrite happens on a background goroutine,
so the UI never waits on it.

Still to measure while building (acceptance below): the opening with pinned rows, paste into real applications, and the
idle cost of watching the clipboard.

## Pins

A pin is an id in an ordered list in `state.json` (`pins`, next to `tags`; the file's version stays 1 and an old file just
has no pins). Ids are the ones rows already have: `app:`, `file:`, `quicklink:<trigger>:`, and the new `cmd:clipboard`,
`cmd:snippets`, `snippet:<name>`.

- **Pin and unpin:** `Ctrl+P` on the selected row, or "Pin" / "Unpin" in the `Ctrl+K` panel. A pinned row shows a small pin mark.
- **Empty bar:** the pinned rows, in the order pinned, up to eight, below the input. Typing replaces them with results;
  clearing the text brings them back. Rows whose target is gone (an uninstalled app, a deleted file) are left out, as tags do.
- **What Enter does on a pinned row:** what it does anywhere else. A pinned quicklink puts its trigger in the input
  (`gh `) so you can type the rest; `cmd:clipboard` and `cmd:snippets` switch the bar to that mode instead of closing it.
- **The first paint.** The page tells Go its height after each render, and Go opens the window at the last height it was
  told. Closing the bar returns the page to the empty state, so the next opening already has the pinned rows painted. The
  acceptance below measures that against today's 34 to 39 ms.
- No reordering in v1 (unpin and pin again); added when it is missed.

## Clipboard history

**Watching.** `GetClipboardSequenceNumber` is a counter that changes whenever anything is copied; the package reads it four
times a second, and once more when the bar opens, and reads the text only when it changed. No window, no message loop, no
hook, and the idle cost is a few microseconds a second. (The alternative, `AddClipboardFormatListener`, pushes a message
but needs a hidden window and its own message loop for the same result.)

**What is not recorded** (each a table test):

- an entry the copying app marked as private: the formats `ExcludeClipboardContentFromMonitorProcessing`,
  `CanIncludeInClipboardHistory` set to 0, and `CanUploadToCloudClipboard` set to 0 (password managers set these);
- anything copied by a program named in `[clipboard] ignore = [...]` (the owner's exe names, with a default list of
  well-known password managers: 1Password, Bitwarden, KeePass and KeePassXC, Dashlane, LastPass, Enpass);
- text that is only whitespace, and text longer than 100,000 characters (skipped and logged, never cut);
- what the launcher puts on the clipboard itself when it pastes (the same text moves to the top instead of being added twice).

**Keeping.** Newest first, the same text kept once, at most 500 entries, 30 days and 20 MB in all; the oldest go first.
The file is `%LOCALAPPDATA%\AGI\clipboard.json` (this computer only, never in the roaming `config.toml` folder), written
atomically. Settings: the switch, the ignore list, "Clear history", and a line saying where the file is.

**In the bar.** Mode `clip` (chip "Clipboard", prefix `clip:`): with nothing typed, the newest 20; typing keeps the entries that
contain every word. A row is the first line (up to about 100 characters), and under it where it was copied from, how long
ago and how long it is. Actions: **Paste** (Enter), **Copy** (`Ctrl+Enter`), **Delete this entry** (in `Ctrl+K`).

**Paste** is: put the text on the clipboard, close the bar and give the keyboard back to the app the panel remembers (it
already does that on `Esc`), wait for that window to be foreground, send `Ctrl+V`. Known limits, to be tested and then
written in the README: Windows does not let a normal program send keys to an elevated window, so for one the text is
copied and the bar says "Copied; press Ctrl+V, this window runs as administrator"; a few programs paste with another key.
The test list: Notepad, VS Code, Zen, Chrome, Word, Windows Terminal, an elevated Notepad.

## Snippets

Kept in `config.toml`, like quicklinks (a file you edit, and a settings page for people who would rather not):

```toml
[[snippet]]
name = "Home address"
keyword = "addr"          # optional: typing it exactly in the bar shows the snippet first
text = "12 Example Street\nSpringfield"
```

`{date}` (2026-10-19), `{time}` (14:05) and `{clipboard}` (what is on the clipboard now) are replaced when it is pasted; `{{`
writes a brace. No cursor placeholder yet: it would need the program to send arrow keys afterwards.

**In the bar.** Mode `snippets` (prefix `snip:`): the names, matched with the same matcher as app names (they are short),
a preview of the text under each. In All mode only an exact keyword shows a snippet, like a quicklink trigger, so snippets
do not crowd the results. Enter pastes, with the same paste as the clipboard. Settings gets a Snippets page built like
Quicklinks (add, edit, remove, validated: a name, text, a keyword that is one word and not a quicklink's trigger).

## An optional hotkey for the clipboard

`[clipboard] hotkey = "Ctrl+Shift+V"` (empty by default) opens the bar already in the Clipboard mode, for the people who
use Win+V. It is one more `RegisterHotKey`, registered and rebound through the same code as the main hotkey, and the panel
asks the page for the mode with an event. Last in the order of work because the in-bar route (pinned row, `clip:`) already
gets there in two keys.

## Where it lives

| Package | Job |
|---|---|
| `internal/clipboard` | `Watch` (the sequence number, the read, the privacy filters), `Store` (the JSON file, the limits, search), `Paste` (put, focus, send keys); Windows parts in `_windows.go` files |
| `internal/snippets` | `Expand(text, now, clipboard)` and the validation of the list; the list itself is `config.Snippet` |
| `internal/state` | gains `pins` and its few methods, tested like tags |
| `internal/palette` | the `clip:`, `snip:` modes, the new row kinds (`clip`, `snippet`, `cmd`), `Pin`, and `Do` for the new ids |
| `internal/panel` | `HideForPaste` (give the keyboard back and say when the window is ready), later `Show(mode)` |
| Frontend | the empty-state list, the pin mark, `Ctrl+P`, the two modes, the Settings pages (Clipboard switch, Snippets) |

## Order of work and acceptance

| Phase | Content | Done when |
|---|---|---|
| P0 Pins | `state` pins, `Ctrl+P`, the empty-state list, the pin mark, seeding | pin an app, a file and a quicklink; they are in the first paint; hotkey to painted p50 not above 45 ms (`cmd/panelbench`) with eight pinned rows |
| P1 Clipboard | package, filters, store, Clipboard mode, paste, settings switch and clear | the filter table tests pass; a copy shows in the bar within half a second; Enter pastes into the seven test programs (or says why not); 1000 entries search in under 2 ms; idle cost measured and below 0.1% of a core; password-manager flag honoured in a real test with a marked copy |
| P2 Snippets | `config.Snippet`, expansion, the mode, the Settings page, `cmd:snippets` pinned | placeholders table test; a snippet pasted with `{clipboard}` filled in; keyword in All mode |
| P3 Hotkey, release | the optional clipboard hotkey; the README says what is kept and where; v0.3.0 | the hotkey opens in Clipboard mode in under 100 ms; release notes mention the opt-in |

## Risks and open questions

- **Paste into other programs** is the least certain part: focus handover timing and elevated windows. P1 starts with a
  spike on the seven programs, before the Clipboard mode is built around it.
- **Clipboard privacy** is a promise. Beyond the filters, history is off by default, the file stays on this computer, and a
  "pause" for the watcher is the same switch. Text copied from a website or terminal can still hold a secret the filters
  cannot know; the Settings text says so plainly.
- **Pins in the first paint** could cost the bar its speed. If eight rows push the p50 over 45 ms, the fallback is to show
  the pinned rows a frame after the capsule, which is measured too.
- **Seeding the pins** assumes a feature the user has not enabled is still worth a row; the Clipboard row, when history is
  off, says so and leads to the switch.
