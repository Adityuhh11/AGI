// The small decisions of the chat window that need no page: what the status dot says, how tokens and times are written, and
// which message a key recalls. Kept apart so they are tested with node.

/** The model's state as the header's dot shows it. `tone` picks the colour; `busy` makes the dot pulse. */
export type Dot = { tone: "off" | "ready" | "busy" | "bad"; label: string; busy: boolean };

export type ModelInfo = { enabled: boolean; installed: boolean; state: string; message: string; device: string };

export function dot(info: ModelInfo | null): Dot {
  if (!info) return { tone: "off", label: "", busy: false };
  if (!info.enabled || !info.installed) return { tone: "off", label: "Omni AI is off", busy: false };
  switch (info.state) {
    case "ready":
      return { tone: "ready", label: info.device === "cpu" ? "Ready, on the processor" : "Ready", busy: false };
    case "loading":
      return { tone: "busy", label: "Starting the model…", busy: true };
    case "failed":
      return { tone: "bad", label: info.message || "The model did not start", busy: false };
    default:
      return { tone: "off", label: "Not loaded; it starts when you ask", busy: false };
  }
}

/** Tokens as a short number: 640, 3.1k, 8k. */
export function kilo(n: number): string {
  if (n < 1000) return String(Math.max(0, Math.round(n)));
  const k = Math.round(n / 100) / 10;
  return `${Number.isInteger(k) ? k : k.toFixed(1)}k`;
}

/** The context meter: "3.1k / 8k". */
export const meter = (used: number, room: number) => `${kilo(used)} / ${kilo(room)}`;

/** How long the model thought, as the collapsed line says it. */
export function thoughtFor(ms: number): string {
  const s = Math.max(1, Math.round(ms / 1000));
  return s < 60 ? `Thought for ${s} s` : `Thought for ${Math.floor(s / 60)} min ${s % 60} s`;
}

/** When an idle model will be unloaded, as the settings and the header say it; "" if it stays loaded. */
export function unloadsIn(unloadsAt: number, now: number = Date.now()): string {
  if (!unloadsAt) return "";
  const minutes = Math.max(1, Math.ceil((unloadsAt - now) / 60_000));
  return `unloads in ${minutes} min`;
}

export type Message = { role: string; text: string };

/** The text the Up arrow brings back into an empty box: the last thing the person asked. */
export function lastQuestion(messages: readonly Message[] | null): string {
  for (let i = (messages?.length ?? 0) - 1; i >= 0; i--) if (messages![i].role === "user") return messages![i].text;
  return "";
}

/** What Escape does in the chat: stop an answer first, then close the history, then leave the window. */
export function escapeStep(busy: boolean, historyOpen: boolean, draft: string): "stop" | "history" | "close" | "none" {
  if (busy) return "stop";
  if (historyOpen) return "history";
  return draft.trim() === "" ? "close" : "none";
}

/** Whether Enter sends: not with Shift (a new line), not while an input method is composing, not an empty box. */
export function sends(event: { key: string; shiftKey: boolean; isComposing: boolean }, draft: string): boolean {
  return event.key === "Enter" && !event.shiftKey && !event.isComposing && draft.trim() !== "";
}

export type Phase = "loading" | "reading" | "thinking" | "answering" | "";

/** The line shown in place of an answer that has not started. */
export function waiting(phase: Phase, text: string): string {
  if (text) return text;
  switch (phase) {
    case "loading":
      return "Starting the model…";
    case "thinking":
      return "Thinking…";
    default:
      return "Reading…";
  }
}
