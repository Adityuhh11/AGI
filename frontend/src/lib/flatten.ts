import type { Item as Entry } from "../../bindings/agi/internal/palette";
import type { Match, Row } from "../../bindings/agi/internal/search";

/** A line of the result list that is a file: the file itself, or (when expanded) one more match inside it. */
export type FileItem = { kind: "row"; row: Row } | { kind: "match"; row: Row; match: Match };

/** One selectable line of the result list: an instant answer (an app, a quicklink, the web search), a file, or a match inside a file. */
export type Item = { kind: "entry"; entry: Entry } | FileItem;

const matchesOf = (row: Row): Match[] => row.Matches ?? [];

/** How many matches a file has beyond the first one the row already shows. */
export function moreCount(row: Row): number {
  return Math.max(0, matchesOf(row).length - 1);
}

/**
 * The visible list: a quicklink the text triggers, then the apps, then every file (each followed by its
 * further matches if it is expanded), and the web search last. With `keepOrder` (the pinned things shown
 * when nothing is typed) the entries are listed as given: pins are in the order they were pinned.
 */
export function flatten(rows: Row[], expanded: ReadonlySet<string>, entries: Entry[] = [], keepOrder = false): Item[] {
  if (keepOrder) return entries.map((entry) => ({ kind: "entry", entry }));
  const of = (match: (e: Entry) => boolean): Item[] =>
    entries.filter(match).map((entry) => ({ kind: "entry", entry }));
  const items: Item[] = [...of((e) => e.Kind === "quicklink"), ...of((e) => e.Kind !== "quicklink" && e.Kind !== "web")];
  for (const row of rows) {
    items.push({ kind: "row", row });
    if (expanded.has(row.Path)) {
      for (const match of matchesOf(row).slice(1)) items.push({ kind: "match", row, match });
    }
  }
  items.push(...of((e) => e.Kind === "web")); // the web search is the fallback, last of all
  return items;
}

/** The selection after moving by delta, never leaving the list. */
export function step(index: number, delta: number, count: number): number {
  if (count === 0) return 0;
  return Math.min(count - 1, Math.max(0, index + delta));
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
  if (item.kind === "entry" && (item.entry.Kind === "snippet" || item.entry.Kind === "cmd")) return item.entry.ID;
  return tagIdOf(item);
}
