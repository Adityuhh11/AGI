import assert from "node:assert/strict";
import { test } from "node:test";
import { clock, duration, files, moment, size, when } from "./format.ts";

test("a time is said in minutes, then in hours and ten minutes", () => {
  assert.equal(duration(8), "under a minute");
  assert.equal(duration(60), "about 1 min");
  assert.equal(duration(25 * 60), "about 25 min");
  assert.equal(duration(89 * 60), "about 89 min");
  assert.equal(duration(2 * 3600), "about 2 h");
  assert.equal(duration(2.5 * 3600 + 120), "about 2 h 30 min");
  assert.equal(duration(11.4 * 3600), "about 11 h");
});

test("a size says MB below a gigabyte and GB above it", () => {
  assert.equal(size(200 * 1024), "under 1 MB");
  assert.equal(size(13 * 1024 * 1024), "13 MB");
  assert.equal(size(3.25 * 1024 * 1024 * 1024), "3.3 GB");
});

test("a count of files is plural except for one", () => {
  assert.equal(files(1), "1 file");
  assert.equal(files(1204), "1,204 files");
  assert.equal(files(0), "0 files");
});

test("a time of change is said as a person says it", () => {
  const now = new Date(2026, 9, 26, 15, 0); // Monday 26 October 2026
  const at = (...a: [number, number, number, number?, number?]) => new Date(...a).getTime();
  assert.equal(when(at(2026, 9, 26, 9, 5), now), "09:05");
  assert.equal(when(at(2026, 9, 25, 23, 59), now), "Yesterday");
  assert.equal(when(at(2026, 9, 22, 12, 0), now), "Thu");
  assert.equal(when(at(2026, 9, 19, 12, 0), now), "19 Oct");
  assert.equal(when(at(2025, 11, 31, 12, 0), now), "31 Dec 2025");
  assert.equal(when(at(2026, 9, 27, 1, 0), now), "01:00"); // a clock a little ahead is still "today"
});

test("seconds are shown as a clock", () => {
  assert.equal(clock(0), "00:00");
  assert.equal(clock(5), "00:05");
  assert.equal(clock(24 * 60 + 31), "24:31");
  assert.equal(clock(3600), "1:00:00");
  assert.equal(clock(3725), "1:02:05");
  assert.equal(clock(-3), "00:00");
});

test("a moment in a recording is written as a player writes it", () => {
  assert.equal(moment(7.9), "0:07");
  assert.equal(moment(75), "1:15");
  assert.equal(moment(3723.4), "1:02:03");
  assert.equal(moment(-2), "0:00");
});

