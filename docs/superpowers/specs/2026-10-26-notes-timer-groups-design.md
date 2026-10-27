# Notes, a timer, grouped file results, and a feel for the arrow keys

Date: 2026-10-26. Status: built on `feat/notes-timer-groups`, for the owner's review.

The owner asked for four things and for more of the same kind. This spec fixes what each does, where it lives, what is
configurable, and what is left out. The hotkey path is untouched: nothing here runs when the bar opens.

## 1. Selection feel (the glass fade)

While a person moves through the results with the arrow keys, the selected row becomes a lit pane (a brighter fill, a lit
edge and a shadow under it) and the text of the others fades to about half. A second after the last arrow key, or at once when
the pointer moves, everything settles back. The list glides to the row. It is only a state of the list (`keying`), set by the
page on ArrowUp or ArrowDown and cleared by a timer, so a held key keeps it on.

First version moved the row (`translateY` and `scale(1.008)`) and faded the others with `opacity`. Measured on a 125% display
that made the selected row's edges 12% softer: a scale or a fractional move makes the browser resample the row's text. So
nothing is moved or scaled, and rows fade by the colour of their text, never by opacity.

- Setting: `[ui] selection_effect` (default on), Settings > General, "Lift the selected result when I use the arrow keys".
- Reduced motion: no smooth scroll; the fade stays (it is not motion).
- Scrolling: our own loop, one step per frame, each closing a fixed share (time constant 70 ms) of the distance to where the
  selected row belongs. CSS `scroll-behavior: smooth` restarts on every key repeat and moved in jerks (12 of 16 frames moved,
  steps of 2 to 21 px); the loop moves on 39 of 42 frames with a smooth ramp. The goal of a glide under way counts as the
  list's position, so a second key aims from the first one's goal.
- Cost: colour and background changes on at most about 12 rows, and one `requestAnimationFrame` loop while the list moves.

## 2. Grouped file results

File rows carry a kind (code, text, pdf, image, video). With grouping on and at least four file rows, the list shows
"Best match" (the single highest-scoring file), then the rest under a heading per kind (Code, Documents, PDFs, Images,
Videos), the kinds ordered by their best file and the files in each by score. Fewer rows stay one list: headings on three
rows are noise. Arrow keys cross headings as if they were not there (headings are not selectable).

- Setting: `[search] group_results` (default on), Settings > File search, "Group files by type".
- Pure frontend: the engine's order and scores are unchanged; `lib/groups.ts` arranges them. Table-tested.

## 3. Notes

Quick markdown notes, kept as plain files so they are the user's own: one `.md` file per note in
`%APPDATA%\Omni\notes\`, the file name a time-based id, the title the first line. Nothing is indexed by the engine and
nothing leaves the machine.

- Package `internal/notes`: `Store` (list, get, save, delete, search; atomic writes; ids cannot escape the folder),
  `Service` bound to the frontend.
- Window: the same host as Settings, route `notes`, 420 x 560, resizable, always on top (a pin button turns that off and
  on). It opens on the list of notes (newest edit first, with a search box) or straight on a note. Editing is a plain
  text area with a preview toggle; the markdown renderer (`lib/markdown.ts`) is ours (headings, bold, italic, code,
  code blocks, quotes, lists, task lists whose boxes can be clicked in the preview, links, rules) and escapes everything
  first, so a note can never run script. Saves 400 ms after the last key, and on closing. Delete asks once.
- The bar: `note` finds the Notes command; `note: words` lists matching notes and, first, "New note"
  (with the words as its first line). Enter opens the note in the window. Tab does not reach it, like Clipboard and
  Snippets.
- Left out: syncing, tags on notes, images, a global hotkey (the bar is the way in; a setting can add one later).

## 4. Timer (and pomodoro)

One timer at a time, kept in Go (`internal/timer`) so it survives its window closing. A small always-on-top window
without a frame sits at the right edge of the screen (draggable) and shows the time left, what it is, and pause, skip
and stop. It opens when a timer starts and closes when it is stopped.

- Starting from the bar: `timer 25` (minutes), `timer 90s`, `timer 1h30`, `timer 10m tea` (a label), `pomodoro` or
  `focus`. While one runs, `timer` also offers Pause or Resume, and Stop.
- Pomodoro: focus, short break, focus, short break, focus, short break, focus, long break. Durations and the number
  of rounds are settings; "start the next one by itself" is a setting (off: a finished period waits for a click).
- Settings: `[timer] focus_minutes` (25), `short_break_minutes` (5), `long_break_minutes` (15), `rounds` (4),
  `auto_next` (false), Settings > Timer. `preset_minutes` (5, 10, 25, 45; at most 8, each 1 to 1440) are the quick countdowns
  that plain `timer` offers; any other length is typed after the word, and the first quick row says so.
- A finished period beeps once (the system sound) and the window pulses until dismissed. No toast: Windows toasts need
  the app registered with the shell, which an unsigned portable program is not.
- The clock is monotonic and injected, so it is tested without waiting.

## 5. More, on top

- **Calculator and unit conversion in the bar** (`internal/calc`): `12*7+3`, `2^10`, `15% of 80`, `sqrt(2)`,
  `5 km in miles`, `72 f in c`, `3 gb in mb`. A row shows the answer; Enter copies it. A hand-written parser (no eval, no
  dependency), table-tested. It only answers when the whole text parses, so ordinary searches are untouched.
- **System commands**: Lock the computer, Sleep, Task Manager, Empty the Recycle Bin, Windows Settings. Typed names
  match like the other commands. Shut down and Restart are left out on purpose: one mistyped Enter should not do that.
- **Settings pages**: Timer (new), and the two switches above.

## What is deliberately not here

A window switcher, currency conversion (needs the network), notes sync, a notes hotkey, timers that outlive the program,
Windows toast notifications. Each is possible later; none is needed to use these.

## Testing

Go: table tests for the notes store, the timer state machine on a fake clock, the calculator and conversions, the bar's
new rows and the config keys. Frontend: node tests for grouping, markdown, timer-time parsing and the selection
state. The windows are checked by building the app and opening them (see the plan).
