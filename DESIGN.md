# Design

The visual system of the launcher panel, recorded from the built page (`frontend/src`) on 2026-10-15. Tokens live in
`frontend/src/theme.css`; the rationale and measurements are in `docs/superpowers/specs/2026-10-15-launcher-visual-design.md`.

## World

Platinum black and chrome. The plate is the black metal card: neutral graphite, lightest at the top left and
nearly black at the far corner, a fine horizontal brushed grain, one soft band of light across it and a lit top rim. It is
opaque (glass was tried and dropped on 2026-10-16: a blurred picture of the screen behind made bright pages turn the plate
grey, and cost 9 to 17 ms to take on every opening). The interface itself has no accent hue (emphasis is
weight and brightness); the only colour is the ring, a metal that changes every time the panel closes.

## Shape and motion

- **Capsule to card.** At rest the bar is a 680 x 65 capsule (radius 32.5 px, a perfect semicircle end). Results grow it
  into a card of the same width and the same corner radius; only the height animates (240 ms, exponential ease-out).
  The window around it is clear and transparent (`BackgroundTypeTransparent`), 20 px wider on each side and 12 px above
  and 40 px below the card, for the shadow. The shadow (`--shadow`) must fit inside those margins (blur minus spread under
  the margin on every side); a larger one is cut off by the window edge as a hard straight line, which showed over white pages.
- **The ring.** Our own WebGL shader (`lib/ringShader.ts`, run by Paper's vanilla `ShaderMount`) draws on the whole card; the
  plate sits 2.5 px inside it, so only the rim shows. The light is laid along the distance travelled
  round the card's outline, not across it as Paper's liquid-metal stripes do (on a thin rim those seemed to come and go from
  different sides): it circulates clockwise only, two fronts on opposite sides keep the whole outline lit, and a slow wobble in
  the phase makes the bands breathe; the colour channels sit a hair apart for coloured fringes. The metal is one of ten in `lib/palettes.ts` (chrome, copper, ember,
  gold, ice, violet, emerald, rose, and two two-colour ones, sunset and aurora, whose highlights are another hue). A new one is
  chosen, never the same twice in a row, at the moment the panel loses the keyboard, so the colour never changes while it is
  looked at. Speed settles at 0.6, or 1.5 while you type or a
  search is in flight and for 0.7 s after; every change eases in over about 350 ms (a ring that lurches with each keystroke
  looks broken), and the ring starts from rest each time the panel opens. It stops completely, with no animation frames,
  when the panel does not have the keyboard or the user prefers reduced motion. Rendered at 1 pixel per CSS pixel, 700k pixels at most.
- **Search glyph.** A hairline lens (1.3 px stroke), a short detached handle and a small glint inside the lens. The glint
  is the only moving part: still at the top left, circling once every 1.1 s while a search is in flight.
- **Loader.** A turning chrome orb and the words under way, each letter breathing a beat after the last (90 ms stagger,
  1.8 s). Used for "Warming up" and "Indexing". Everything animated pauses via `html[data-active="false"]`.
- **Content.** The list fades in 60 ms after the card starts growing; old results dim to 55% while a new search is in
  flight; the window grows at once and shrinks 280 ms after the card.

## Color

| Token | Value | Use |
|---|---|---|
| `--plate` | `#0e0e10` | the plate |
| `--plate-tint` | opaque graphite gradient plus a band of light | the finish over the plate |
| `--grain` | SVG noise, 0.004 x 0.7 frequency | brushed streaks over the tint |
| `--lift` | `rgba(255,255,255,.075)` | selected row |
| `--tile` | `rgba(255,255,255,.055)` | icon tiles, count and key chips |
| `--line` | `rgba(255,255,255,.07)` | hairlines between bar, list and hints |
| `--text` | `#eceef2` | names, input |
| `--text-2` | `#aaacb2` | snippets, notes |
| `--text-3` | `#8a8c93` | placeholder, paths, locations (5:1 on the plate) |
| `--chrome` | `#e6e8ec` | caret, focused glyph, selected tile |
| `--danger` | `#e0877f` | problems only |

## Type

Geist Variable for text, Geist Mono Variable for paths, locations, counts and key caps (measurement and data only), and
Sora Variable (19 px / 400, tracking -0.01em) for the one thing you type and its placeholder, chosen by the owner from six
candidates on 2026-10-16 (Bricolage Grotesque was tried first).
All three are bundled from `@fontsource-variable` (OFL), never loaded from a network. Scale: names 14.5 / 520,
snippets 12.5, notes 12.5, paths 11.5 mono, locations and chips 11 mono, hints 11.5.

## Components

- **Bar:** 60 px tall, search glyph (drawn, 1.6 stroke), input, loader chip at the right.
- **Row:** 44 px tile (kind icon, `PDF` in mono for PDFs, or the image's thumbnail), name with the matched words in
  weight 620 and white, the last two folders as a front-trimmed path, up to two lines of snippet with its location, `+N`
  chip for further matches. Selected: lifted plate with a 1 px top highlight. Expanded matches indent under the file.
- **Scope chip:** `Code`, `Images`, `PDFs` or `Documents` at the right of the bar when the query began with `code:`, `img:`,
  `pdf:` or `notes:` (a mono chip on `--tile`). Typing only the scope says what to do next.
- **Hints:** one row of key caps (`↑↓`, `Enter`, `Ctrl+Enter`, `→` when the file has more matches, `Esc`), with the
  index status at the right while indexing.
- **Notes:** one line each above the results (hotkey taken, engine failed, nothing matched, no strong match).
- **Browser surfaces:** themed selection, caret and scrollbar; no focus ring on rows (selection is the focus).

## Rules

- Nothing animated may run while the panel is closed; nothing visual may be added to the hotkey path.
- No accent color in the interface. Emphasis by weight and brightness; the ring is the one coloured thing.
- The ring is real shader output, never a CSS imitation (the static gradient behind it is only the no-WebGL fallback).
