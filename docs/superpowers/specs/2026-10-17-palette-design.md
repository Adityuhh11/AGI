# Palette: apps, commands, browsing and tags in the one bar

Date: 2026-10-17. Status: proposed, for the owner's review. Branch `feat/palette`. Builds on the launcher
(`2026-10-15-launcher-design.md`, `2026-10-15-launcher-visual-design.md`) and the memory engine (`docs/api.md`). The v1 spec
listed "app and command launching" as its own later spec; this is it, with browsing, tags and the decision-model question.

## Goal

One hotkey opens one bar, and the bar is the only thing you reach for: launch an app, run a command, search the web in
Zen, jump to a window, open a tagged thing, or find a file by what it says. Windows Search is never touched again.
It stays fast (hotkey to painted input under 100 ms, every instant result in under 5 ms), local and ours: adding or
removing a capability is a package or a few lines of config, never a plugin system.

## What we are building against

Raycast's core, and where we stand. "Have" is already built here.

| Raycast feature | Here | Plan |
|---|---|---|
| App launcher, ranked by use | no | **P1** |
| File search by name | have, and by meaning (text, PDF pages, images) | have |
| Action panel (Cmd+K) | no | **P0** |
| Quicklinks, web search fallback | no | **P2** |
| System commands (lock, sleep, quit app...) | no | **P3** |
| Switch windows | no | **P3** |
| Calculator, unit conversion | no | **P3** |
| Aliases and per-item hotkeys | no | P3, as config |
| Snippets, text expansion | no | later |
| Clipboard history | no | **P5**, and searchable by meaning |
| Window management (snap, halves) | no | later |
| Floating notes, timers, emoji, translator | no | skip until wanted |
| AI chat, AI commands | no | **P6**, optional, cheap or local |
| Extension store | no | skip: user commands in `config.toml` instead |

What we add that Raycast cannot: **semantic search of your own files, PDFs and screenshots** (built), **search of what you
copied and browsed by meaning** (P5, reuses the engine unchanged), everything on disk and private, and a keymap and
command set that is a file you edit.

## Measured on this machine (2026-10-17)

| Fact | Number |
|---|---|
| Start Menu shortcuts (`.lnk`, all users and current user) | 130 files, scanned in 110 ms |
| Everything Windows shows in Start (`shell:AppsFolder`, includes 60 Store apps) | 203 entries; `Get-StartApps` takes 1.25 s, of which 0.18 s is PowerShell starting |
| Zen | `C:\Program Files\Zen Browser\zen.exe`, registered under App Paths, version 1.23.2b on Firefox 157 |
| Default browser | Chrome, so "open this URL" through Windows would open the wrong browser |
| Zen history and bookmarks | `places.sqlite`, 10 MB, in WAL mode (locked while Zen runs) |
| GPU memory in use with the embedding model loaded | 1.16 GB of 6.14 GB, so a 4B model at 4-bit (about 3 GB) fits beside it |
| Visible top-level windows | 10 (window switching is a small enumeration) |

## Interaction model

One hotkey (`Alt+Shift+Space`, exists) opens the bar. Everything else is a key inside the bar or a typed prefix, so the
muscle memory is: open, type, Enter.

| Key | Does |
|---|---|
| `Enter` | primary action of the selected row (open a file, launch an app, run a command, open a URL) |
| `Ctrl+Enter` | secondary action (reveal a file, show an app's folder, run as administrator) |
| `Ctrl+K` | action panel for the selected row: every action for that kind of thing, filterable by typing |
| `Tab` / `Shift+Tab` | cycle the mode chip: All, Apps, Files, Web, Tags. The chip is the scope chip that `code:` already sets |
| `Alt+Enter` | search the web for the text in Zen, from any mode |
| `Ctrl+T` | tag the selected row (type the tag, Enter) |
| `Esc` | clear the mode, then the text, then close |
| `↑ ↓ → ←` | as now (move, expand matches) |

Typed prefixes set the same chips without the keyboard detour: `code:` `img:` `pdf:` `doc:` (exist), plus `app:` (apps
only), `?` or `web:` (web), `#work` (items tagged work), `>` (commands only), `=` (calculator). A prefix and a Tab-chosen
mode are the same state, so there is one thing to learn.

Optional extra global hotkeys, one config line each, open the bar already in a mode (for example `Alt+Shift+B` opens in
Web mode). They cost one `RegisterHotKey` each, so they wait until the in-bar flow has been lived with.

### Flows

- **Launch**: hotkey, `vsc`, Enter. Instant results, ranked by how you use them.
- **Browse in Zen**: hotkey, type a question, `Alt+Enter`. Or `Tab` to Web mode, type, Enter. Zen opens the search in a
  new tab in its running window. With nothing matching locally, the last row is "Search the web for ..." so a plain
  Enter also gets there.
- **Quicklink**: `gh rust lifetimes` expands a trigger (`gh`, `yt`, `mdn`, anything configured) to a URL template.
- **Tag**: select anything, `Ctrl+T`, `work`, Enter. Later, `#work` lists everything tagged, mixed kinds.
- **Command**: `> lock`, or just `lock`, Enter. Destructive ones (shutdown, restart) ask first.

## Architecture

The launcher already has one provider, the engine's file search, called from `internal/search`. The palette adds
providers that answer from memory in microseconds and shows them above the files, which arrive a moment later.

```
keys ──> Svelte bar ──> internal/palette.Query(text, mode) ──> instant sections (apps, commands, windows, tags, web row)
                    └─> internal/search.Search(text)  ─────────> files (engine, async, as today)
```

- `internal/palette`: the `Provider` interface (`Query(text) []Item`) and the aggregator. An `Item` is `{ID, Kind, Title,
  Subtitle, Icon, Score}` plus the actions its kind supports. Files keep their richer row (matched chunks); the page
  renders by kind. One interface, written when the second provider arrives, not before.
- `internal/apps`: finds, indexes, launches and ranks applications.
- `internal/commands`: the registry of commands, built in or from `config.toml`.
- `internal/rank`: the fuzzy matcher and frecency, shared by every provider.
- `internal/state`: the one small file of things learned about you (frecency, tags, learned prefixes), JSON written
  atomically in `%APPDATA%\AGI\state.json`. Thousands of entries at most, so a database would be bloat.
- `internal/web` (P2): browser detection, engines and quicklinks.

Instant results never wait on the engine, the model or the disk, and nothing here touches the hotkey path: the indexes
live in memory and are loaded before the first keystroke.

### Which results show, and in what order

Local providers show only on a strong name match (prefix, word start or acronym), at most five rows, so
"meeting notes from march" is not polluted by an app called Notes. Sections appear in a fixed order: apps and commands,
then files. The first row is selected, so a strong app match makes Enter launch it. Within a provider the score is
`match quality x (1 + frecency)`, and each launch also teaches `typed prefix -> item` ("ch" becomes Chrome after you
choose it twice), which is how Raycast's ranking feels so right.

## Apps (P1)

**How they are found.** Two sources, merged by target: the Start Menu folders (130 shortcuts here, 110 ms to scan) and
Windows' own list (`shell:AppsFolder`, 203 entries, the only way to see Store apps). Windows' list is reached by running
`Get-StartApps` in a background process (1.25 s, off the hot path), cached to disk, and refreshed at start, when the Start
Menu folders change (the file watcher already exists) and daily. A native COM enumeration would remove the 1.25 s but
adds a dependency for no visible gain; it is upgraded only if the refresh ever matters.

**How "what is there and what is not" is kept.** The index is rebuilt from the sources on each refresh, so an uninstalled app
disappears at the next refresh; and every launch checks the target first, dropping the entry if it vanished
(never a dead row that does nothing). Nothing is hand-maintained.

**How they start.** Shortcuts with `ShellExecute` on the `.lnk` (keeps arguments, working directory and the
administrator flag), Store apps through `shell:AppsFolder\<AppUserModelID>`. The same call the launcher already uses to
open files.

**Icons.** Extracted from the shortcut or executable with the shell's icon API into a cache, served the way thumbnails
are (`/icon`). A launcher without icons feels dead, so they are in P1, not later.

**Matching.** A small fuzzy matcher (about 100 lines, tested as a table): prefix over word-start over acronym
(`vsc` finds Visual Studio Code, `ppt` PowerPoint) over subsequence, with a penalty for gaps. Not a dependency.

**Measure before moving on:** index load and match time per keystroke (target under 2 ms p99 over the real 203 apps), the
refresh time, and the app list compared with `Get-StartApps` (nothing missing, nothing extra). The hotkey path is
re-measured with `cmd/panelbench` and must not move.

## Browsing in Zen (P2)

Zen is Firefox underneath, so it takes a URL on its command line and opens it in a new tab of the running window. The
launcher cannot ask Windows to "open the default browser" (yours is Chrome), so the browser is configured and detected:
`browser = "zen"` finds `zen.exe` through App Paths (as measured). A search is `zen.exe <url>` with the URL built from a
template (`https://duckduckgo.com/?q={q}`), which does not depend on Zen's own default engine. A spike checks whether
Firefox's `-search <text>` flag (uses the engine set inside Zen) works in this build, and uses it if it does, because it
respects the engine you chose.

Quicklinks are the same template with a trigger word, in `config.toml`:

```toml
[[quicklink]]
trigger = "gh"
title = "GitHub search"
url = "https://github.com/search?q={q}"
```

**Later, and the part Raycast cannot do (P5):** Zen's bookmarks and history are searchable by meaning. A job copies
`places.sqlite` and its WAL (Zen holds it open), writes bookmarks and history as plain text files under
`%LOCALAPPDATA%\AGI\browser\` (one line per page: title and URL), and the engine indexes that folder like any other
root, with no engine change. A hit on one of those lines opens the URL in Zen instead of the text file. "That article
about CUDA graphs I read last week" becomes a query.

## Commands (P3)

A registry of `{ID, Title, Keywords, Run}`. Built in: lock, sleep, restart, shutdown (each asks), empty the Recycle Bin,
open a Settings page (`ms-settings:` links), quit a running app, switch to a window (enumerates top-level windows and
focuses one), calculator and unit conversion in the bar (`= 2 * 3.5`). User commands and aliases live in `config.toml`:

```toml
[[command]]
title = "Open the AGI repo"
run = ['code', 'C:\dev\VS_CODE\AGI']
alias = "agi"
```

This is the extension story: a file you edit, not a plugin API. Anything that outgrows it becomes a Go package here.

## Tags (P4)

A tag is a label on anything the bar can show: a file, a folder, an app, a URL. Stored in `state.json` as
`tag -> [item ids]`. `Ctrl+T` adds one, `#work` lists everything tagged (all kinds), `#work budget` filters file results
to tagged paths. The model stays that small; automatic tags suggested by a model come with P6, if wanted.

## Decision models (P6, optional)

Researched on 2026-10-17; the field is weeks old (everything below launched in September and October 2026) and mostly
vendor-reported, so each number is a claim until the spike measures it.

**What a decision model is.** Not a chat model. You send a `state` (text, JSON, an image) and typed `questions`: a **choice**
(pick one option from a labelled set), a **score** (place it on ordered levels) or a **noul** (yes/no as a probability). It
answers with a probability for every option and a calibrated confidence, in typed JSON, in tens to hundreds of
milliseconds, for about four hundredths of a dollar per million input tokens (output is free). Several questions about the
same state run in one request. That is exactly "route a loose phrase to the right action, and know when not to": below a
confidence threshold the bar does nothing clever.

**Who has one.**

| Model | Where | Reported |
|---|---|---|
| Jev (TypeSafe AI, first, September 2026) | hosted: `api.typesafe.ai/v1/systemone`, or OpenRouter `POST /api/alpha/decisions` | $0.042 per M input tokens, 70 to 500 ms; Vercel tests it for classification |
| Clef-flash (Cloudflare, 9B, October 2026) | Workers AI; weights open (Apache 2.0, Hugging Face `Cloudflare/clef-flash`) | $0.038 per M input, 24k context, median 38.8 ms (p95 122 ms) |
| Clef (Cloudflare, 27B, with vision) | Workers AI; weights open | $0.24 per M input, 64k context, median 209 ms |
| Microsoft-Decision-1 | Foundry and OpenRouter; weights not released | $0.042 per M input |
| OpenAI Decisions API | limited preview, no public contract found | not usable yet |
| Laya | open, reported about 0.8 GB, median 5.8 ms, "trades off quality" | one source only |

AWS, Upstage and Ollama have agreed on the Jev-style request shape (`/v1/systemone`), and the gateways (OpenRouter,
LiteLLM, TrueFoundry, Requesty) expose it, so one small client covers all of them: switching provider is a base URL and a key.
Cloudflare says it does not read, store or train on Workers AI requests.

**What people built with them** (all hobby or vendor demos): voice smart-home control at about 185 ms a command; a Home
Assistant conversation agent that routes each sentence in one request and hands anything below a confidence threshold to a
fallback agent; a voice-driven browser where the model picks intent and target in about 300 ms per spoken word; agent skill
and model routing; ticket triage and guardrails. No command palette found: that niche is open.

**Local.** Clef-flash runs under Ollama (0.35.1 and later) but its builds are about 11 GB, a GGUF reported to fit 8 GB, and an
open Ollama issue reports `non-finite logit` failures; this laptop has 6.14 GB with 1.16 GB in use, so it does not fit
today. Laya, if it checks out, is small enough. Until then local means the embedding router.

**Tiers.**

1. **Deterministic** (prefix, fuzzy, quicklink trigger): almost everything, costs nothing.
2. **Embedding router**, local: each command's title, keywords and a sentence are embedded at startup by the model already
   loaded; a loose phrase picks the nearest above a threshold. No network, milliseconds, one small engine route to embed text.
3. **A decision model**, hosted, asynchronous and optional: the bar shows tier 1 and 2 results at once, and when the
   decision model is confident it adds or reorders one row ("Run: Lock", "Open: Calculator") a moment later. One request
   can carry several questions at once (which kind of thing is this, is it a command, which one, is it a web question, does
   it name a file kind), and the answers set the mode chip and filters for you. Used on typed text only, never on file
   contents unless you choose "Ask about this file", and the bar shows when it is calling out. At these prices a hundred
   calls a day cost less than a cent, and Cloudflare's free allowance may cover it.
4. **A chat model**, later and separate (summarise hits, answer questions): a different API from decisions, and a later
   decision.

The client is `internal/decide`: the System One request shape, a provider list in `config.toml` (base URL, model, key in
Credential Manager), a timeout of a few hundred milliseconds so a slow network never slows the bar, and a confidence floor.

**Spike before choosing** (measure, don't guess): about 60 phrasings of commands and web or file questions in your own words,
each with the answer you would want, run through the embedding router, Jev (direct and through OpenRouter), Clef-flash and
Clef on Workers AI, and Microsoft-Decision-1. Record accuracy at 1, whether confidence separates right from wrong (the
point of these models), p50 and p95 latency from this machine on this network, and cost per thousand calls. Add a local
candidate if one fits 6 GB. The output goes in `docs/eval/` and the choice becomes an ADR.

## Order of work and acceptance

Each phase ships on its own, leaves the bar faster or more useful, and is measured before the next.

| Phase | Content | Done when |
|---|---|---|
| P0 | Palette model, action panel (`Ctrl+K`), `Tab` modes, `state.json`, fuzzy matcher and frecency | files behave as before; `Ctrl+K` shows reveal/copy path/open; matcher table tests pass |
| P1 | App launcher: index, icons, launch, learned prefixes | `hotkey, vsc, Enter` launches Code; match under 2 ms p99; list equals `Get-StartApps`; hotkey path unchanged |
| P2 | Browse in Zen: browser detection, engines, quicklinks, `Alt+Enter`, web row | a question typed in the bar opens as a search in Zen in under a second |
| P3 | Commands, window switcher, calculator, user commands | lock, quit, switch window and `= 12*1.18` work; destructive ones confirm |
| P4 | Tags | `Ctrl+T`, `#work` across kinds |
| P5 | Clipboard history and Zen history/bookmarks, indexed by meaning | "the link about X" finds it; passwords never recorded |
| P6 | Embedding router, decision-model client (`internal/decide`), model spike | the spike's numbers are in `docs/eval/` and the choice is an ADR |

This branch covers P0 to P2, the core of "never touch Windows search again" and "browse from the bar". P3 and P4 follow
on the same branch if the owner wants; P5 and P6 are each their own spec when reached.

## Risks and open questions

- **Store apps.** Launching through `shell:AppsFolder` is standard but untested here: the P1 spike launches Calculator,
  Settings and a packaged app before the design is relied on.
- **Zen's command line.** `zen.exe <url>` reusing the running window is Firefox behaviour; the P2 spike confirms it and
  `-search`.
- **Clipboard privacy (P5).** Password managers mark what they copy; we honour that flag and a deny list, and keep the
  history in the index folder only, never anywhere else.
- **Ranking across kinds.** The "strong match only" rule may hide a weak but wanted app; the threshold is tuned on the
  owner's real queries, as the file ranking was.
- **Which decision model.** Open until the spike: Jev, Clef-flash and Microsoft-Decision-1 are all about four hundredths of a
  dollar per million input tokens, so quality on your phrasings and the latency from here decide it. All of it is weeks old and
  vendor-reported.
