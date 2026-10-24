/** The last few folders of a path, so a long one stays readable: `C:\a\b\c\d` -> `… › b › c › d`. */
export function shortFolder(folder: string, keep = 3): string {
  const parts = folder.split(/[\\/]+/).filter(Boolean);
  if (parts.length <= keep) return parts.join(" › ");
  return "… › " + parts.slice(-keep).join(" › ");
}

/** Whether `folder` is `root` or lies below it, as Windows sees paths (case does not matter). */
export function isInside(root: string, folder: string): boolean {
  const norm = (p: string) => p.replace(/[\\/]+$/, "").toLowerCase();
  const r = norm(root);
  const f = norm(folder);
  return f === r || f.startsWith(r + "\\");
}
