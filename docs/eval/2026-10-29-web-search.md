# Web search for Omni AI: what was measured

Date: 2026-10-29. ADR 0009. Machine: the owner's laptop, real network, the installed
LFM3-1.7B (QAD Q4_0) on the graphics card. Throwaway `curl` calls and the gated test `TestRealModelUsesTheWeb`
(`OMNI_CHAT_DIR=<chat folder> go test ./internal/chat -run Real -v`; `OMNI_CHAT_THINKING=1` for thinking on).

## Which search source

| Source | Result |
|---|---|
| DuckDuckGo `html.duckduckgo.com/html/` (POST) | HTTP 200, 10 results with direct URLs, no key. **HTTP 202 (a check) after about fifteen searches in a few minutes from this address**, and still 202 when the session ended |
| DuckDuckGo `lite.duckduckgo.com/lite/` | HTTP 202 at once |
| Bing `bing.com/search` (keyless) | HTTP 200, 10 results, but unrelated ones: hotels for "golang 1.24 release date", Google's own pages for "latest stable go version" (decoded the `u=a1...` links) |
| Mojeek | HTTP 403 |
| Wikipedia `action=query&list=search` | Official API, no key, did not block |

So: DuckDuckGo first, Wikipedia when DuckDuckGo asks for a check, a ten-minute memory of searches. Brave and the other keyed APIs
stay the way out if the HTML page changes.

## Through the real model (tools offered, 8,192 context)

| Question | Thinking off | Thinking on |
|---|---|---|
| Latest stable Go, and when released | searched, read go.dev: Go 1.27, 19 August 2026, sources go.dev (13.3 s) | via Wikipedia (DuckDuckGo blocked): read go.dev/dl, Go 1.27.3, said the date was not on the page (26.9 s) |
| 12 times 7 | answered directly, no search (1.1 s) | answered directly (0.7 s) |
| Most recent F1 drivers' champion | searched, read Wikipedia's list (3.8 s) | via Wikipedia: read the 2026 season page (28.9 s) |

Found and fixed on the way:

- **Thinking on, last request without tools: the model wrote its next call as text** (`<tool_call><function=web_search>...`) and that was the
  answer. The last request now ends with "Answer now with what you have found... Do not search again."
- **Repeated searches** used up DuckDuckGo's allowance: the model repeats itself. Searches are remembered for ten minutes.
- The model with searches failing (HTTP 202, before the fallback existed) said it could not look the answer up, rather than inventing one.

## In the real app

Built with `scripts/build.ps1`, started, the owner's chat hotkey pressed with synthetic keys, windows screenshotted at 125%: the **+** menu
opens above the composer with Think (on) and Search the web (off, with "Your search words go to DuckDuckGo"); switching Web on shows a Web
pill beside the Think pill; a question showed the live reasoning, "Reading go.dev…", then the answer with a `go.dev` source chip under it.
Clicking the Web pill switched it off again and wrote `web_search = false`.

Not measured: a search with no network at all (covered by the unit test of a failing search), a page in a non-UTF-8 charset (read as UTF-8; garbled),
and the cost of the loop on the 4,096-token context (the room check is tested with a fake model, not with a real long page).
