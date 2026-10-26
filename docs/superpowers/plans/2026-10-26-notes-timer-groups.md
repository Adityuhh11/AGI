# Plan: notes, timer, grouped results, selection feel

Spec: `docs/superpowers/specs/2026-10-26-notes-timer-groups-design.md`. One commit per step; each step ends with its tests green
(`go test ./...`, `npm test`, `npm run check`).

1. Config: `[ui] selection_effect`, `[search] group_results`, `[timer]` (validated), plus the settings plumbing and the General,
   File search and Timer pages.
2. Selection feel: the `keying` state in the bar, CSS, smooth scroll.
3. Grouped results: `lib/groups.ts`, headings in the list.
4. `internal/calc`: parser, units; the bar's answer row; copy on Enter.
5. System commands rows.
6. `internal/notes`: store, service; the window host learns always-on-top and a record of its size; `lib/markdown.ts`;
   `Notes.svelte`; the bar's `note:` mode.
7. `internal/timer`: state machine on an injected clock; service; the overlay window; `Timer.svelte`; the bar's `timer` rows.
8. Docs, a build of the real app, opening each window, a review of the whole diff.
