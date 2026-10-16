// Run with: node --test src/lib/modes.test.ts (Node strips the types itself)
import assert from "node:assert/strict";
import { test } from "node:test";
import { cycle, escapeStep, label } from "./modes.ts";

test("Tab walks the modes and wraps, Shift+Tab walks back", () => {
  assert.equal(cycle("all", 1), "apps");
  assert.equal(cycle("apps", 1), "files");
  assert.equal(cycle("files", 1), "web");
  assert.equal(cycle("web", 1), "tags");
  assert.equal(cycle("tags", 1), "all");
  assert.equal(cycle("all", -1), "tags");
  assert.equal(cycle("apps", -1), "all");
});

test("Escape leaves the mode, then clears the text, then closes", () => {
  assert.equal(escapeStep("apps", "chrome"), "mode");
  assert.equal(escapeStep("files", ""), "mode");
  assert.equal(escapeStep("all", "chrome"), "text");
  assert.equal(escapeStep("all", ""), "close");
});

test("the all mode shows no chip and the others name themselves", () => {
  assert.equal(label.all, "");
  assert.equal(label.apps, "Apps");
  assert.equal(label.files, "Files");
  assert.equal(label.web, "Web");
  assert.equal(label.tags, "Tags");
});
