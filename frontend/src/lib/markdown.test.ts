// Run with: node --test src/lib/markdown.test.ts (Node strips the types itself)
import assert from "node:assert/strict";
import { test } from "node:test";
import { render, toggleTask } from "./markdown.ts";

test("block elements", () => {
  assert.equal(render("# Title"), "<h1>Title</h1>");
  assert.equal(render("###### Small"), "<h6>Small</h6>");
  assert.equal(render("####### seven"), "<p>####### seven</p>");
  assert.equal(render("---"), "<hr>");
  assert.equal(render("* * *"), "<hr>");
  assert.equal(render("> quoted\n> more"), "<blockquote><p>quoted<br>more</p></blockquote>");
  assert.equal(render("- a\n- b"), "<ul><li>a</li><li>b</li></ul>");
  assert.equal(render("1. one\n2) two"), "<ol><li>one</li><li>two</li></ol>");
  assert.equal(render("first\nsecond\n\nthird"), "<p>first<br>second</p>\n<p>third</p>");
  assert.equal(render(""), "");
});

test("a fenced block is shown as typed, escaped, even without a closing fence", () => {
  assert.equal(render("```\n<b>x</b> **y**\n```"), "<pre><code>&lt;b&gt;x&lt;/b&gt; **y**</code></pre>");
  assert.equal(render("```\nopen"), "<pre><code>open</code></pre>");
});

test("inline marks", () => {
  assert.equal(render("**bold** and __bold__"), "<p><strong>bold</strong> and <strong>bold</strong></p>");
  assert.equal(render("*it* and _it_"), "<p><em>it</em> and <em>it</em></p>");
  assert.equal(render("~~gone~~"), "<p><del>gone</del></p>");
  assert.equal(render("use `a*b*c` here"), "<p>use <code>a*b*c</code> here</p>");
  assert.equal(render("snake_case_name and 2*3*4"), "<p>snake_case_name and 2*3*4</p>");
});

test("links are limited to web and mail addresses", () => {
  assert.equal(render("[site](https://example.com/a?b=1)"), '<p><a href="https://example.com/a?b=1">site</a></p>');
  assert.equal(render("[mail](mailto:a@b.co)"), '<p><a href="mailto:a@b.co">mail</a></p>');
  assert.equal(render("see https://example.com."), '<p>see <a href="https://example.com">https://example.com</a>.</p>');
  const bad = render("[x](javascript:alert(1)) [y](file:///c:/secret) [z](data:text/html,hi)");
  assert.ok(!bad.includes("<a "), bad);
});

test("nothing typed can become markup or script", () => {
  const nasty = [
    "<script>alert(1)</script>",
    '<img src=x onerror="alert(1)">',
    '[x](https://a.com" onmouseover="alert(1))',
    "**<b onclick=1>**",
    "- <iframe src=//evil>",
    "# <svg onload=alert(1)>",
    "`</code><script>`",
    "> <a href=javascript:1>",
    "```\n</code></pre><script>\n```",
  ];
  const allowedTags = new Set(["a", "input", "li", "code", "pre", "p", "strong", "em", "del", "ul", "ol", "blockquote", "hr", "br", "h1", "h2", "h3", "h4", "h5", "h6"]);
  const allowedAttributes = /^( href="[^"<>]*")?( type="checkbox")?( data-line="\d+")?( checked)?( class="[a-z ]*")?$/;
  for (const text of nasty) {
    const html = render(text);
    for (const [, name, attributes] of html.matchAll(/<\/?([a-z0-9]+)([^>]*)>/gi)) {
      assert.ok(allowedTags.has(name.toLowerCase()), `${name} in ${html}`);
      assert.match(attributes, allowedAttributes, html);
    }
  }
  const injected = render('[x](https://a.com" onmouseover="alert(1))');
  assert.ok(injected.includes('href="https://a.com"') && !injected.includes(' onmouseover="'), injected);
});

test("task lists carry the line they came from and keep their state", () => {
  const html = render("intro\n\n- [ ] open\n- [x] done\n- plain");
  assert.match(html, /<input type="checkbox" data-line="2"> <span>open<\/span>/);
  assert.match(html, /<li class="task done"><input type="checkbox" data-line="3" checked> <span>done<\/span>/);
  assert.match(html, /<li>plain<\/li>/);
});

test("toggleTask flips one line and only a task line", () => {
  const note = "intro\n\n- [ ] open\n- [x] done\n  * [X] nested";
  assert.equal(toggleTask(note, 2), "intro\n\n- [x] open\n- [x] done\n  * [X] nested");
  assert.equal(toggleTask(note, 3), "intro\n\n- [ ] open\n- [ ] done\n  * [X] nested");
  assert.equal(toggleTask(note, 4), "intro\n\n- [ ] open\n- [x] done\n  * [ ] nested");
  assert.equal(toggleTask(note, 0), note);
  assert.equal(toggleTask(note, 99), note);
  assert.equal(toggleTask("a\r\n- [ ] b", 1), "a\r\n- [x] b");
});

test("windows line endings render like unix ones", () => {
  assert.equal(render("# a\r\n\r\n- b\r\n"), render("# a\n\n- b\n"));
});
