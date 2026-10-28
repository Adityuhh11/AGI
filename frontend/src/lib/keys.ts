// With extensions, so the Node test runner can load it too.
import { forgets, isClip, type Way } from "./actions.ts";
import { filePathOf, pinIdOf, tagIdOf, type Item } from "./flatten.ts";

/** What a key press in the bar asks for. The page carries it out; null leaves the key to the input. */
export type Command =
  | { do: "settings" | "escape" | "web" | "expand" | "collapse" | "copy-path" | "tag" | "pin" | "actions" }
  | { do: "mode"; by: 1 | -1 }
  | { do: "move"; by: 1 | -1 }
  | { do: "run"; how: Way };

/** What the bar knows about the selection when a key is pressed. */
export type KeyContext = {
  current: Item | undefined;
  atEndOfText: boolean; // the cursor is after the last character, with nothing selected
  expanded: boolean; // the selected file shows its further matches
};

type Key = Pick<KeyboardEvent, "key" | "ctrlKey" | "shiftKey" | "altKey">;

/** The command for a key, or null. */
export function keyCommand(e: Key, { current, atEndOfText, expanded }: KeyContext): Command | null {
  if (e.ctrlKey && e.key === ",") return { do: "settings" };
  switch (e.key) {
    case "Escape":
      return { do: "escape" };
    case "Tab":
      return e.ctrlKey || e.altKey ? null : { do: "mode", by: e.shiftKey ? -1 : 1 };
    case "ArrowDown":
    case "ArrowUp":
      return { do: "move", by: e.key === "ArrowDown" ? 1 : -1 };
    case "Enter":
      if (e.altKey) return { do: "web" };
      if (e.ctrlKey && e.shiftKey) return { do: "run", how: "admin" };
      if (e.ctrlKey) return { do: "run", how: current && isClip(current) ? "copy" : "reveal" };
      return { do: "run", how: "open" };
    case "Delete":
      return e.ctrlKey && current && forgets(current) ? { do: "run", how: "delete" } : null;
    case "ArrowRight":
      return atEndOfText && current ? { do: "expand" } : null;
    case "ArrowLeft":
      return atEndOfText && current && current.kind !== "entry" && (current.kind === "match" || expanded) ? { do: "collapse" } : null;
  }
  if (!e.ctrlKey || !current) return null;
  switch (e.key.toLowerCase()) {
    case "c":
      return e.shiftKey && filePathOf(current) ? { do: "copy-path" } : null;
    case "t":
      return tagIdOf(current) ? { do: "tag" } : null;
    case "p":
      return pinIdOf(current) ? { do: "pin" } : null;
    case "k":
      return { do: "actions" };
  }
  return null;
}
