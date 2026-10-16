// Run with: node --test src/lib/palettes.test.ts (Node strips the types itself)
import assert from "node:assert/strict";
import { test } from "node:test";
import { METALS, nextMetal } from "./palettes.ts";

test("the next metal is never the one before, and every other metal can come up", () => {
  for (let previous = 0; previous < METALS.length; previous++) {
    const seen = new Set();
    for (let step = 0; step < 1000; step++) {
      const next = nextMetal(previous, () => step / 1000);
      assert.ok(next >= 0 && next < METALS.length, `${next} is not a metal`);
      assert.notEqual(next, previous);
      seen.add(next);
    }
    assert.equal(seen.size, METALS.length - 1);
  }
});

test("the edges of the random range stay inside the list", () => {
  assert.notEqual(nextMetal(0, () => 0), 0);
  assert.ok(nextMetal(0, () => 0.999999) < METALS.length);
  assert.ok(nextMetal(METALS.length - 1, () => 0.999999) < METALS.length - 1);
});

test("every metal is two hex colours", () => {
  for (const metal of METALS) assert.match(`${metal.back}${metal.tint}`, /^(#[0-9a-f]{6}){2}$/i);
});
