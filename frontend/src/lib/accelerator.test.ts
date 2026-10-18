// Run with: node --test src/lib/accelerator.test.ts (Node strips the types itself)
import assert from "node:assert/strict";
import { test } from "node:test";
import { capture } from "./accelerator.ts";

const press = (code: string, mods: Partial<{ ctrlKey: boolean; altKey: boolean; shiftKey: boolean; metaKey: boolean }> = {}) =>
  capture({ code, ctrlKey: false, altKey: false, shiftKey: false, metaKey: false, ...mods });

test("modifiers come first in a fixed order, then the key", () => {
  assert.deepEqual(press("Space", { shiftKey: true, altKey: true }), { accelerator: "Alt+Shift+Space" });
  assert.deepEqual(press("KeyK", { metaKey: true, ctrlKey: true }), { accelerator: "Ctrl+Super+K" });
});

test("letters, digits and function keys keep their plain names", () => {
  assert.deepEqual(press("KeyJ", { ctrlKey: true }), { accelerator: "Ctrl+J" });
  assert.deepEqual(press("Digit7", { altKey: true }), { accelerator: "Alt+7" });
  assert.deepEqual(press("F9", { ctrlKey: true, altKey: true }), { accelerator: "Ctrl+Alt+F9" });
  assert.deepEqual(press("F24", { ctrlKey: true }), { accelerator: "Ctrl+F24" });
});

test("named and punctuation keys use the names the launcher understands", () => {
  assert.deepEqual(press("PageUp", { ctrlKey: true }), { accelerator: "Ctrl+Page Up" });
  assert.deepEqual(press("ArrowLeft", { altKey: true }), { accelerator: "Alt+Left" });
  assert.deepEqual(press("Comma", { ctrlKey: true }), { accelerator: "Ctrl+," });
  assert.deepEqual(press("Backquote", { ctrlKey: true }), { accelerator: "Ctrl+`" });
});

test("pressing only modifiers waits for the rest of the combination", () => {
  assert.equal(press("ControlLeft", { ctrlKey: true }), null);
  assert.equal(press("ShiftRight", { shiftKey: true }), null);
  assert.equal(press("MetaLeft", { metaKey: true }), null);
});

test("a key with no modifier is refused with a hint", () => {
  assert.deepEqual(press("KeyK"), { hint: "Hold Ctrl, Alt, Shift or the Windows key as well." });
});

test("a key the launcher has no name for is refused with a hint", () => {
  assert.deepEqual(press("AudioVolumeUp", { ctrlKey: true }), { hint: "That key cannot be part of a hotkey." });
  assert.deepEqual(press("F25", { ctrlKey: true }), { hint: "That key cannot be part of a hotkey." });
});
