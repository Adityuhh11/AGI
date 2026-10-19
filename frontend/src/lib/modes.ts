/**
 * What a search covers. Tab cycles the chip through the first five; a prefix typed in the text (`app: code`) chooses
 * one too. `clip` (what was copied) is reached by its prefix, its pinned row and a hotkey, not by Tab.
 */
export type Mode = "all" | "apps" | "files" | "web" | "tags" | "clip";

const MODES: readonly Mode[] = ["all", "apps", "files", "web", "tags"];

/** The mode after Tab (delta 1) or Shift+Tab (delta -1), wrapping round. */
export function cycle(mode: Mode, delta: 1 | -1): Mode {
  return MODES[(MODES.indexOf(mode) + delta + MODES.length) % MODES.length];
}

/** What the chip says; `all` shows no chip. */
export const label: Record<Mode, string> = { all: "", apps: "Apps", files: "Files", web: "Web", tags: "Tags", clip: "Clipboard" };

/** What Escape does: leave the mode first, then clear the text, and only then close the bar. */
export function escapeStep(mode: Mode, text: string): "mode" | "text" | "close" {
  if (mode !== "all") return "mode";
  return text !== "" ? "text" : "close";
}
