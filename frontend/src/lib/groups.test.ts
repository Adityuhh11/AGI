// Run with: node --test src/lib/groups.test.ts (Node strips the types itself)
import assert from "node:assert/strict";
import { test } from "node:test";
import { flatten } from "./flatten.ts";
import { arrange } from "./groups.ts";

const row = (Path: string, Kind: string, Score: number) => ({ Path, Name: Path, Folder: "", Kind, Line: 0, Score, Matches: [] });

const mixed = [row("a.go", "code", 0.5), row("b.pdf", "pdf", 0.9), row("c.go", "code", 0.7), row("d.png", "image", 0.6), row("e.md", "text", 0.4)];

test("grouping puts the best match alone first, then a heading per kind ordered by their best file", () => {
  const groups = arrange(mixed, true);
  assert.deepEqual(
    groups.map((g) => [g.heading, g.rows.map((r) => r.Path)]),
    [
      ["Best match", ["b.pdf"]],
      ["Code", ["c.go", "a.go"]],
      ["Images", ["d.png"]],
      ["Documents", ["e.md"]],
    ],
  );
});

test("the best match is not listed twice", () => {
  const all = arrange(mixed, true).flatMap((g) => g.rows);
  assert.equal(all.length, mixed.length);
  assert.equal(new Set(all.map((r) => r.Path)).size, mixed.length);
});

test("ties keep the engine's order", () => {
  const rows = [row("x.go", "code", 1), row("y.pdf", "pdf", 1), row("z.png", "image", 1), row("w.md", "text", 1)];
  assert.deepEqual(arrange(rows, true)[0].rows.map((r) => r.Path), ["x.go"]);
});

test("a list is left alone when grouping is off, short, or of one kind", () => {
  assert.deepEqual(arrange(mixed, false), [{ heading: "", rows: mixed }]);
  assert.deepEqual(arrange(mixed.slice(0, 3), true), [{ heading: "", rows: mixed.slice(0, 3) }]);
  const oneKind = [row("a", "code", 1), row("b", "code", 2), row("c", "code", 3), row("d", "code", 4)];
  assert.deepEqual(arrange(oneKind, true), [{ heading: "", rows: oneKind }]);
  assert.deepEqual(arrange([], true), [{ heading: "", rows: [] }]);
});

test("a kind the page does not know gets a heading of its own", () => {
  const rows = [row("a", "code", 1), row("b", "archive", 0.9), row("c", "code", 0.8), row("d", "pdf", 0.7)];
  assert.ok(arrange(rows, true).some((g) => g.heading === "Other files"));
});

test("flatten carries each group's heading on its first row only", () => {
  const items = flatten(mixed, new Set(), [], false, true);
  const headings = items.map((i) => (i.kind === "row" ? i.heading : undefined));
  assert.deepEqual(headings, ["Best match", "Code", undefined, "Images", "Documents"]);
  assert.ok(flatten(mixed, new Set(), [], false, false).every((i) => i.kind !== "row" || i.heading === undefined));
});

test("audio has its own heading", () => {
  const groups = arrange([row("best.pdf", "pdf", 0.95), row("talk.mp3", "audio", 0.6), row("a.go", "code", 0.5), row("c.md", "text", 0.3)], true);
  assert.deepEqual(
    groups.map((g) => [g.heading, g.rows.map((r) => r.Path)]),
    [
      ["Best match", ["best.pdf"]],
      ["Audio", ["talk.mp3"]],
      ["Code", ["a.go"]],
      ["Documents", ["c.md"]],
    ],
  );
});
