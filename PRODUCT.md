# Product

<!-- impeccable:product-schema 1 -->

<!-- Written 2026-10-15 from the owner's brief and the repo docs without an interview round: the owner was away and said
     "go ahead, I will check the design later". Facts marked (assumed) are inferences to confirm. -->

## Platform

web

## Stack

Wails v3 (Go) hosting a Svelte 5 + TypeScript page in WebView2 on Windows 11. Fonts and libraries are bundled, never
loaded from a network.

## Users

The owner, a student and developer who keeps code, lecture PDFs, screenshots and notes spread over Downloads, OneDrive
and `C:\dev`. They open the launcher dozens of times a day with a global hotkey from inside another app, usually
mid-task, and want to be back in that app within seconds. (assumed: other people may use it later; v1 is built for one.)

## Product Purpose

A personal, local-first, Raycast-like launcher: press a hotkey, type, find what is on this laptop by meaning or by exact
words, open it. Files first (code, PDFs by page, images by what they show, notes); apps, commands, clipboard and an LLM
layer are planned. Success is a hotkey-to-result loop that feels faster than thinking, over an index that never
leaves the machine. It replaces the paid tiers of existing launchers.

## Positioning

Semantic search over the user's own files, run entirely on the laptop, with the match shown in place (the page, the
line range, the thumbnail). Nothing leaves the machine except calls the owner configures to an LLM provider.

## Operating Context

A frameless, transparent, always-on-top panel that appears over whatever app is in front, opens in about 32 ms and
hides again on Escape, a click elsewhere, or opening a result. The page stays alive while the panel is closed (the
window is cloaked, not hidden), so anything animated must pause when the panel loses the keyboard. The sidecar engine
takes 30 to 40 s to load its model after a cold start; the page must say so honestly. Results come from an index of
about 56,000 chunks and update live while typing (120 ms debounce, about 200 ms answer).

## Capabilities and Constraints

- Hotkey to focused, painted input under 100 ms; the path never waits on a model. Measured with `cmd/panelbench`.
- Results are files with up to three matching chunks each; Right/Left expand, Enter opens, Ctrl+Enter reveals,
  Ctrl+Shift+C copies the path, Escape closes, the pointer also works.
- Image results show a thumbnail served from `/thumb`; everything else shows a kind icon.
- States the page must carry: engine starting, indexing, no matches, no strong match (closest files), engine failed,
  hotkey taken.
- Windows 11 only; 6 GB VRAM shared with the embedding model, so the page must be cheap on the GPU.

## Brand Commitments

Binding visual asks from the owner (2026-10-15): a heavily rounded, near-circular pill search bar that can morph into
a card; a liquid-metal animated border flowing around it; a text loader with a spinning orb for waiting states; better
typography. No flat rectangular box.

## Evidence on Hand

Real index of the owner's laptop (56k chunks), screenshots and PDFs for thumbnails, the two reference components named
by the owner (liquid-metal button, text loader). No logo or wordmark exists. No user research.

## Product Principles

1. The hotkey path is sacred: no visual feature may cost latency or run while the panel is closed.
2. Show the match, not just the file: the evidence for a result is part of the result.
3. Honest states over spinners: say what the engine is doing, and say when nothing matched strongly.
4. Keyboard first, pointer supported.
5. Local and private by construction; nothing in the UI implies a cloud.

## Accessibility & Inclusion

Keyboard-complete; the results are a labelled listbox; contrast at WCAG AA; `prefers-reduced-motion` freezes the metal
ring and the loader. (assumed: no screen-reader audience today, kept correct anyway.)
