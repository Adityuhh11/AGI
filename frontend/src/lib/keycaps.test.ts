import assert from "node:assert/strict";
import { test } from "node:test";
import { keycaps } from "./keycaps.ts";

test("an accelerator becomes one cap per key", () => {
  assert.deepEqual(keycaps("Alt+Shift+Space"), ["Alt", "Shift", "Space"]);
  assert.deepEqual(keycaps("Ctrl+,"), ["Ctrl", ","]);
  assert.deepEqual(keycaps("F9"), ["F9"]);
});

test("a plus key is a cap of its own", () => {
  assert.deepEqual(keycaps("Ctrl++"), ["Ctrl", "+"]);
  assert.deepEqual(keycaps(""), []);
});
