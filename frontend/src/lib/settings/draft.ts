import { writable } from "svelte/store";
import type { Choices } from "../indexingPlan";

/** The indexing choices the person has picked on an index that exists but not saved yet. It lives here, not in the File
 *  search component, so it survives a visit to another settings page; closing the window drops it. */
export const draft = writable<Choices | null>(null);
