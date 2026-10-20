import type { TagInfo } from "../../bindings/agi/internal/palette";

/** The tag picker's lines: every tag, ticked when the thing carries it. */
export function tagLines(all: TagInfo[], applied: string[]) {
  return all.map((tag) => ({ id: tag.Name, title: "#" + tag.Name, keys: applied.includes(tag.Name) ? "✓" : "" }));
}

/** The line that creates the tag typed in the picker, or null if nothing is typed or the tag exists. */
export function newTag(typed: string, all: TagInfo[]) {
  const name = typed.replace(/^#/, "").trim();
  if (!name || all.some((tag) => tag.Name === name.toLowerCase())) return null;
  return { id: "new:" + name, title: `Add the tag “${name}”` };
}
