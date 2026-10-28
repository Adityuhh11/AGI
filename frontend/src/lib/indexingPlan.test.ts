import assert from "node:assert/strict";
import { test } from "node:test";
import { changes, effects, type Choices } from "./indexingPlan.ts";

const saved: Choices = { PDFPages: "auto", Photos: "content", Videos: "name", Audio: "name" };

test("nothing differs, nothing to save", () => {
  assert.deepEqual(changes(saved, { ...saved }), []);
});

test("each kind says whether the new choice costs more to index", () => {
  const list = changes(saved, { PDFPages: "all", Photos: "name", Videos: "name", Audio: "name" });
  assert.deepEqual(
    list.map((c) => [c.kind, c.costlier]),
    [
      ["PDFPages", true],
      ["Photos", false],
    ],
  );
  assert.equal(changes(saved, { ...saved, PDFPages: "text" })[0].costlier, false);
  assert.equal(changes(saved, { ...saved, Videos: "content" })[0].costlier, true);
  assert.equal(changes(saved, { ...saved, Audio: "content" })[0].costlier, true);
});

test("a dearer choice names the time on this device, a cheaper one computes nothing", () => {
  const list = changes(saved, { PDFPages: "all", Photos: "name", Videos: "name", Audio: "name" });
  assert.match(effects(list, "vulkan")[0], /3 minutes per 1,000 pictures on the graphics card/);
  assert.match(effects(list, "cpu")[0], /80 minutes per 1,000 pictures on the processor/);
  assert.match(effects(list, "vulkan")[1], /Nothing is computed/);
});

test("sound is priced per hour of sound, not per picture, and says what going back removes", () => {
  const dearer = effects(changes(saved, { ...saved, Audio: "content" }), "vulkan")[0];
  assert.match(dearer, /Audio and sound: .*about 1 minute per hour of sound on the graphics card/);
  assert.match(effects(changes(saved, { ...saved, Audio: "content" }), "cpu")[0], /about 15 minutes per hour of sound on the processor/);
  const cheaper = effects(changes({ ...saved, Audio: "content" }, saved), "cpu")[0];
  assert.match(cheaper, /sound you no longer search by is removed from the index\. Nothing is computed/);
});

