import { Events } from "@wailsio/runtime";
import { Service as Settings } from "../../bindings/omni/internal/settingsui";

/** The themes, in the order the picker lists them; `plate` and `text` are the two colours its swatch shows. Same names as `config.Themes`. */
export const THEMES = [
  { id: "black", label: "Black", plate: "#000000", text: "#eceef2" },
  { id: "midnight", label: "Midnight", plate: "#0a0f1e", text: "#e6ebf7" },
  { id: "forest", label: "Forest", plate: "#09140f", text: "#e4efe8" },
  { id: "cream", label: "Cream", plate: "#f6f0e2", text: "#2a2620" },
  { id: "blush", label: "Blush", plate: "#f8e8e6", text: "#33201f" },
  { id: "coral", label: "Coral", plate: "#f1c9c2", text: "#3a1512" },
] as const;

const KEY = "omni.theme";

const listeners = new Set<(id: string) => void>();

/** Calls `fn` with the theme's name whenever the page's theme changes; returns what stops it. */
export function onTheme(fn: (id: string) => void): () => void {
  listeners.add(fn);
  return () => listeners.delete(fn);
}

/** The theme the page wears now. */
export const currentTheme = () => document.documentElement.dataset.theme ?? "black";

/** Paints the page in a theme and remembers it, so the next window starts in it without waiting for the launcher. */
export function applyTheme(id: string) {
  if (!THEMES.some((t) => t.id === id)) return;
  const changed = currentTheme() !== id;
  document.documentElement.dataset.theme = id;
  if (changed) listeners.forEach((fn) => fn(id));
  try {
    localStorage.setItem(KEY, id);
  } catch {
    // no storage in this web view: the launcher is asked at every start instead
  }
}

/** Starts the page in the last theme it knew, asks the launcher for the real one and follows changes made in Settings. */
export function followTheme() {
  try {
    applyTheme(localStorage.getItem(KEY) ?? "black");
  } catch {
    applyTheme("black");
  }
  Settings.Theme().then(applyTheme, () => {});
  Events.On("ui:theme", (e) => applyTheme(e.data as string));
}
