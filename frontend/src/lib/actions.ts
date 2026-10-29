import type { Item } from "./flatten";
import { words } from "./platform.ts";

/** Something you can do with the selected row, listed in the action panel (Ctrl+K). */
export type Action = {
  id: "open" | "reveal" | "copy-path" | "admin" | "tag" | "pin" | "copy" | "delete";
  title: string;
  /** The shortcut that does it without the panel, shown beside it so the panel teaches the keys. */
  keys: string;
};

const open: Action = { id: "open", title: "Open", keys: "Enter" };
const paste: Action = { id: "open", title: "Paste", keys: "Enter" };
const copy: Action = { id: "copy", title: "Copy", keys: "Ctrl+Enter" };
const forget: Action = { id: "delete", title: "Delete this entry", keys: "Ctrl+Delete" };
const reveal: Action = { id: "reveal", title: `Show in ${words().fileManager}`, keys: "Ctrl+Enter" };
const copyPath: Action = { id: "copy-path", title: "Copy path", keys: "Ctrl+Shift+C" };
const admin: Action = { id: "admin", title: "Run as administrator", keys: "Ctrl+Shift+Enter" };
const tag: Action = { id: "tag", title: "Add or remove a tag", keys: "Ctrl+T" };
const pin: Action = { id: "pin", title: "Pin", keys: "Ctrl+P" };
const unpin: Action = { id: "pin", title: "Unpin", keys: "Ctrl+P" };
const turnOn: Action = { id: "open", title: "Turn on", keys: "Enter" };
const notNow: Action = { id: "delete", title: "Not now", keys: "Ctrl+Delete" };

/**
 * What can be done with a row: a file offers all of these; anything else offers what its entry says it can.
 * `taggable` says whether the row can carry a tag (see `tagIdOf`); `pinning` is what Ctrl+P would do to it,
 * or null if it cannot be pinned (see `pinIdOf`).
 */
export function actionsFor(item: Item, taggable: boolean, pinning: "pin" | "unpin" | null = null): Action[] {
  const offered = item.kind === "entry" ? new Set(item.entry.Actions ?? []) : new Set(["reveal", "copy-path"]);
  if (item.kind === "entry" && item.entry.Kind === "offer") return [turnOn, notNow];
  if (item.kind === "entry" && item.entry.Kind === "clip") return [paste, ...(offered.has("copy") ? [copy] : []), ...(offered.has("delete") ? [forget] : [])];
  if (item.kind === "entry" && item.entry.Kind === "snippet") return [paste, ...(pinning === "pin" ? [pin] : pinning === "unpin" ? [unpin] : [])];
  return [
    open,
    ...(offered.has("reveal") ? [reveal] : []),
    ...(offered.has("admin") ? [admin] : []),
    ...(offered.has("copy-path") ? [copyPath] : []),
    ...(taggable ? [tag] : []),
    ...(pinning === "pin" ? [pin] : pinning === "unpin" ? [unpin] : []),
  ];
}

/** What can be done with a row besides opening it: files can be revealed, everything else says what it can. */
export type Way = "open" | "reveal" | "admin" | "copy" | "delete";

/** Whether the row can be revealed, run as administrator, copied or deleted. */
export function available(item: Item, how: Exclude<Way, "open">): boolean {
  if (item.kind !== "entry") return how === "reveal";
  return item.entry.Actions?.includes(how) ?? false;
}

/** A copied text: Ctrl+Enter copies it and Ctrl+Delete forgets it. */
export const isClip = (item: Item) => item.kind === "entry" && item.entry.Kind === "clip";

/** A row Ctrl+Delete sets aside: a copied text is forgotten, an offer is put off for a month. */
export const forgets = (item: Item) => item.kind === "entry" && (item.entry.Kind === "clip" || item.entry.Kind === "offer");

/** A row whose Enter pastes (a copied text, a snippet) instead of opening. */
export const pastes = (item: Item) => item.kind === "entry" && (item.entry.Kind === "clip" || item.entry.Kind === "snippet");

/** The actions whose title contains every word typed, in the order given. */
export function filterActions<T extends { title: string }>(actions: readonly T[], text: string): T[] {
  const words = text.toLowerCase().split(/\s+/).filter(Boolean);
  return actions.filter((action) => words.every((word) => action.title.toLowerCase().includes(word)));
}
