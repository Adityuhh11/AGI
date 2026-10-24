import assert from "node:assert/strict";
import { test } from "node:test";
import { after, before, stepsFor } from "./flow.ts";

test("file search adds the folder and option screens", () => {
  assert.deepEqual(stepsFor(true), ["welcome", "keys", "files", "folders", "options", "done"]);
  assert.deepEqual(stepsFor(false), ["welcome", "keys", "files", "done"]);
});

test("walking forward and back stays inside the screens", () => {
  assert.equal(after("files", true), "folders");
  assert.equal(after("files", false), "done");
  assert.equal(after("done", true), "done");
  assert.equal(before("done", false), "files");
  assert.equal(before("welcome", true), "welcome");
});
