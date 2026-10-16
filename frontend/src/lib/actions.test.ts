// Run with: node --test src/lib/actions.test.ts (Node strips the types itself)
import assert from "node:assert/strict";
import { test } from "node:test";
import { fileActions, filterActions } from "./actions.ts";

const ids = (text: string) => filterActions(fileActions, text).map((a) => a.id);

test("no text lists every action, in order", () => {
  assert.deepEqual(ids(""), ["open", "reveal", "copy-path"]);
  assert.deepEqual(ids("   "), ["open", "reveal", "copy-path"]);
});

test("typing narrows by words, in any case", () => {
  assert.deepEqual(ids("COPY"), ["copy-path"]);
  assert.deepEqual(ids("show explor"), ["reveal"]);
  assert.deepEqual(ids("path copy"), ["copy-path"]);
});

test("a word that matches nothing leaves an empty list", () => {
  assert.deepEqual(ids("delete"), []);
});

test("every action names the shortcut that does the same without the panel", () => {
  for (const action of fileActions) assert.ok(action.keys.length > 0, action.id);
});
