/** A metal for the ring: `back` is its body, `tint` its highlights (the shader blends the tint over the body). */
export type Metal = { name: string; back: string; tint: string };

// The first is the default look; the rest are what the ring turns into. Two-colour ones (sunset,
// aurora) put a different hue in the highlights, which reads as a gradient moving through the metal.
export const METALS: readonly Metal[] = [
  { name: "chrome", back: "#8b8e98", tint: "#ffffff" },
  { name: "copper", back: "#b4622b", tint: "#ffe2c0" },
  { name: "ember", back: "#c2410c", tint: "#fde68a" },
  { name: "gold", back: "#a8842c", tint: "#fff3c4" },
  { name: "ice", back: "#5f7c9e", tint: "#e6f2ff" },
  { name: "violet", back: "#6b5b9a", tint: "#efe8ff" },
  { name: "emerald", back: "#2f7d68", tint: "#d8fff0" },
  { name: "rose", back: "#a85a74", tint: "#ffe3ec" },
  { name: "sunset", back: "#e0612b", tint: "#8a7cff" },
  { name: "aurora", back: "#2aa198", tint: "#b08cff" },
];

/** A random metal other than `previous`, so the ring never repeats itself between two openings. */
export function nextMetal(previous: number, random: () => number = Math.random): number {
  const others = METALS.length - 1;
  const pick = Math.min(others - 1, Math.floor(random() * others));
  return pick >= previous ? pick + 1 : pick;
}
