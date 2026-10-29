/**
 * What a search covers. Tab cycles the chip through the first five; a prefix typed in the text (`app: code`) chooses
 * one too. `clip` (what was copied), `snip` (snippets), `note` (notes) and `ask` (Omni AI) are reached by their prefixes and their
 * pinned rows, not by Tab.
 */
export type Mode = "all" | "apps" | "files" | "web" | "tags" | "clip" | "snip" | "note" | "ask";

const MODES: readonly Mode[] = ["all", "apps", "files", "web", "tags"];

/** The mode after Tab (delta 1) or Shift+Tab (delta -1), wrapping round. With file search off, Tab skips Files. */
export function cycle(mode: Mode, delta: 1 | -1, fileSearch = true): Mode {
  const modes = fileSearch ? MODES : MODES.filter((m) => m !== "files");
  return modes[(modes.indexOf(mode) + delta + modes.length) % modes.length];
}

/** A search that names a kind of file (`code: retry`, `pdf: lecture`) asks for files whatever the mode; the names are search/scope.go's. */
export const asksForFiles = (text: string) => /^\s*(code|img|images?|vid|videos?|aud|audio|sounds?|pdfs?|docs?|text|notes)\s*:/i.test(text);

/** What the chip says; `all` shows no chip. */
export const label: Record<Mode, string> = { all: "", apps: "Apps", files: "Files", web: "Web", tags: "Tags", clip: "Clipboard", snip: "Snippets", note: "Notes", ask: "Ask" };

/** What Escape does: leave the mode first, then clear the text, and only then close the bar. */
export function escapeStep(mode: Mode, text: string): "mode" | "text" | "close" {
  if (mode !== "all") return "mode";
  return text !== "" ? "text" : "close";
}
