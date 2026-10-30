// Run with: node --test src/lib/palettes.test.ts (Node strips the types itself)
import assert from "node:assert/strict";
import { test } from "node:test";
import { METALS, nextMetal, ringsFor } from "./palettes.ts";

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

test("every theme has its own rings, the next one is chosen among that theme's, and an unknown theme wears the metals", () => {
  for (const theme of ["black", "midnight", "forest", "cream", "blush", "coral"]) {
    const rings = ringsFor(theme);
    assert.ok(rings.length >= 2, theme);
    for (const ring of rings) assert.match(`${ring.back}${ring.tint}`, /^(#[0-9a-f]{6}){2}$/i);
    for (let previous = 0; previous < rings.length; previous++) {
      const next = nextMetal(previous, () => 0.999999, rings.length);
      assert.ok(next >= 0 && next < rings.length && next !== previous, `${theme}: ${previous} -> ${next}`);
    }
  }
  assert.equal(ringsFor("black"), METALS);
  assert.equal(ringsFor("neon"), METALS);
  assert.notEqual(ringsFor("cream")[0].back, METALS[0].back);
});
