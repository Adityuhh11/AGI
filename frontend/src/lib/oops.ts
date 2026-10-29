/**
 * The safety net for every window: a script error nobody expected shows a short message in the window instead of
 * leaving it blank or stuck. The error itself goes to the console; what the person sees is in plain words.
 */
import { Service as Status } from "../../bindings/omni/internal/status";

/** What went wrong as text: an error's stack when it has one. */
function describe(error: unknown): string {
  if (error instanceof Error) return error.stack ?? error.message;
  return String(error);
}

export function catchUnexpected() {
  let shown: HTMLElement | null = null;

  const show = (error: unknown) => {
    // A browser notice, not a failure: layout settled over two frames instead of one.
    if (typeof error === "string" && error.startsWith("ResizeObserver loop")) return;
    console.error(error);
    Status.PageError(describe(error)).catch(() => {}); // the log is where the message says the details are
    if (shown) return;
    const note = document.createElement("div");
    note.setAttribute("role", "alert");
    note.style.cssText =
      "position:fixed;left:12px;right:12px;bottom:12px;z-index:99;display:flex;align-items:center;gap:12px;padding:10px 12px;" +
      "border:1px solid var(--line);border-radius:10px;background:var(--plate);color:var(--danger);font:500 12.5px/1.4 var(--sans)";
    const text = document.createElement("span");
    text.style.flex = "1";
    text.textContent = "Something went wrong. Try again; if it keeps happening, the details are in the log (Settings, About).";
    const dismiss = document.createElement("button");
    dismiss.type = "button";
    dismiss.textContent = "Dismiss";
    dismiss.style.cssText = "border:0;background:transparent;color:var(--text-2);font:inherit;cursor:pointer";
    dismiss.onclick = () => {
      note.remove();
      shown = null;
    };
    note.append(text, dismiss);
    document.body.append(note);
    shown = note;
  };

  window.addEventListener("error", (e) => show(e.error ?? e.message));
  window.addEventListener("unhandledrejection", (e) => show(e.reason));
}
