/** Something you can do with the selected row, listed in the action panel (Ctrl+K). */
export type Action = {
  id: "open" | "reveal" | "copy-path";
  title: string;
  /** The shortcut that does it without the panel, shown beside it so the panel teaches the keys. */
  keys: string;
};

/** What can be done with a file. Other kinds of row (apps, links) add their own lists beside this one. */
export const fileActions: readonly Action[] = [
  { id: "open", title: "Open", keys: "Enter" },
  { id: "reveal", title: "Show in Explorer", keys: "Ctrl+Enter" },
  { id: "copy-path", title: "Copy path", keys: "Ctrl+Shift+C" },
];

/** The actions whose title contains every word typed, in the order given. */
export function filterActions(actions: readonly Action[], text: string): Action[] {
  const words = text.toLowerCase().split(/\s+/).filter(Boolean);
  return actions.filter((action) => words.every((word) => action.title.toLowerCase().includes(word)));
}
