import type { Kind } from "../indexingPlan";

export type Option = { id: string; name: string; how: string };

// The words are the measurements in docs/eval/2026-10-18-pace.md: 17.6 s, 33.4 s and 103.5 s to index the same
// folder, with the graphics card busy 65%, 27% and 16% of the time.
export const SPEEDS: Option[] = [
  { id: "fast", name: "Fast", how: "Finishes soonest. The graphics card works hard, and other things may slow down meanwhile." },
  { id: "balanced", name: "Balanced", how: "Takes about twice as long. The graphics card is busy about a third of the time." },
  { id: "gentle", name: "Gentle", how: "Takes about six times as long. Your computer stays free and search stays quick." },
];

// What each kind of file is searched by (docs/superpowers/specs/2026-10-20-indexing-choices-design.md). Pictures are
// what indexing time goes on: about 0.16 s each on a graphics card, about 5 s on a processor alone.
export const CHOICES: { kind: Kind; label: string; options: Option[] }[] = [
  {
    kind: "PDFPages",
    label: "PDF pages",
    options: [
      { id: "auto", name: "Pages that need it", how: "Scans, slides and pages with pictures or tables are also found by how they look. Every page is found by its words." },
      { id: "all", name: "Every page", how: "Any page can be found by how it looks. The slowest: every page is a picture to index." },
      { id: "text", name: "Words only", how: "Pages are found by their words. The fastest, and the choice for a computer without a graphics card." },
    ],
  },
  {
    kind: "Photos",
    label: "Photos",
    options: [
      { id: "content", name: "What they show", how: "Find a photo by describing it: “dog on a beach”." },
      { id: "name", name: "Their names", how: "Only the file name is searched. Much faster without a graphics card." },
    ],
  },
  {
    kind: "Videos",
    label: "Videos",
    options: [
      { id: "name", name: "Their names", how: "Videos are found by their file names." },
      { id: "content", name: "What is in them", how: "Eight frames of each video are searched too; a video costs as much as eight photos." },
    ],
  },
];
