/** The indexing choices of File search, and what changing them on an existing index will do. */
export type Kind = "PDFPages" | "Photos" | "Videos" | "Audio";
export type Choices = Record<Kind, string>;

export const KIND_LABEL: Record<Kind, string> = { PDFPages: "PDF pages", Photos: "Photos", Videos: "Videos", Audio: "Audio and sound" };

// Cheapest to dearest to index, as the engine does it (docs/eval/2026-10-20-indexing-choices.md).
const COST: Record<Kind, string[]> = { PDFPages: ["text", "auto", "all"], Photos: ["name", "content"], Videos: ["name", "content"], Audio: ["name", "content"] };

export type Change = { kind: Kind; from: string; to: string; costlier: boolean };

/** What differs between the saved choices and the draft, kind by kind. */
export function changes(saved: Choices, draft: Choices): Change[] {
  return (Object.keys(COST) as Kind[])
    .filter((kind) => saved[kind] !== draft[kind])
    .map((kind) => ({
      kind,
      from: saved[kind],
      to: draft[kind],
      costlier: COST[kind].indexOf(draft[kind]) > COST[kind].indexOf(saved[kind]),
    }));
}

// A picture takes about 0.16 s on a graphics card and 4.9 s on the processor: 3 and 80 minutes per thousand.
const rate = (device: string) => (device === "cpu" ? "about 80 minutes per 1,000 pictures on the processor" : "about 3 minutes per 1,000 pictures on the graphics card");

// A window of sound takes 0.28 s on a graphics card and 3.8 s on the processor, four to the minute of sound: about 1 minute and
// 15 minutes per hour (docs/eval/2026-10-28-audio-embedding.md; six times slower when another large model holds the card).
const soundRate = (device: string) => (device === "cpu" ? "about 15 minutes per hour of sound on the processor" : "about 1 minute per hour of sound on the graphics card");

/** One sentence per change, saying what saving will do. */
export function effects(list: Change[], device: string): string[] {
  return list.map((change) => {
    const sound = change.kind === "Audio";
    const label = KIND_LABEL[change.kind];
    if (change.costlier) {
      return sound
        ? `${label}: the files and videos are looked at again and their sound indexed, ${soundRate(device)}. Search keeps working meanwhile.`
        : `${label}: the files are looked at again and the new pictures indexed, ${rate(device)}. Search keeps working meanwhile.`;
    }
    return `${label}: ${sound ? "sound" : "pictures"} you no longer search by ${sound ? "is" : "are"} removed from the index. Nothing is computed, so it is quick.`;
  });
}
