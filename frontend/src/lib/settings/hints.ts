import type { Estimates, FolderSurvey } from "../../../bindings/omni/internal/settingsui";
import { duration, files, size } from "../format.ts";

/** What each option of a choice would make indexing take, for the buttons: "about 25 min". Empty until every folder is counted. */
export function choiceHints(kind: string, ids: string[], estimate: Estimates): Record<string, string> {
  if (!estimate.Complete) return {};
  return Object.fromEntries(ids.map((id) => [id, duration(estimate.ByChoice?.[`${kind}:${id}`] ?? 0)]));
}

/** The same for the three speeds. */
export function paceHints(estimate: Estimates): Record<string, string> {
  if (!estimate.Complete) return {};
  return Object.fromEntries(["fast", "balanced", "gentle"].map((id) => [id, duration(estimate.ByPace?.[id] ?? 0)]));
}

/** What a folder holds and what indexing it would take: "1,204 files · 3.2 GB · about 25 min", "Counting...", or "" when the engine cannot say. */
export function surveyLine(folder: FolderSurvey | undefined): string {
  if (!folder) return "";
  switch (folder.State) {
    case "done":
      return [files(folder.Files), size(folder.Bytes), duration(folder.Seconds)].join(" · ");
    case "pending":
    case "running":
      return "Counting...";
    default:
      return "";
  }
}
