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

test("a row that can be pinned offers Pin or Unpin last, with its shortcut; one that cannot offers neither", () => {
  assert.deepEqual(actionsOf(file, true, "pin").map((a) => [a.id, a.title, a.keys]).at(-1), ["pin", "Pin", "Ctrl+P"]);
  assert.deepEqual(actionsOf(file, true, "unpin").map((a) => [a.id, a.title]).at(-1), ["pin", "Unpin"]);
  assert.deepEqual(actionsOf(file, true).map((a) => a.id), ["open", "reveal", "copy-path", "tag"]);
  const link = { kind: "entry", entry: { ID: "quicklink:gh:", Kind: "quicklink", Actions: null } };
  assert.deepEqual(actionsOf(link, false, "pin").map((a) => a.id), ["open", "pin"]);
});

test("a copied text offers Paste first, then Copy and Delete, and cannot be tagged or revealed", () => {
  const clip = { kind: "entry", entry: { ID: "clip:7", Kind: "clip", Title: "x", Actions: ["copy", "delete"] } };
  assert.deepEqual(actionsOf(clip, false).map((a) => [a.id, a.title, a.keys]), [
    ["open", "Paste", "Enter"],
    ["copy", "Copy", "Ctrl+Enter"],
    ["delete", "Delete this entry", "Ctrl+Delete"],
  ]);
});

test("a snippet offers Paste and can be pinned; a built-in row opens and can be pinned", () => {
  const snippet = { kind: "entry", entry: { ID: "snippet:A", Kind: "snippet", Actions: null } };
  assert.deepEqual(actionsOf(snippet, false, "pin").map((a) => [a.id, a.title]), [["open", "Paste"], ["pin", "Pin"]]);
  assert.deepEqual(actionsOf(snippet, false, "unpin").map((a) => a.title), ["Paste", "Unpin"]);
  const command = { kind: "entry", entry: { ID: "cmd:snippets", Kind: "cmd", Actions: null } };
  assert.deepEqual(actionsOf(command, false, "unpin").map((a) => [a.id, a.title]), [["open", "Open"], ["pin", "Unpin"]]);
});
