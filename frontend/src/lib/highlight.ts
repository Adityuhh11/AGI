/** A stretch of text, and whether it is one of the words the user searched for. */
export type Part = { text: string; hit: boolean };

const WORD = /[\p{L}\p{N}]{2,}/gu;

// Little words the engine ignores too; emphasising them marks every sentence.
const LITTLE = new Set(
  "an as at be by do if in is it me my no of on or so to up we and are but can for how not the was who you your with what that this from into about where".split(" "),
);

/** The searched words (two letters or more, no little words, no repeats), lower case. */
export function words(query: string): string[] {
  return [...new Set((query.toLowerCase().match(WORD) ?? []).filter((word) => !LITTLE.has(word)))];
}

const escape = (word: string) => word.replace(/[.*+?^${}()|[\]\\]/g, "\\$&");

/**
 * Splits text so the words of query can be emphasised; the parts always join back to text. A word
 * counts where it starts a word ("token" marks the start of "tokenizer", never the middle of "Random").
 */
export function highlight(text: string, query: string): Part[] {
  const list = words(query);
  if (list.length === 0 || text === "") return [{ text, hit: false }];
  const alternatives = list.sort((a, b) => b.length - a.length).map(escape).join("|");
  const pattern = new RegExp(String.raw`(?<![\p{L}\p{N}])(${alternatives})`, "giu");
  return text
    .split(pattern)
    .filter((piece) => piece !== "")
    .map((piece) => ({ text: piece, hit: list.includes(piece.toLowerCase()) }));
}

/** Snippet text made fit to show: PDF extraction leaves private-use and control characters that render as boxes. */
export function tidy(text: string): string {
  return text.replace(/[\p{Co}\p{Cc}�]+/gu, " ").replace(/ {2,}/g, " ").trim();
}
