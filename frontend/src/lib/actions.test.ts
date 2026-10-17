// Run with: node --test src/lib/actions.test.ts (Node strips the types itself)
import assert from "node:assert/strict";
import { test } from "node:test";
import { actionsFor as actionsOf, filterActions } from "./actions.ts";
import { tagIdOf } from "./flatten.ts";

const actionsFor = (item) => actionsOf(item, tagIdOf(item) !== null);

const file = { kind: "row", row: { Path: "a.js" } };
const app = (actions) => ({ kind: "entry", entry: { ID: "app:x", Kind: "app", Actions: actions } });
const fileActions = actionsFor(file);

const ids = (text: string) => filterActions(fileActions, text).map((a) => a.id);

test("no text lists every action, in order", () => {
  assert.deepEqual(ids(""), ["open", "reveal", "copy-path", "tag"]);
  assert.deepEqual(ids("   "), ["open", "reveal", "copy-path", "tag"]);
});

test("typing narrows by words, in any case", () => {
  assert.deepEqual(ids("COPY"), ["copy-path"]);
  assert.deepEqual(ids("show explor"), ["reveal"]);
  assert.deepEqual(ids("path copy"), ["copy-path"]);
});

test("a word that matches nothing leaves an empty list", () => {
  assert.deepEqual(ids("delete"), []);
});

test("a file offers everything, an app only what its entry can do, plus a tag", () => {
  assert.deepEqual(actionsFor(file).map((a) => a.id), ["open", "reveal", "copy-path", "tag"]);
  assert.deepEqual(actionsFor(app(null)).map((a) => a.id), ["open", "tag"]);
  assert.deepEqual(actionsFor(app(["reveal", "admin"])).map((a) => a.id), ["open", "reveal", "admin", "tag"]);
  assert.deepEqual(actionsFor(app(["admin"])).map((a) => a.id), ["open", "admin", "tag"]);
});

test("the web, a quicklink and a tag row cannot be tagged", () => {
  const other = (Kind) => ({ kind: "entry", entry: { ID: Kind + ":x", Kind, Actions: null } });
  for (const kind of ["web", "quicklink", "tag"]) assert.deepEqual(actionsFor(other(kind)).map((a) => a.id), ["open"], kind);
  assert.deepEqual(actionsFor(other("file")).map((a) => a.id), ["open", "tag"]);
});

test("every action names the shortcut that does the same without the panel", () => {
  for (const action of fileActions) assert.ok(action.keys.length > 0, action.id);
});
