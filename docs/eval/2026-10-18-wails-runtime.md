# Wails v3 beta.29 at run time: a second window, and rebinding the hotkey (spike S1, 2026-10-18)

Throwaway program (deleted): an app with an off-screen window like the panel, then at run time two framed 760 x 560 windows opened on
`/#settings` and closed, then global shortcuts registered and unregistered. Windows 11 (build 26200), WebView2 155.

| Question | Result |
|---|---|
| Open a framed window after the app is running | works; the page was loaded 78 ms and 82 ms after `NewWithOptions` (two runs), a native handle exists at once |
| Close it and open another with a new name | works; each window builds its own WebView2 environment (a log line per window) |
| `GlobalShortcut.Register` while running | works; a duplicate fails with `global shortcut "Alt+Ctrl+F9" is already registered` |
| Register the new accelerator, then `Unregister` the old one | works; `GetAll()` then lists only the new one, so "register first, drop the old after" is safe |
| A combination another program or Windows owns (`Ctrl+Alt+Delete`, `Super+Space`) | `failed to register ... (possibly by another application)`: usable as the message in the settings form |
| Invalid text | `invalid global shortcut "garbage": 'garbage' is not a valid key` |
| Modifier names | `Ctrl`, `Alt`, `Shift`, and `Super` for the Windows key (`Win` is rejected: `'Win' is not a valid modifier`); the capture field must produce these |

Not measured here: the effect of an open settings window on the hotkey-to-input time (block B measures it with `panelbench -real`),
and the native folder picker (it needs a person; block B checks it by hand).
