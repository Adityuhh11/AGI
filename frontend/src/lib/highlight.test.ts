// Run with: node --test src/lib/highlight.test.ts (Node strips the types itself)
import assert from "node:assert/strict";
import { test } from "node:test";
import { highlight, tidy, words } from "./highlight.ts";

const hits = (text, query) => highlight(text, query).filter((p) => p.hit).map((p) => p.text);

test("words keeps letters and digits, drops one-letter words, little words and repeats", () => {
  assert.deepEqual(words("a Binary, binary search of 2x and the tree"), ["binary", "search", "2x", "tree"]);
});

test("a word is marked where it starts a word, not in the middle of one", () => {
  assert.deepEqual(hits("Random and handy, tokenizer", "and token"), ["token"]);
  assert.deepEqual(hits("xtoken token_a 9token token", "token"), ["token", "token"]); // after a letter or digit: no; after punctuation: yes
  assert.deepEqual(hits("a, b and c", "and"), []); // a little word is never marked
});

test("every occurrence is marked, in the case the text has", () => {
  assert.deepEqual(hits("JWT token: verify the Token", "token"), ["token", "Token"]);
});

test("the parts always join back to the text", () => {
  const text = "const verify = (token) => jwt.verify(token)";
  assert.equal(highlight(text, "verify token").map((p) => p.text).join(""), text);
});

test("regex characters in the query are plain text", () => {
  assert.deepEqual(hits("f(x) = a+b", "a+b (x"), []);
  assert.deepEqual(hits("1+1 is 2", "1+1"), []);
});

test("no query or no text gives one plain part", () => {
  assert.deepEqual(highlight("hello", ""), [{ text: "hello", hit: false }]);
  assert.deepEqual(highlight("", "hello"), [{ text: "", hit: false }]);
});

test("a longer word wins over its prefix", () => {
  assert.deepEqual(hits("search searching", "search searching"), ["search", "searching"]);
});

test("tidy replaces private-use and control characters with a space and squeezes spaces", () => {
  assert.equal(tidy("Quiz   100 � B"), "Quiz 100 B");
  assert.equal(tidy("plain text"), "plain text");
});
