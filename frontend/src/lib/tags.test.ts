// Run with: node --test src/lib/tags.test.ts (Node strips the types itself)
import assert from "node:assert/strict";
import { test } from "node:test";
import { newTag, tagLines } from "./tags.ts";

const all = [{ Name: "work", Count: 2 }, { Name: "home", Count: 1 }];

test("every tag is a line, ticked when the thing carries it", () => {
  assert.deepEqual(tagLines(all, ["home"]), [
    { id: "work", title: "#work", keys: "" },
    { id: "home", title: "#home", keys: "✓" },
  ]);
});

test("a new name offers to create it; an existing one, in any case, or nothing does not", () => {
  assert.deepEqual(newTag("#Travel ", all), { id: "new:Travel", title: "Add the tag “Travel”" });
  assert.equal(newTag("WORK", all), null);
  assert.equal(newTag("#", all), null);
  assert.equal(newTag("", all), null);
});
