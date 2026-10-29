/** Whether the page runs in a Mac's web view. The pages are the same on both systems; only how keys are drawn and a few words differ. */
export const isMac = typeof navigator !== "undefined" && /Macintosh|Mac OS X/.test(navigator.userAgent);

// How a key is drawn on a Mac's keyboard. Anything not here is drawn as it is named.
const MAC_KEYS: Record<string, string> = {
  Ctrl: "⌃", Alt: "⌥", Shift: "⇧", Super: "⌘", Enter: "↩", Backspace: "⌫", Delete: "⌦", Esc: "⎋", Tab: "⇥",
  Left: "←", Right: "→", Up: "↑", Down: "↓",
};

/** One key as its cap shows it: the name on Windows, the symbol on a Mac. */
export function keyLabel(key: string, mac = isMac): string {
  return mac ? (MAC_KEYS[key] ?? key) : key;
}

/** A combination written `Ctrl+Shift+C` as it is drawn: unchanged on Windows, `⌃⇧C` on a Mac (symbols sit together, as they do in menus). */
export function combo(text: string, mac = isMac): string {
  if (!mac) return text;
  const keys = text.split("+");
  const joined: string[] = [];
  for (let i = 0; i < keys.length; i++) {
    if (keys[i] === "" && i < keys.length - 1 && keys[i + 1] === "") {
      joined.push("+");
      i++;
    } else if (keys[i] !== "") {
      joined.push(keyLabel(keys[i], true));
    }
  }
  return joined.join("");
}

/** Words that differ by system. */
export function words(mac = isMac) {
  return {
    fileManager: mac ? "Finder" : "Explorer",
    defaultBrowser: mac ? "Default browser" : "Windows default browser",
    modifiers: mac ? "⌃, ⌥, ⇧ or ⌘" : "Ctrl, Alt, Shift or the Windows key",
  };
}
