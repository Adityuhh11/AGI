# The hotkey path with apps in the bar (2026-10-17)

`cmd/panelbench -real` (the launcher's real page, hotkey to a focused, painted input, n=49 per run), the page before and after
adding apps, the modes and the action panel (block F), alternating runs on the same machine:

| Run | Before (p50 / p95 / max) | After (p50 / p95 / max) |
|---|---|---|
| 1 | 53.1 / 67.8 / 77.7 ms | 53.4 / 71.5 / 83.4 ms |
| 2 | 53.8 / 63.7 / 93.0 ms | 53.7 / 64.6 / 68.8 ms |
| 3 | 53.2 / 64.6 / 72.1 ms | 52.9 / 70.7 / 74.0 ms |

No difference: the new work happens after the first keystroke, never before the panel paints. The limit is 100 ms.

The earlier figure of 41 ms (p50 39, p95 46 on 2026-10-15) is not comparable with these: the same measurement run today
gives 53 ms even for the page as it was, so the machine is slower now (Windows, a browser with 28 processes and the
engine running), not the page. The before-and-after comparison is what this table is for.
