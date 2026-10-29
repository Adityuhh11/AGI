import { Clipboard } from "@wailsio/runtime";

/**
 * A Svelte action for a block of rendered markdown: it gives each code block a Copy button. The text is re-rendered while an
 * answer streams (which replaces the blocks), so the buttons are added again after every change of `html`.
 */
export function copyable(node: HTMLElement, _html: string) {
  const decorate = () => {
    for (const pre of node.querySelectorAll("pre")) {
      if (pre.querySelector(":scope > button.copy")) continue;
      const button = document.createElement("button");
      button.type = "button";
      button.className = "copy";
      button.textContent = "Copy";
      button.setAttribute("aria-label", "Copy this code");
      pre.append(button);
    }
  };
  const onclick = async (event: MouseEvent) => {
    const button = (event.target as HTMLElement).closest("button.copy");
    if (!button) return;
    const code = button.parentElement?.querySelector("code")?.textContent ?? "";
    try {
      await Clipboard.SetText(code);
      button.textContent = "Copied";
    } catch {
      button.textContent = "Not copied";
    }
    setTimeout(() => (button.textContent = "Copy"), 1400);
  };
  node.addEventListener("click", onclick);
  decorate();
  return {
    update: decorate,
    destroy: () => node.removeEventListener("click", onclick),
  };
}
