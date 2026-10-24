/** How long indexing takes, as a person says it: "about 25 min", "about 2 h 30 min". Under a minute is "under a minute". */
export function duration(seconds: number): string {
  if (seconds < 45) return "under a minute";
  const minutes = Math.round(seconds / 60);
  if (minutes < 90) return `about ${minutes} min`;
  const tenMinutes = Math.round(seconds / 600);
  const hours = Math.floor(tenMinutes / 6);
  const rest = (tenMinutes % 6) * 10;
  if (hours >= 10 || rest === 0) return `about ${Math.round(seconds / 3600)} h`;
  return `about ${hours} h ${rest} min`;
}

/** A size: "under 1 MB", "13 MB", "3.2 GB". */
export function size(bytes: number): string {
  const mb = bytes / (1024 * 1024);
  if (mb < 1) return "under 1 MB";
  if (mb < 1024) return `${Math.round(mb)} MB`;
  return `${(mb / 1024).toFixed(1)} GB`;
}

/** "1 file", "1,204 files". */
export function files(count: number): string {
  return `${count.toLocaleString("en-US")} ${count === 1 ? "file" : "files"}`;
}
