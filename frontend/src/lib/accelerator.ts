/** Turns a key press into the accelerator text the launcher registers (`Ctrl+Alt+K`, `Alt+Shift+Space`). */
export type KeyEventLike = Pick<KeyboardEvent, "code" | "ctrlKey" | "altKey" | "shiftKey" | "metaKey">;

// The names the Go side (Wails) accepts for keys that are not a letter, a digit or a function key.
const NAMED: Record<string, string> = {
  Space: "Space", Enter: "Enter", NumpadEnter: "Enter", Tab: "Tab", Backspace: "Backspace", Delete: "Delete",
  Home: "Home", End: "End", PageUp: "Page Up", PageDown: "Page Down",
  ArrowLeft: "Left", ArrowRight: "Right", ArrowUp: "Up", ArrowDown: "Down",
  Minus: "-", Equal: "=", BracketLeft: "[", BracketRight: "]", Backslash: "\\", Semicolon: ";",
  Quote: "'", Comma: ",", Period: ".", Slash: "/", Backquote: "`",
};

const MODIFIER_CODES = /^(Control|Alt|Shift|Meta|OS)(Left|Right)?$/;

function keyName(code: string): string | null {
  if (/^Key[A-Z]$/.test(code)) return code.slice(3);
  if (/^Digit\d$/.test(code)) return code.slice(5);
  if (/^F([1-9]|1\d|2[0-4])$/.test(code)) return code;
  return NAMED[code] ?? null;
}

/**
 * The accelerator for a finished combination, a `hint` saying why a press cannot be a hotkey, or `null`
 * while only modifier keys are down (the person is still reaching for the last key).
 */
export function capture(e: KeyEventLike): { accelerator: string } | { hint: string } | null {
  if (MODIFIER_CODES.test(e.code)) return null;
  const key = keyName(e.code);
  if (key === null) return { hint: "That key cannot be part of a hotkey." };
  const modifiers = [e.ctrlKey && "Ctrl", e.altKey && "Alt", e.shiftKey && "Shift", e.metaKey && "Super"].filter(Boolean);
  if (modifiers.length === 0) return { hint: "Hold Ctrl, Alt, Shift or the Windows key as well." };
  return { accelerator: [...modifiers, key].join("+") };
}
