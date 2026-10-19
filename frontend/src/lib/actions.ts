import type { Item } from "./flatten";

/** Something you can do with the selected row, listed in the action panel (Ctrl+K). */
export type Action = {
  id: "open" | "reveal" | "copy-path" | "admin" | "tag";
  title: string;
  /** The shortcut that does it without the panel, shown beside it so the panel teaches the keys. */
  keys: string;
};

const open: Action = { id: "open", title: "Open", keys: "Enter" };
const reveal: Action = { id: "reveal", title: "Show in Explorer", keys: "Ctrl+Enter" };
const copyPath: Action = { id: "copy-path", title: "Copy path", keys: "Ctrl+Shift+C" };
const admin: Action = { id: "admin", title: "Run as administrator", keys: "Ctrl+Shift+Enter" };
const tag: Action = { id: "tag", title: "Add or remove a tag", keys: "Ctrl+T" };

/**
 * What can be done with a row: a file offers all of these; anything else offers what its entry says it can.
 * `taggable` says whether the row can carry a tag (see `tagIdOf`).
 */
export function actionsFor(item: Item, taggable: boolean): Action[] {
  const offered = item.kind === "entry" ? new Set(item.entry.Actions ?? []) : new Set(["reveal", "copy-path"]);
  return [
    open,
    ...(offered.has("reveal") ? [reveal] : []),
    ...(offered.has("admin") ? [admin] : []),
    ...(offered.has("copy-path") ? [copyPath] : []),
    ...(taggable ? [tag] : []),
  ];
}

/** The actions whose title contains every word typed, in the order given. */
export function filterActions<T extends { title: string }>(actions: readonly T[], text: string): T[] {
  const words = text.toLowerCase().split(/\s+/).filter(Boolean);
  return actions.filter((action) => words.every((word) => action.title.toLowerCase().includes(word)));
}
