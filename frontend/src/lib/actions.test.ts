// Run with: node --test src/lib/actions.test.ts (Node strips the types itself)
import assert from "node:assert/strict";
import { test } from "node:test";
import { actionsFor, filterActions } from "./actions.ts";

const file = { kind: "row", row: {} };
const app = (actions) => ({ kind: "entry", entry: { ID: "app:x", Actions: actions } });
const fileActions = actionsFor(file);

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

test("a file always offers all three, an app only what its entry can do", () => {
  assert.deepEqual(actionsFor(file).map((a) => a.id), ["open", "reveal", "copy-path"]);
  assert.deepEqual(actionsFor(app(null)).map((a) => a.id), ["open"]);
  assert.deepEqual(actionsFor(app(["reveal", "admin"])).map((a) => a.id), ["open", "reveal", "admin"]);
  assert.deepEqual(actionsFor(app(["admin"])).map((a) => a.id), ["open", "admin"]);
});

test("every action names the shortcut that does the same without the panel", () => {
  for (const action of fileActions) assert.ok(action.keys.length > 0, action.id);
});
