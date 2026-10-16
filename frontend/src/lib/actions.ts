import type { Item } from "./flatten";

/** Something you can do with the selected row, listed in the action panel (Ctrl+K). */
export type Action = {
  id: "open" | "reveal" | "copy-path" | "admin";
  title: string;
  /** The shortcut that does it without the panel, shown beside it so the panel teaches the keys. */
  keys: string;
};

const open: Action = { id: "open", title: "Open", keys: "Enter" };
const reveal: Action = { id: "reveal", title: "Show in Explorer", keys: "Ctrl+Enter" };
const copyPath: Action = { id: "copy-path", title: "Copy path", keys: "Ctrl+Shift+C" };
const admin: Action = { id: "admin", title: "Run as administrator", keys: "Ctrl+Shift+Enter" };

/** What can be done with a row: a file always offers all of these; an app offers what its entry says it can. */
export function actionsFor(item: Item): Action[] {
  if (item.kind !== "app") return [open, reveal, copyPath];
  const offered = new Set(item.entry.Actions ?? []);
  return [open, ...(offered.has("reveal") ? [reveal] : []), ...(offered.has("admin") ? [admin] : [])];
}

/** The actions whose title contains every word typed, in the order given. */
export function filterActions(actions: readonly Action[], text: string): Action[] {
  const words = text.toLowerCase().split(/\s+/).filter(Boolean);
  return actions.filter((action) => words.every((word) => action.title.toLowerCase().includes(word)));
}
