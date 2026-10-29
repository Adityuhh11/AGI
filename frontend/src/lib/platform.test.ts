import assert from "node:assert/strict";
import { test } from "node:test";
import { combo, keyLabel, words } from "./platform.ts";

test("keys are drawn as names on Windows and as symbols on a Mac", () => {
  assert.equal(keyLabel("Ctrl", false), "Ctrl");
  assert.equal(keyLabel("Ctrl", true), "⌃");
  assert.equal(keyLabel("Space", true), "Space");
  assert.equal(keyLabel("K", true), "K");
});

test("a combination is left alone on Windows and its symbols are set together on a Mac", () => {
  assert.equal(combo("Ctrl+Shift+Enter", false), "Ctrl+Shift+Enter");
  assert.equal(combo("Ctrl+Shift+Enter", true), "⌃⇧↩");
  assert.equal(combo("Alt+Space", true), "⌥Space");
  assert.equal(combo("Esc", true), "⎋");
  assert.equal(combo("Ctrl++", true), "⌃+");
});

test("a Mac says Finder and the system's default browser", () => {
  assert.equal(words(true).fileManager, "Finder");
  assert.equal(words(false).fileManager, "Explorer");
  assert.ok(!words(true).defaultBrowser.includes("Windows"));
});
