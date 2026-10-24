# File search flow: choose first, then index; save before re-indexing

Date: 2026-10-24. Status: approved by the owner in conversation ("start with the specs and get started"); phase S2 in `STATUS.md`.

## Why

Two things on the File search page surprised the owner while using it for real:

1. **Indexing starts before the person chose anything.** "Download and turn on" ends with `Enabled = true`; the engine starts
   and `indexing.Manager` immediately syncs every configured folder with the default choices. A person who would have picked
   "Words only" (no graphics card) or "Gentle" finds an hour of indexing already under way.
2. **Every click on a choice re-indexes at once.** `SetIndexing` saves, `main.go` calls `indexer.SetOptions` and `Rescan`. Clicking
   through Pages, Photos and Videos starts up to three passes, each of which can embed thousands of pictures, with no word about
   what it will cost.

What already works and stays: a changed choice re-processes only the kind of file concerned, a chunk already stored keeps its vector,
and chunks the new choice no longer wants are deleted by the same sync (`indexer.py`, `eval/2026-10-20-indexing-choices.md`). So
"delete the old one" needs no engine work; only the moment of starting does.

## What changes

### 1. A first install waits for "Start indexing"

`[search] hold_indexing` (bool, default false, written only when true). `TurnOn` on an engine that is not installed sets it, together with
the download. `main.go` passes `engine ready && !hold_indexing` as the indexing manager's `ready`, so the manager's first cycle simply waits
(it already polls `ready` every 500 ms) and the folders, the watcher's events and "Rescan now" all wait with it.

The page, once the engine is ready and the hold is on, shows at the top of the File search page:

> **Ready.** Choose what to index and which folders, then start. *[Start indexing]*

with the device line ("runs on the graphics card" / "on the processor"). `StartIndexing()` clears the flag; the held cycle goes on at its next
poll. While held, choices apply at once (nothing is indexed, so there is nothing to protect). The button is disabled with "Add a folder first"
when there are no folders. The tray says "File search is ready. Open Settings to choose what to index." and each folder says "Waits for you to
start indexing".

Why a flag that is *held* and not one that is *started*: an existing install has no such key, so it keeps indexing exactly as before; no
migration. An update of an installed engine (`NeedsUpdate`) never sets it. A reinstall after "Delete downloaded files" does, and costs one click.

### 2. Changing a choice on an index that exists is a draft until saved

When some folder already holds files and the hold is off, clicking an option on **What to index** changes a draft held by the page, not the
setting. A bar under the choices appears:

> **Not saved.** *What this does:* one line per changed kind. *[Save and re-index]* *[Discard]*

The lines are static and come from the measured rates; the direction is known without asking the engine:

| Change | Line |
|---|---|
| to a cheaper option (`all`→`auto`→`text`, `content`→`name`, video `content`→`name`) | "Pictures you no longer search by are removed from the index. Nothing is computed, so it is quick." |
| to a costlier option | "Those files are looked at again and the new pictures indexed. About 3 minutes per 1,000 pictures on the graphics card, about 80 minutes on the processor. Search keeps working meanwhile." |

*Save and re-index* calls the existing `SetIndexing`, which saves and rescans, exactly one pass for all three kinds. *Discard* drops the draft.
The draft lives in a module of the page (not in the File search component), so it survives switching to another settings page; closing the
window drops it. The speed (pace) is not a draft: it is cheap and reversible and applies live, as today. Folders are not drafts either:
adding one is the act of asking for it to be indexed, and removing one already asks for confirmation.

### 3. The page, tidied

The page is in the order a person acts in, each part a block with one job:

1. **Search engine**: what is installed and running (download, progress, turn off, update, delete files), then the Start callout when it applies.
2. **Folders to search**, each with its own state line ("Indexing: 41 of 96 files", "Waits for you to start indexing", "96 files searchable").
3. **What to index** (the three choices, the device warning, the draft bar).
4. **Indexing**: the status sentence, a progress bar for the folder being indexed, pause and resume, and the speed. Pause moves out of the
   engine block, where it sat beside "Turn off".

Folders stay visible before the engine is installed so they can be set up first; each says it waits for file search to be turned on.

## Not doing

- **A dry-run count from the engine** ("this will embed 1,284 pages"). Would need a new route that walks the folders. The static line is honest
  about the cost per item; add the route if people ask for the count.
- **Auto-applying on a timer** or on closing the window: a re-index that starts when a window closes is the surprise we are removing.
- **Staging folders and pace.** See above.

## Tests

- `config`: `hold_indexing` round-trips, is omitted when false, a file without it keeps indexing.
- `settingsui`: `TurnOn` on an uninstalled engine sets the hold, on an installed or updating one does not; `StartIndexing` clears it;
  `Search()` reports `HoldIndexing` and a folder's text while held.
- `main` wiring: the `ready` function is false while held (a small pure function, tested), and the status line says so.
- Frontend: `indexingPlan.ts` (which lines a draft produces, which direction each change is) with `node --test`, like the other libs.
- By hand, in a clean profile (`APPDATA` and `LOCALAPPDATA` pointed at an empty folder): download, see nothing index, pick choices, Start; then
  change a choice, see the bar, Discard, change again, Save, watch one pass.
