/** The last few folders of a path, so a long one stays readable: `C:\a\b\c\d` -> `… › b › c › d`. */
export function shortFolder(folder: string, keep = 3): string {
  const parts = folder.split(/[\\/]+/).filter(Boolean);
  if (parts.length <= keep) return parts.join(" › ");
  return "… › " + parts.slice(-keep).join(" › ");
}
