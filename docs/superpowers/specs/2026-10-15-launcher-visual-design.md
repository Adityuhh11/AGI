# Launcher visual design: the capsule, the metal ring, the loader

Date: 2026-10-15. Status: built, awaiting the owner's review. Supersedes the "plain black bar" of
`2026-10-15-launcher-design.md` (its Frontend section). The visual system itself is recorded in `/DESIGN.md`; product
context is in `/PRODUCT.md`.

## What the owner asked for

A heavily rounded, near-circular search bar that morphs into a card; a liquid-metal border flowing around it (from the
`liquid-metal` component of vengenceui.com); a text loader with a spinning orb (from opensourceui.in) for waiting states;
better typography. The two components are React/Tailwind; the page is Svelte in WebView2, so they are re-made, not copied.

## Decisions

- **The shader is the open-source one behind that component.** Its prop names (`colorBack`, `colorTint`, `speed`,
  `repetition`, `distortion`) match `@paper-design/shaders` (Apache-2.0, no dependencies), whose `ShaderMount` runs without
  React. One new dependency; it is a WebGL2 fragment shader, so WebView2 runs it as is.
- **The loader is native CSS** (about 30 lines): per-letter breathing and a conic-gradient orb that turns. Nothing copied.
- **Transparent window, drawn by the page.** `BackgroundTypeTransparent` plus `DisableFramelessWindowDecorations`: the page
  draws the capsule, its ring and its shadow; Windows adds no square border. The window is wider and taller than the
  card by the shadow margins.
- **Morph = height only.** Radius is constant (32.5 px = half the bar height), so the capsule becomes a card without a
  shape change. The card animates 240 ms in CSS; the window jumps to the larger of old and new height at once, so
  nothing clips, and shrinks after the motion.
- **The cloak stays** (owner asked why not hide). The page keeps running while closed, so everything that moves pauses on
  blur: the ring (`setSpeed(0)` stops its animation frames entirely) and all CSS animations. See
  `docs/eval/2026-10-15-design-latency.md` for the numbers behind keeping it.
- **Fonts:** Geist and Geist Mono, bundled, no network.
- **No search-quality change.** The page only displays what the engine returns.

## Performance rules (binding)

1. Hotkey to focused, painted input under 100 ms at p95, measured with `go run ./cmd/panelbench -real`.
2. No animation while the panel does not have the keyboard.
3. The ring renders at 1 pixel per CSS pixel; the GPU is shared with the embedding model.

## Out of scope here

Scope chips and filters, the preview pane, the actions menu, apps and commands. The result row and the footer were laid out
to take them (kind tile, location, hints), but none is built.

## Known limits

- The first query after a cold start takes about a second (query model warm-up); after that about 200 ms, so results land
  roughly 320 ms after the last key. The engine call is the bottleneck, not the page.
- The engine's "confident" flag is too generous on the full index (a gibberish query still counts as confident), so
  "No strong match" rarely shows. A search-quality item, not a page one.
- Photos taken sideways on a phone may show rotated in their thumbnail (EXIF orientation is ignored).
- Check on a second monitor with a different scaling, and a screen reader, have not been done.
