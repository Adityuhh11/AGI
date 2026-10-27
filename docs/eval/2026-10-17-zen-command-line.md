# Zen's command line: spike S2 (2026-10-17)

Question: can the launcher open a URL or a search in Zen, in the window that is already open? Zen 1.23.2b (Firefox 157), running
(28 processes), default browser of Windows is Chrome. Checked through Zen's own history (`places.sqlite`, copied with its
WAL), looking only for the test strings.

| Command | Result |
|---|---|
| `zen.exe https://example.com/?omni-spike=url` | a tab loaded in the existing window; the launching process exited in 363 ms, exit code 0; still one Zen window |
| `zen.exe -search "omni"` | DuckDuckGo results for the term, that is **Zen's own search engine**; launcher exited in 88 ms |
| the same two from Go (`exec.Command`) with `-search "omni spike two words & symbols"` and a DuckDuckGo URL | both correct, the `&` and spaces intact; `Start` returned in 6 to 12 ms |

Mistake worth recording: PowerShell's `Start-Process -ArgumentList "-search","a b"` joins the array without quotes, so Zen got
`-search a`, `b` and opened `http://b/`. Go's `exec.Command` quotes each argument properly, so this is a shell quirk, not a Zen one.

Decision for block G: the browser's own `-search` is the default web search (respects the engine chosen in Zen, no
escaping to get wrong); URL templates are for quicklinks and for browsers without `-search`. Opening goes through
`exec.Command(zen, args...)`, started and not waited for. Not tested: Zen not running (a cold start with arguments); the
launcher treats it the same and a browser that starts slowly is the browser's own cost.
