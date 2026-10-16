import type { Item as Entry } from "../../bindings/agi/internal/palette";
import type { Match, Row } from "../../bindings/agi/internal/search";

/** A line of the result list that is a file: the file itself, or (when expanded) one more match inside it. */
export type FileItem = { kind: "row"; row: Row } | { kind: "match"; row: Row; match: Match };

/** One selectable line of the result list: an app (or other instant answer), a file, or a match inside a file. */
export type Item = { kind: "app"; entry: Entry } | FileItem;

const matchesOf = (row: Row): Match[] => row.Matches ?? [];

/** How many matches a file has beyond the first one the row already shows. */
export function moreCount(row: Row): number {
  return Math.max(0, matchesOf(row).length - 1);
}

/**
 * The visible list: the instant answers (apps) first, then every file, each followed by its further
 * matches if the file is expanded.
 */
export function flatten(rows: Row[], expanded: ReadonlySet<string>, entries: Entry[] = []): Item[] {
  const items: Item[] = entries.map((entry) => ({ kind: "app", entry }));
  for (const row of rows) {
    items.push({ kind: "row", row });
    if (expanded.has(row.Path)) {
      for (const match of matchesOf(row).slice(1)) items.push({ kind: "match", row, match });
    }
  }
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
