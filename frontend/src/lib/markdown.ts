/**
 * A small markdown renderer for notes: headings, paragraphs, quotes, lists (also numbered and task lists), code and code
 * blocks, rules, and bold, italic, strike-through, code, links. Everything in the text is escaped first and only the tags
 * written here are added, so a note can never put script or markup into the page. Links are limited to http, https and mailto.
 */

const escapes: Record<string, string> = { "&": "&amp;", "<": "&lt;", ">": "&gt;", '"': "&quot;", "'": "&#39;" };
const escape = (text: string) => text.replace(/[&<>"']/g, (c) => escapes[c]);

/** Inline marks inside one already-escaped line. Code spans are lifted out first so their content is left alone. */
function inline(escaped: string): string {
  if (escaped.length > MAX_INLINE) return escaped; // a pasted wall of text is shown as it is: the patterns below are not meant for it
  // Code spans and links are lifted out first and put back last, so emphasis marks inside them (a URL with __x__ in it) are never touched.
  const spans: string[] = [];
  const lift = (html: string) => `\u0000${spans.push(html) - 1}\u0000`;
  let text = escaped.replace(/`([^`]+)`/g, (_, code) => lift(`<code>${code}</code>`));
  text = text.replace(/\[([^\]]+)\]\(((?:https?:\/\/|mailto:)(?:(?!&(?:quot|#39|lt|gt);)[^\s)])+)\)/gi, (_, label, url) => lift(`<a href="${url}">${label}</a>`));
  text = text.replace(/(^|[\s(])(https?:\/\/(?:(?!&(?:quot|#39|lt|gt);)[^\s<)])+(?<![.,;:!?]))/gi, (_, before, url) => `${before}${lift(`<a href="${url}">${url}</a>`)}`);
  text = text.replace(/\*\*([^*]+)\*\*|__([^_]+)__/g, (_, a, b) => `<strong>${a ?? b}</strong>`);
  text = text.replace(/(^|[^\w*])\*([^*\s][^*]*)\*(?!\w)|(^|[^\w_])_([^_\s][^_]*)_(?!\w)/g, (_, p1, a, p2, b) =>
    a !== undefined ? `${p1}<em>${a}</em>` : `${p2}<em>${b}</em>`,
  );
  text = text.replace(/~~([^~]+)~~/g, "<del>$1</del>");
  const put = (t: string) => t.replace(/\u0000(\d+)\u0000/g, (_, i) => spans[Number(i)] ?? "");
  return put(put(text)); // a link's words can hold a code span, which is itself a placeholder
}

const MAX_INLINE = 4000; // characters of one line that get emphasis and links
const MAX_QUOTE_DEPTH = 8; // levels of > that are nested; deeper ones are shown as text

const heading = /^(#{1,6})\s+(.*)$/;
const rule = /^\s*([-*_])(\s*\1){2,}\s*$/;
const bullet = /^(\s*)[-*+]\s+(.*)$/;
const ordered = /^(\s*)\d+[.)]\s+(.*)$/;
const task = /^\[([ xX])\]\s+(.*)$/;
const quote = /^>\s?(.*)$/;
const fence = /^```/;

/** Renders markdown to HTML. A task's box carries `data-line`, the line of the source it came from (see toggleTask). */
export function render(markdown: string, firstLine = 0, depth = 0): string {
  const lines = markdown.replace(/\r\n?/g, "\n").replaceAll("\u0000", "").split("\n");
  const out: string[] = [];
  let i = 0;
  while (i < lines.length) {
    const line = lines[i];
    if (line.trim() === "") {
      i++;
    } else if (fence.test(line)) {
      const code: string[] = [];
      i++;
      while (i < lines.length && !fence.test(lines[i])) code.push(lines[i++]);
      i++; // the closing fence (a missing one ends the block at the end of the note)
      out.push(`<pre><code>${escape(code.join("\n"))}</code></pre>`);
    } else if (heading.test(line)) {
      const [, hashes, text] = line.match(heading)!;
      out.push(`<h${hashes.length}>${inline(escape(text.trim()))}</h${hashes.length}>`);
      i++;
    } else if (rule.test(line)) {
      out.push("<hr>");
      i++;
    } else if (quote.test(line) && depth < MAX_QUOTE_DEPTH) {
      const start = i;
      const body: string[] = [];
      while (i < lines.length && quote.test(lines[i])) body.push(lines[i++].match(quote)![1]);
      // The quoted lines keep their place in the note, so a task inside a quote toggles its own line (see toggleTask).
      out.push(`<blockquote>${render(body.join("\n"), firstLine + start, depth + 1)}</blockquote>`);
    } else if (bullet.test(line) || ordered.test(line)) {
      const isOrdered = !bullet.test(line);
      const items: string[] = [];
      while (i < lines.length && (isOrdered ? ordered : bullet).test(lines[i])) {
        const text = lines[i].match(isOrdered ? ordered : bullet)![2];
        const t = !isOrdered ? text.match(task) : null;
        if (t) {
          const checked = t[1] !== " ";
          items.push(
            `<li class="task${checked ? " done" : ""}"><input type="checkbox" data-line="${firstLine + i}"${checked ? " checked" : ""}> <span>${inline(escape(t[2]))}</span></li>`,
          );
        } else {
          items.push(`<li>${inline(escape(text))}</li>`);
        }
        i++;
      }
      out.push(`<${isOrdered ? "ol" : "ul"}>${items.join("")}</${isOrdered ? "ol" : "ul"}>`);
    } else {
      const para: string[] = [];
      while (
        i < lines.length &&
        lines[i].trim() !== "" &&
        !fence.test(lines[i]) &&
        !heading.test(lines[i]) &&
        !rule.test(lines[i]) &&
        !(quote.test(lines[i]) && depth < MAX_QUOTE_DEPTH) &&
        !bullet.test(lines[i]) &&
        !ordered.test(lines[i])
      ) {
        para.push(inline(escape(lines[i++])));
      }
      out.push(`<p>${para.join("<br>")}</p>`);
    }
  }
  return out.join("\n");
}

/** The note with the task on this line (0-based) ticked if it was open, or unticked if it was done. Other lines are left alone. */
export function toggleTask(markdown: string, line: number): string {
  const lines = markdown.split("\n");
  const at = lines[line];
  if (at === undefined) return markdown;
  const toggled = at.replace(/^(\s*(?:>\s?)*[-*+]\s+)\[([ xX])\]/, (_, before, mark) => `${before}[${mark === " " ? "x" : " "}]`);
  if (toggled === at) return markdown;
  lines[line] = toggled;
  return lines.join("\n");
}
