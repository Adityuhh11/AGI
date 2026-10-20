// Run with: node --test src/lib/notes.test.ts (Node strips the types itself)
import assert from "node:assert/strict";
import { test } from "node:test";
import { notesFor } from "./notes.ts";

const base = {
  info: null,
  problem: "",
  query: "budget",
  effective: "all",
  entries: 0,
  local: 0,
  answerNote: "",
  results: null,
  fileRows: 0,
  answered: "",
};
const texts = (over) => notesFor({ ...base, ...over }).map((n) => [n.text, n.problem]);
const results = (over) => ({ State: "results", Message: "", Confident: true, ...over });

test("a hotkey error and a failed action come first, as problems", () => {
  assert.deepEqual(texts({ info: { HotkeyError: "Alt+Space is taken" }, problem: "could not open" }), [
    ["Alt+Space is taken", true],
    ["could not open", true],
  ]);
});

test("an empty bar only reports an engine that failed", () => {
  assert.deepEqual(texts({ query: " ", info: { Engine: "failed", EngineMessage: "no GPU" } }), [["no GPU", true]]);
  assert.deepEqual(texts({ query: "", results: results({ State: "warming", Message: "starting" }) }), []);
});

test("each mode explains its own empty list", () => {
  assert.deepEqual(texts({ effective: "tags" }), [["Nothing is tagged like that yet. Select a row and press Ctrl+T.", false]]);
  assert.deepEqual(texts({ effective: "clip", answerNote: "History is off" }), [["History is off", false]]);
  assert.deepEqual(texts({ effective: "apps", query: "app: zed" }), [["No app matched “zed”", false]]);
});

test("file search states: warming, off, failed", () => {
  assert.deepEqual(texts({ results: results({ State: "warming", Message: "starting" }) }), [["starting", false]]);
  assert.deepEqual(texts({ results: results({ State: "off", Message: "off" }) }), [["off", false]]);
  assert.deepEqual(texts({ results: results({ State: "off", Message: "off" }), local: 1 }), []);
  assert.deepEqual(texts({ results: results({ State: "error", Message: "boom" }) }), [["boom", true]]);
});

test("answers: nothing matched, or only weak matches", () => {
  assert.deepEqual(texts({ results: results({}), answered: "budget" }), [["Nothing matched “budget”", false]]);
  assert.deepEqual(texts({ results: results({ Message: "Searching code." }) }), [["Searching code.", false]]);
  assert.deepEqual(texts({ results: results({ Confident: false }), fileRows: 3 }), [["No strong match. The closest files:", false]]);
  assert.deepEqual(texts({ results: results({ Confident: false }), fileRows: 3, local: 1 }), []);
});
