import type { Item as Entry } from "../../bindings/omni/internal/palette";
import type { Match, Row } from "../../bindings/omni/internal/search";
import { arrange } from "./groups.ts";

/** A line of the result list that is a file: the file itself, or (when expanded) one more match inside it. */
export type FileItem = { kind: "row"; row: Row; heading?: string } | { kind: "match"; row: Row; match: Match };

/** One selectable line of the result list: an instant answer (an app, a quicklink, the web search), a file, or a match inside a file. */
export type Item = { kind: "entry"; entry: Entry } | FileItem;

const matchesOf = (row: Row): Match[] => row.Matches ?? [];

/** How many matches a file has beyond the first one the row already shows. */
export function moreCount(row: Row): number {
  return Math.max(0, matchesOf(row).length - 1);
}

/**
 * The visible list: a quicklink the text triggers, then the apps, then every file (each followed by its
 * further matches if it is expanded; under headings per kind of file when `group` is on, see groups.ts), an offer, and the web search last. With `keepOrder` (the pinned things shown
 * when nothing is typed) the entries are listed as given: pins are in the order they were pinned.
 */
export function flatten(rows: Row[], expanded: ReadonlySet<string>, entries: Entry[] = [], keepOrder = false, group = false): Item[] {
  if (keepOrder) return entries.map((entry) => ({ kind: "entry", entry }));
  const of = (match: (e: Entry) => boolean): Item[] =>
    entries.filter(match).map((entry) => ({ kind: "entry", entry }));
  const items: Item[] = [...of((e) => e.Kind === "quicklink"), ...of((e) => e.Kind !== "quicklink" && e.Kind !== "web" && e.Kind !== "offer")];
  for (const { heading, rows: inGroup } of arrange(rows, group)) {
    inGroup.forEach((row, at) => {
      items.push({ kind: "row", row, heading: at === 0 && heading ? heading : undefined });
      if (expanded.has(row.Path)) {
        for (const match of matchesOf(row).slice(1)) items.push({ kind: "match", row, match });
      }
    });
  }
  items.push(...of((e) => e.Kind === "offer")); // an offer (to search sound too) comes after the files it is about
  items.push(...of((e) => e.Kind === "web")); // the web search is the fallback, last of all
  return items;
}

/** The selection after moving by delta, wrapping round: holding Down runs to the last row and comes back to the first. */
export function step(index: number, delta: number, count: number): number {
  if (count === 0) return 0;
  return (((index + delta) % count) + count) % count;
}

/** The line to open an editor at for a selected item (0 means no particular line). */
export function openLine(item: FileItem): number {
  return item.kind === "match" ? item.match.Line : item.row.Line;
}

/** The file a selected item belongs to. */
export function pathOf(item: FileItem): string {
  return item.row.Path;
}

/** The id a row is tagged by: `file:<path>` for a file (and each of its matches), the entry's own id for an app or a tagged file. */
export function tagIdOf(item: Item): string | null {
  if (item.kind !== "entry") return "file:" + item.row.Path;
  return item.entry.Kind === "app" || item.entry.Kind === "file" ? item.entry.ID : null;
}

/** The file on disk behind a row, if it has one (a file result, or a tagged file). */
export function filePathOf(item: Item): string | null {
  if (item.kind !== "entry") return item.row.Path;
  return item.entry.Kind === "file" ? item.entry.ID.slice("file:".length) : null;
}

/**
 * The id a row is pinned by: the same as its tag id for a file or an app, and for a quicklink its trigger
 * alone (`quicklink:gh:`), because a pin is the shortcut, not what was typed after it; a snippet and a built-in
 * row are pinned by their own ids. The web search, copied texts and tags cannot be pinned.
 */
export function pinIdOf(item: Item): string | null {
  if (item.kind === "entry" && item.entry.Kind === "quicklink") return "quicklink:" + item.entry.ID.split(":")[1] + ":";
  if (item.kind === "entry" && (item.entry.Kind === "snippet" || item.entry.Kind === "cmd" || item.entry.Kind === "system")) return item.entry.ID;
  return tagIdOf(item);
}
