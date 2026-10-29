// Run with: node --test src/lib/path.test.ts (Node strips the types itself)
import assert from "node:assert/strict";
import { test } from "node:test";
import { isInside, shortFolder } from "./path.ts";

test("a short path is shown whole, as folders", () => {
  assert.equal(shortFolder("C:\\Users\\me"), "C: › Users › me");
});

test("a long path keeps its last three folders", () => {
  assert.equal(shortFolder("C:\\Users\\me\\OneDrive\\Documents\\Prob and stats"), "… › OneDrive › Documents › Prob and stats");
});

test("forward slashes and doubled separators are handled", () => {
  assert.equal(shortFolder("C:/dev//VS_CODE/Omni/Sample Data/NextPlore"), "… › Omni › Sample Data › NextPlore");
});

test("an empty path is empty", () => {
  assert.equal(shortFolder(""), "");
});

test("a folder is inside a root when it is the root or below it, whatever the case", () => {
  assert.equal(isInside("C:\\Users\\me\\Docs", "c:\\users\\ME\\docs\\work"), true);
  assert.equal(isInside("C:\\Users\\me\\Docs\\", "C:\\Users\\me\\Docs"), true);
  assert.equal(isInside("/Users/me/Docs", "/Users/me/Docs/node_modules"), true);
  assert.equal(isInside("/Users/me/Docs/", "/Users/me/Docs"), true);
  assert.equal(isInside("/Users/me/Docs", "/Users/me/DocsExtra"), false);
  assert.equal(isInside("C:\\Users\\me\\Docs", "C:\\Users\\me\\Docs2"), false);
  assert.equal(isInside("C:\\a", "D:\\a"), false);
});
