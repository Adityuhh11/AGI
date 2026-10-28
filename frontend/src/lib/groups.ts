// With extensions, so the Node test runner can load it too.
import type { Row } from "../../bindings/omni/internal/search";

/** A run of file results under one heading; an empty heading means no heading (the list is not grouped). */
export type Group = { heading: string; rows: Row[] };

/** What the engine's kinds are called in a heading. */
const HEADINGS: Record<string, string> = { code: "Code", text: "Documents", pdf: "PDFs", image: "Images", video: "Videos", audio: "Audio" };

/** Fewer file rows than this stay one list: a heading over each of three rows is noise. */
export const MIN_ROWS_TO_GROUP = 4;

/**
 * Arranges file results for the list. With `group` on, the single best match comes first under "Best match", and the rest
 * follow under a heading per kind of file: the kinds ordered by their best file, the files in each by score. Without
 * `group`, or with too few rows, or with only one kind of file (a `code:` search), the rows are one list in the engine's order.
 */
export function arrange(rows: Row[], group: boolean): Group[] {
  const plain: Group[] = [{ heading: "", rows }];
  if (!group || rows.length < MIN_ROWS_TO_GROUP || new Set(rows.map((r) => r.Kind)).size < 2) return plain;
  const byScore = [...rows].sort((a, b) => b.Score - a.Score); // stable: ties keep the engine's order
  const [best, ...rest] = byScore;
  const groups: Group[] = [{ heading: "Best match", rows: [best] }];
  for (const row of rest) {
    const heading = HEADINGS[row.Kind] ?? "Other files";
    const found = groups.find((g, i) => i > 0 && g.heading === heading);
    if (found) found.rows.push(row);
    else groups.push({ heading, rows: [row] });
  }
  return groups;
}
