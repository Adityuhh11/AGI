// Run with: node --test src/lib/flatten.test.ts (Node strips the types itself)
import assert from "node:assert/strict";
import { test } from "node:test";
import { flatten, moreCount, openLine, pathOf, step } from "./flatten.ts";

const match = (location, line) => ({ Location: location, Snippet: "", Line: line });
const row = (path, matches, line = 0) => ({
  Path: path, Name: path, Folder: "", Kind: "code", Line: line, Score: 1, Matches: matches,
});

const a = row("a.js", [match("lines 1-40", 1), match("lines 31-70", 31), match("lines 91-130", 91)], 1);
const b = row("b.md", [match("lines 1-9", 1)], 1);
const c = row("c.png", null);

test("a collapsed list shows one line per file", () => {
  const items = flatten([a, b, c], new Set());
  assert.deepEqual(items.map((i) => i.kind), ["row", "row", "row"]);
});

test("an expanded file is followed by its matches after the first", () => {
  const items = flatten([a, b], new Set(["a.js"]));
  assert.deepEqual(items.map((i) => i.kind), ["row", "match", "match", "row"]);
  assert.equal(items[1].match.Line, 31);
  assert.equal(items[2].match.Line, 91);
  assert.equal(items[2].row, a);
});

test("expanding a file with a single match adds nothing", () => {
  assert.equal(flatten([b], new Set(["b.md"])).length, 1);
  assert.equal(flatten([c], new Set(["c.png"])).length, 1);
});

test("moreCount counts matches beyond the first and tolerates none", () => {
  assert.equal(moreCount(a), 2);
  assert.equal(moreCount(b), 0);
  assert.equal(moreCount(c), 0);
});

test("step stays inside the list, also when it is empty", () => {
  assert.equal(step(0, 1, 3), 1);
  assert.equal(step(2, 1, 3), 2);
  assert.equal(step(0, -1, 3), 0);
  assert.equal(step(0, 1, 0), 0);
  assert.equal(step(5, 0, 3), 2);
});

test("a selected match opens at its own line, a selected file at its best match", () => {
  const items = flatten([a], new Set(["a.js"]));
  assert.equal(openLine(items[0]), 1);
  assert.equal(openLine(items[2]), 91);
  assert.equal(pathOf(items[2]), "a.js");
});
