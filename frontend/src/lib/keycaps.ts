/** The keys of an accelerator ("Alt+Shift+Space") as the caps to draw. A "+" key is written "Ctrl++". */
export function keycaps(accelerator: string): string[] {
  const keys = accelerator.split("+");
  const caps: string[] = [];
  for (let i = 0; i < keys.length; i++) {
    if (keys[i] === "" && i < keys.length - 1 && keys[i + 1] === "") {
      caps.push("+");
      i++;
    } else if (keys[i] !== "") {
      caps.push(keys[i]);
    }
  }
  return caps;
}
