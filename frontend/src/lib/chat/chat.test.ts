import assert from "node:assert/strict";
import { test } from "node:test";
import { dot, escapeStep, kilo, lastQuestion, meter, sends, thoughtFor, unloadsIn, waiting } from "./chat.ts";

const info = (state: string, extra: object = {}) => ({ enabled: true, installed: true, state, message: "", device: "gpu", ...extra });

test("the dot says off, loading, ready, failed or not loaded", () => {
  assert.equal(dot(null).label, "");
  assert.equal(dot({ ...info("ready"), enabled: false }).label, "Omni AI is off");
  assert.equal(dot({ ...info("ready"), installed: false }).tone, "off");
  assert.deepEqual(dot(info("ready")), { tone: "ready", label: "Ready", busy: false });
  assert.equal(dot(info("ready", { device: "cpu" })).label, "Ready, on the processor");
  assert.deepEqual(dot(info("loading")), { tone: "busy", label: "Starting the model…", busy: true });
  assert.deepEqual(dot(info("failed", { message: "no card" })), { tone: "bad", label: "no card", busy: false });
  assert.equal(dot(info("failed")).label, "The model did not start");
  assert.equal(dot(info("unloaded")).tone, "off");
});

test("tokens are written short", () => {
  assert.equal(kilo(0), "0");
  assert.equal(kilo(640), "640");
  assert.equal(kilo(999), "999");
  assert.equal(kilo(1000), "1k");
  assert.equal(kilo(3100), "3.1k");
  assert.equal(kilo(8192), "8.2k");
  assert.equal(meter(3100, 8192), "3.1k / 8.2k");
  assert.equal(meter(0, 4096), "0 / 4.1k");
});

test("thinking time is in seconds, then minutes", () => {
  assert.equal(thoughtFor(100), "Thought for 1 s");
  assert.equal(thoughtFor(4200), "Thought for 4 s");
  assert.equal(thoughtFor(61_000), "Thought for 1 min 1 s");
});

test("the unload time counts up to the minute and says nothing when the model stays", () => {
  const now = 1_000_000;
  assert.equal(unloadsIn(0, now), "");
  assert.equal(unloadsIn(now + 7 * 60_000 - 1, now), "unloads in 7 min");
  assert.equal(unloadsIn(now + 5_000, now), "unloads in 1 min");
  assert.equal(unloadsIn(now - 5_000, now), "unloads in 1 min");
});

test("Up recalls the last question, not the last answer", () => {
  assert.equal(lastQuestion(null), "");
  assert.equal(lastQuestion([]), "");
  assert.equal(
    lastQuestion([
      { role: "user", text: "first" },
      { role: "assistant", text: "a" },
      { role: "user", text: "second" },
      { role: "assistant", text: "b" },
    ]),
    "second",
  );
});

test("Escape stops an answer, then closes the history, then leaves", () => {
  assert.equal(escapeStep(true, true, "x"), "stop");
  assert.equal(escapeStep(false, true, ""), "history");
  assert.equal(escapeStep(false, false, "  "), "close");
  assert.equal(escapeStep(false, false, "half a question"), "none"); // a typed question is not thrown away by a stray Escape
});

test("Enter sends unless Shift is held, an input method is composing, or the box is empty", () => {
  const key = (extra: object = {}) => ({ key: "Enter", shiftKey: false, isComposing: false, ...extra });
  assert.equal(sends(key(), "hello"), true);
  assert.equal(sends(key({ shiftKey: true }), "hello"), false);
  assert.equal(sends(key({ isComposing: true }), "hello"), false);
  assert.equal(sends(key(), "   "), false);
  assert.equal(sends(key({ key: "a" }), "hello"), false);
});

test("the waiting line prefers what the program said", () => {
  assert.equal(waiting("loading", "The graphics card ran out of memory; starting on the processor…"), "The graphics card ran out of memory; starting on the processor…");
  assert.equal(waiting("loading", ""), "Starting the model…");
  assert.equal(waiting("thinking", ""), "Thinking…");
  assert.equal(waiting("reading", ""), "Reading…");
  assert.equal(waiting("", ""), "Reading…");
});
