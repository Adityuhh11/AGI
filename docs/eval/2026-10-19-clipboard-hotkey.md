# The clipboard hotkey, end to end (2026-10-19)

Phase P3 of the pins, clipboard and snippets work: an optional second global hotkey
(`[clipboard] hotkey`) that opens the bar with `clip: ` already typed in. Acceptance: opens in the Clipboard mode in
under 100 ms. Run in a clean profile with the history switched on, a window of another program in front, three copies in
the history, driven by synthetic key events (`keybd_event`), every key guarded to fire only while one of my own windows
was in front.

## Result

| Step | Seen |
|---|---|
| The hotkey (Ctrl+Alt+V in the test) | the bar opens with the keyboard, `clip:` in the input with the caret after it, the three copies listed newest first with their source and age, the "Clipboard" chip |
| Typing `beta` | goes after `clip: `, the list narrows to one row |
| Enter | `beta kubernetes pod yaml` arrives in the other program's window; the bar closes |
| Hotkey to the last paint of the list | **p50 56.8 ms, p95 62.5 ms, max 72.2 ms** (15 runs) |
| For comparison, the main hotkey to the capsule | p50 16.9 ms, p95 19.3 ms (15 runs) |

The time is the last change of a patch of the screen where the first rows appear, polled as fast as the screen can be
copied (a capture takes a few milliseconds, so the figures are good to about that); it includes the page's own round trip to
Go for the list. `cmd/panelbench -real` (hotkey to painted input, which the list does not change) after these changes:
no pins p50 28.0 ms, p95 34.7 ms; eight pinned rows p50 27.9 ms, p95 33.2 ms.

## What the test found (all three fixed before this commit)

1. **The launcher froze at start when the second hotkey was configured.** `RebindFill` was called inside the application's
   started handler, which runs on the main thread, and registering a hotkey waits for the main thread: the launcher waited for
   itself, and neither hotkey worked. Now it is registered from a goroutine, as the panel's own hotkey is (`go p.prepare()`).
2. **The text typed in by the hotkey was wiped.** The window sends a blur event of its own while it opens, and the page took
   every blur for the bar closing and cleared the input (closing is meant to clear it). The page now counts the bar as closed
   only if the keyboard has not come back 120 ms after a blur (`document.hasFocus()`).
3. **The text was selected, so the first keystroke replaced it.** The window gets more than one focus event while it opens
   and the first one used up the flag that kept the text; the page now leaves the text alone for a second after it arrives.

## A limit of the test, not of the feature

With synthetic key events, a hotkey **without Alt** (`Ctrl+Shift+Space`, `Ctrl+Shift+V`) opened the bar but did not give it
the keyboard, for the main hotkey as much as for the clipboard one; the tap of Alt in every working hotkey is the usual way a
program is allowed to take the foreground. The owner's own `Ctrl+K` and `Ctrl+M` worked from his keyboard, so this is
believed to be about injected keys, but it has **not** been checked with a physical keyboard for a hotkey without Alt: if the bar
ever opens without the keyboard, the answer is to ask Windows for the foreground the long way (attach to the foreground
window's input thread) in `Panel.Show`, which this branch does not do.
