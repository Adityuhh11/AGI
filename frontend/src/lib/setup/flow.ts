/** The screens of the setup guide, in order. File search is optional: without it only the first four are visited. */
export type Step = "welcome" | "keys" | "files" | "folders" | "options" | "done";

const WITH_FILES: Step[] = ["welcome", "keys", "files", "folders", "options", "done"];
const WITHOUT_FILES: Step[] = ["welcome", "keys", "files", "done"];

/** The screens to walk through, given whether the person is setting up file search. */
export function stepsFor(files: boolean): Step[] {
  return files ? WITH_FILES : WITHOUT_FILES;
}

/** The screen after `step`, or the same one at the end. */
export function after(step: Step, files: boolean): Step {
  const steps = stepsFor(files);
  return steps[Math.min(steps.indexOf(step) + 1, steps.length - 1)] ?? "done";
}

/** The screen before `step`, or the same one at the start. */
export function before(step: Step, files: boolean): Step {
  const steps = stepsFor(files);
  return steps[Math.max(steps.indexOf(step) - 1, 0)] ?? "welcome";
}
