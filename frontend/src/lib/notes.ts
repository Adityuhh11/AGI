import type { Results } from "../../bindings/omni/internal/search";
import type { Info } from "../../bindings/omni/internal/status";
import type { Mode } from "./modes";
import { combo } from "./platform.ts";

/** One line shown above the results; a problem is shown in the danger colour. */
export type Note = { text: string; problem: boolean };

/** What the bar knows when it decides which notes to show. */
export type NoteState = {
  info: Info | null;
  problem: string; // the last action that failed, if any
  query: string;
  effective: Mode;
  entries: number; // instant answers listed
  local: number; // instant answers about this machine (not the web search)
  answerNote: string;
  results: Results | null;
  fileRows: number;
  answered: string; // the query `results` answers
};

/** The notes above the results, one line each, most urgent first. */
export function notesFor(s: NoteState): Note[] {
  const out: Note[] = [];
  const say = (text: string, problem = false) => out.push({ text, problem });
  if (s.info?.HotkeyError) say(s.info.HotkeyError, true);
  if (s.problem) say(s.problem, true);
  const nothing = s.fileRows + s.local === 0;
  if (s.query.trim() === "") {
    if (s.info?.Engine === "failed") say(s.info.EngineMessage, true);
  } else if (s.effective === "tags") {
    if (s.entries === 0) say(`Nothing is tagged like that yet. Select a row and press ${combo("Ctrl+T")}.`);
  } else if (s.effective === "clip" || s.effective === "snip" || s.effective === "note" || s.effective === "ask") {
    if (s.answerNote) say(s.answerNote);
  } else if (s.effective === "apps") {
    if (s.local === 0) say(`No app matched “${s.query.replace(/^\s*apps?:/i, "").trim()}”`);
  } else if (s.results?.State === "warming") {
    say(s.results.Message);
  } else if (s.results?.State === "off") {
    if (nothing) say(s.results.Message);
  } else if (s.results?.State === "unavailable" || s.results?.State === "error") {
    say(s.results.Message, true);
  } else if (s.results) {
    if (nothing && s.results.Message) say(s.results.Message);
    else if (nothing && s.answered.trim() !== "") say(`Nothing matched “${s.answered}”`);
    else if (!s.results.Confident && s.local === 0) say("No strong match. The closest files:");
  }
  return out;
}
