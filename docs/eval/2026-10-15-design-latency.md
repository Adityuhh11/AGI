# Design pass: hotkey latency and the cloak

Date: 2026-10-15. Harness: `cmd/panelbench` (the real `internal/panel`, another app holding the keyboard, 50 hotkey presses,
`keybd_event`, a wait until the page reports focus and a painted frame, then `Hide`). The first run of each series warms the
web view and is dropped (n = 49). Limit: 100 ms at p95.

| page in the panel | input focused p50 / p95 / max | painted p50 / p95 / max |
|---|---|---|
| plain black bar, opaque window (launcher block D) | about 46 / 52-66 / 64-68 ms | not recorded here |
| plain bar, **transparent** window | 31.8 / 38.9 / 40.8 ms | 32.2 / 40.0 / 40.8 ms |
| capsule + animated WebGL ring (stand-in page), run 1 | 32.2 / 41.8 / 45.1 ms | 32.5 / 41.8 / 45.1 ms |
| capsule + animated WebGL ring (stand-in page), run 2 | 31.9 / 39.5 / 48.7 ms | 32.3 / 39.5 / 48.7 ms |
| **the real designed page**, run 1 | 39.1 / 45.0 / 55.4 ms | 40.2 / 45.5 / 55.4 ms |
| **the real designed page**, run 2 | 38.6 / 45.1 / 45.8 ms | 39.5 / 46.1 / 46.5 ms |

No misbehaving run in any series (keyboard returned, size and place sane). A transparent window is not slower than an opaque
one; the shader on the page costs nothing on the hotkey path because it is already running (cloaked, not hidden) and is
paused only while the panel is closed. `go run ./cmd/panelbench -real` measures the page in `frontend/dist`; `-pill` measures
the stand-in; `-hold 12s` keeps the panel open after the first run so it can be looked at.

## Correction, 2026-10-16: the ring was not pausing

The ring was told its speed but never that the panel had closed, so it kept animating, invisibly, while cloaked: **the launcher's
web view used 35% of a CPU core with the panel closed** (8 s sample, nothing else running). The ring now takes a `running` flag
(false when the panel loses the keyboard) and stops the shader outright: **0.0% of a core** closed. Its speed also eases (350 ms)
instead of jumping between 0.35 and 1.1 on every keystroke, which looked jerky once searches took 20 ms. The page gets frames
at 144 per second with a longest gap of 7 ms while the ring runs, so frame pacing was never the problem. The ring restarting on
each opening costs about 5 ms: hotkey to focused input on the real page is now p50 44 ms, p95 53 ms, max 55 ms (two runs of 50).

## Why the cloak stays

Earlier numbers (launcher experiments): Wails `Show()`/`Hide()` median 72 ms, max 96 ms; cloak 41 ms, max 58 ms. A hide also
stops WebView2 rendering, so the first frame after the hotkey is late, and the animated ring would start from nothing. With the
cloak the page keeps its last frame and uncloaking shows it at once. The price is that the page runs while invisible, which
the pause-on-blur rule removes (ring `setSpeed(0)`, CSS `animation-play-state: paused`).

## Seen in screenshots (real app, real index)

Capsule, ring, loader chip, results card, thumbnails, expanded matches, snippets with matched words emphasised, one-row footer:
all render as designed at 125% scaling. Bugs found by looking and fixed: whole-word highlighting (marked "and" inside "Random"),
footer hints wrapping, folder path truncated at the wrong end, an illegible PDF icon, a "Nothing matched" flash for an empty
query while typing, teal fringes on the ring (dispersion lowered), boxes where PDF text had private-use characters.

## Found about search while here (not fixed)

- Warm search over the full index (56,252 chunks): p50 203 ms, p95 261 ms through HTTP including the query embedding. **Wrong:** measured while the live engine was indexing on the same GPU; see `2026-10-16-query-latency.md` (76 ms then, 16 ms now).
- A gibberish query ("qzxvbnmw") returns ten results flagged confident; the 0.65 cosine threshold was tuned on the 431-chunk
  sample folder. DigUp's per-query "stands out" gate is the candidate fix.
