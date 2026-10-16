// Run with: node --test src/lib/path.test.ts (Node strips the types itself)
import assert from "node:assert/strict";
import { test } from "node:test";
import { shortFolder } from "./path.ts";

test("a short path is shown whole, as folders", () => {
  assert.equal(shortFolder("C:\\Users\\me"), "C: › Users › me");
});

test("a long path keeps its last three folders", () => {
  assert.equal(shortFolder("C:\\Users\\me\\OneDrive\\Documents\\Prob and stats"), "… › OneDrive › Documents › Prob and stats");
});

test("forward slashes and doubled separators are handled", () => {
  assert.equal(shortFolder("C:/dev//VS_CODE/AGI/Sample Data/NextPlore"), "… › AGI › Sample Data › NextPlore");
});

test("an empty path is empty", () => {
  assert.equal(shortFolder(""), "");
});
