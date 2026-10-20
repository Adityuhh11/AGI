// Run with: node --test src/lib/keys.test.ts (Node strips the types itself)
import assert from "node:assert/strict";
import { test } from "node:test";
import { keyCommand } from "./keys.ts";

const file = { kind: "row", row: { Path: "C:\\a.txt", Matches: [] } };
const clip = { kind: "entry", entry: { ID: "clip:1", Kind: "clip" } };
const web = { kind: "entry", entry: { ID: "web:x", Kind: "web" } };
const press = (key, mods = {}, ctx = {}) =>
  keyCommand({ key, ctrlKey: false, shiftKey: false, altKey: false, ...mods }, { current: file, atEndOfText: true, expanded: false, ...ctx });

test("Enter opens; Ctrl reveals a file or copies a copied text; Ctrl+Shift runs as administrator; Alt searches the web", () => {
  assert.deepEqual(press("Enter"), { do: "run", how: "open" });
  assert.deepEqual(press("Enter", { ctrlKey: true }), { do: "run", how: "reveal" });
  assert.deepEqual(press("Enter", { ctrlKey: true }, { current: clip }), { do: "run", how: "copy" });
  assert.deepEqual(press("Enter", { ctrlKey: true, shiftKey: true }), { do: "run", how: "admin" });
  assert.deepEqual(press("Enter", { altKey: true }), { do: "web" });
});

test("Tab cycles modes unless Ctrl or Alt is held; arrows move", () => {
  assert.deepEqual(press("Tab"), { do: "mode", by: 1 });
  assert.deepEqual(press("Tab", { shiftKey: true }), { do: "mode", by: -1 });
  assert.equal(press("Tab", { ctrlKey: true }), null);
  assert.deepEqual(press("ArrowUp"), { do: "move", by: -1 });
});

test("right and left expand and collapse a file only with the cursor at the end of the text", () => {
  assert.deepEqual(press("ArrowRight"), { do: "expand" });
  assert.equal(press("ArrowRight", {}, { atEndOfText: false }), null);
  assert.equal(press("ArrowLeft"), null);
  assert.deepEqual(press("ArrowLeft", {}, { expanded: true }), { do: "collapse" });
});

test("Ctrl shortcuts need a row that supports them", () => {
  assert.deepEqual(press("C", { ctrlKey: true, shiftKey: true }), { do: "copy-path" });
  assert.equal(press("c", { ctrlKey: true }), null);
  assert.deepEqual(press("t", { ctrlKey: true }), { do: "tag" });
  assert.equal(press("t", { ctrlKey: true }, { current: web }), null);
  assert.deepEqual(press("P", { ctrlKey: true }), { do: "pin" });
  assert.deepEqual(press("k", { ctrlKey: true }), { do: "actions" });
  assert.equal(press("k", { ctrlKey: true }, { current: undefined }), null);
  assert.deepEqual(press("Delete", { ctrlKey: true }, { current: clip }), { do: "run", how: "delete" });
  assert.equal(press("Delete", { ctrlKey: true }), null);
  assert.deepEqual(press(",", { ctrlKey: true }, { current: undefined }), { do: "settings" });
  assert.equal(press("x"), null);
});
