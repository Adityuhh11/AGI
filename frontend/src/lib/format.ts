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

const MONTHS = ["Jan", "Feb", "Mar", "Apr", "May", "Jun", "Jul", "Aug", "Sep", "Oct", "Nov", "Dec"];
const DAYS = ["Sun", "Mon", "Tue", "Wed", "Thu", "Fri", "Sat"];

/** When a note was changed, as a person says it: the time today, "Yesterday", the weekday this week, else the date. */
export function when(ms: number, now: Date = new Date()): string {
  const then = new Date(ms);
  const midnight = (d: Date) => new Date(d.getFullYear(), d.getMonth(), d.getDate()).getTime();
  const days = Math.round((midnight(now) - midnight(then)) / 86_400_000);
  const pad = (n: number) => String(n).padStart(2, "0");
  if (days <= 0) return `${pad(then.getHours())}:${pad(then.getMinutes())}`;
  if (days === 1) return "Yesterday";
  if (days < 7) return DAYS[then.getDay()];
  const date = `${then.getDate()} ${MONTHS[then.getMonth()]}`;
  return then.getFullYear() === now.getFullYear() ? date : `${date} ${then.getFullYear()}`;
}

/** Seconds as a clock: 05:00, 24:31, 1:05:00. */
export function clock(seconds: number): string {
  const s = Math.max(0, Math.round(seconds));
  const pad = (n: number) => String(n).padStart(2, "0");
  const h = Math.floor(s / 3600);
  const rest = s % 3600;
  return `${h > 0 ? `${h}:` : ""}${pad(Math.floor(rest / 60))}:${pad(rest % 60)}`;
}
