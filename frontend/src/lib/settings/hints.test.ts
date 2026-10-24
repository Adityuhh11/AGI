import assert from "node:assert/strict";
import { test } from "node:test";
import { choiceHints, paceHints, surveyLine } from "./hints.ts";

const estimate = { Complete: true, Seconds: 600, ByChoice: { "PDFPages:auto": 600, "PDFPages:all": 1500, "PDFPages:text": 120 }, ByPace: { fast: 300, balanced: 600, gentle: 1800 } };

test("each option says how long indexing would take with it", () => {
  assert.deepEqual(choiceHints("PDFPages", ["auto", "all", "text"], estimate), { auto: "about 10 min", all: "about 25 min", text: "about 2 min" });
  assert.deepEqual(paceHints(estimate), { fast: "about 5 min", balanced: "about 10 min", gentle: "about 30 min" });
});

test("nothing is promised until every folder is counted", () => {
  assert.deepEqual(choiceHints("PDFPages", ["auto"], { ...estimate, Complete: false }), {});
  assert.deepEqual(paceHints({ ...estimate, Complete: false }), {});
});

test("a folder says what it holds once it is counted, and that it is being counted before", () => {
  const folder = { Root: "C:\\a", State: "done", Files: 1204, Bytes: 3.2 * 1024 ** 3, Seconds: 1500 };
  assert.equal(surveyLine(folder), "1,204 files · 3.2 GB · about 25 min");
  assert.equal(surveyLine({ ...folder, State: "running" }), "Counting...");
  assert.equal(surveyLine({ ...folder, State: "unavailable" }), "");
  assert.equal(surveyLine(undefined), "");
});
