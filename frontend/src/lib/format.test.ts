import assert from "node:assert/strict";
import { test } from "node:test";
import { duration, files, size } from "./format.ts";

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
