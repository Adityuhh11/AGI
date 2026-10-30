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

// The other themes do not wear metal: each has rings in colours that belong to its plate. On the light plates the bodies are
// deeper than the plate so the thin ring still reads. The first of a family is the ring of the windows that do not change it.
const RINGS: Record<string, readonly Metal[]> = {
  black: METALS,
  midnight: [
    { name: "sapphire", back: "#3b62c4", tint: "#cfe0ff" },
    { name: "glacier", back: "#2b84b8", tint: "#a8ecff" },
    { name: "indigo", back: "#5a4fcf", tint: "#e0d9ff" },
    { name: "moonlight", back: "#6d7aa6", tint: "#f1f5ff" },
    { name: "dusk", back: "#3b62c4", tint: "#c58bff" },
  ],
  forest: [
    { name: "moss", back: "#4d8a4b", tint: "#e3f7c9" },
    { name: "jade", back: "#2f8f6f", tint: "#cffff0" },
    { name: "gold leaf", back: "#a8933a", tint: "#fff4c2" },
    { name: "fern", back: "#5a9a3c", tint: "#e8ffc9" },
    { name: "mist", back: "#7fa48f", tint: "#f2fff7" },
  ],
  cream: [
    { name: "espresso", back: "#6b4a2f", tint: "#e9cfae" },
    { name: "terracotta", back: "#b5603a", tint: "#ffd9bd" },
    { name: "olive", back: "#7d7a2e", tint: "#f2f0b5" },
    { name: "ink", back: "#3d4a63", tint: "#cfd9ee" },
    { name: "brass", back: "#a07a2a", tint: "#f6e2a0" },
  ],
  blush: [
    { name: "rose", back: "#c2556f", tint: "#ffd3de" },
    { name: "plum", back: "#8a4a73", tint: "#f3d3e8" },
    { name: "peach", back: "#d98066", tint: "#ffe1d3" },
    { name: "mauve", back: "#a6708f", tint: "#f6dcea" },
    { name: "berry", back: "#a83a5a", tint: "#ffc6d4" },
  ],
  coral: [
    { name: "coral", back: "#d0534a", tint: "#ffd2cb" },
    { name: "brick", back: "#a8402f", tint: "#ffcdb8" },
    { name: "amber", back: "#e0793c", tint: "#ffe0b8" },
    { name: "crimson", back: "#b02a3a", tint: "#ffc2c8" },
    { name: "sunset", back: "#d0534a", tint: "#b05a8a" },
  ],
};

/** The rings a theme can wear (the metals for black); an unknown theme wears the metals. */
export function ringsFor(theme: string): readonly Metal[] {
  return RINGS[theme] ?? METALS;
}

/** A random ring other than `previous` out of `count`, so the ring never repeats itself between two openings. */
export function nextMetal(previous: number, random: () => number = Math.random, count: number = METALS.length): number {
  const others = count - 1;
  const pick = Math.min(others - 1, Math.floor(random() * others));
  return pick >= previous ? pick + 1 : pick;
}
