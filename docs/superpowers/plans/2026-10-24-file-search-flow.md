# File search flow: plan

Spec: `../specs/2026-10-24-file-search-flow-design.md`. Branch `feat/file-search-flow` from `dev`.

1. **Config.** `Search.HoldIndexing` (`hold_indexing`, omitted when false). Test: round trip, absent by default, a file without the key.
2. **Service.** `install()` sets the hold when the engine was not installed; `StartIndexing()` clears it; `SearchView` gains `HoldIndexing`
   and `Indexed` (files in the index); a folder says "Waits for you to start indexing" while held. Tests with the existing fakes.
3. **Wiring.** `main.go`: the indexing manager's `ready` is `engine ready && !hold`; `statusLine` says "File search is ready. Open Settings
   to choose what to index." when held; the settings subscriber refreshes the tray when the hold changes. Test `statusLine`.
4. **Page.** `lib/indexingPlan.ts` (cost order, direction of a change, the lines) with `node --test`; `lib/settings/draft.svelte.ts` (the
   draft, outside the component); `FileSearch.svelte` reordered into Engine, Folders, What to index (with the draft bar), Indexing (progress bar,
   speed, pause, per-folder lines) and the Start callout.
5. **Docs.** `STATUS.md` S2, README and `scripts/release-readme.txt` (the first-time flow), `ARCHITECTURE.md` (the hold) if it needs a line.
6. **Real run.** Clean profile as in the spec; record in `docs/eval/2026-10-24-file-search-flow.md`.
