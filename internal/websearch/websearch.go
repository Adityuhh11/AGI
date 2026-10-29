// Package websearch is what Omni AI uses to look things up: Search finds pages on DuckDuckGo and Read fetches one page and keeps
// its readable text. It knows nothing of the model. Both only read; neither can reach this computer or the local network (guard.go).
// Why DuckDuckGo's HTML page and not an API: docs/decisions/0009-web-search-tool.md.
package websearch

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"golang.org/x/net/html"
)

const (
	searchEndpoint = "https://html.duckduckgo.com/html/"
	// userAgent is an ordinary browser's: DuckDuckGo and many sites answer a program that says what it is with a challenge page.
	userAgent    = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/130.0 Safari/537.36"
	maxResults   = 5
	maxQuery     = 300
	requestLimit = 12 * time.Second
)

// Result is one hit of a search.
type Result struct {
	Title   string `json:"title"`
	URL     string `json:"url"`
	Snippet string `json:"snippet"`
}

// Client searches and reads. The zero value is not usable; use New.
type Client struct {
	http         *http.Client
	endpoint     string // DuckDuckGo's
	wikiEndpoint string

	mu    sync.Mutex
	cache map[string]cached
}

type cached struct {
	results []Result
	at      time.Time
}

const (
	cacheFor = 10 * time.Minute
	cacheMax = 64
)

// errBlocked is DuckDuckGo asking for a check instead of answering: a burst of searches from one address does it, and it passes.
var errBlocked = errors.New("DuckDuckGo is asking for a check")

// New returns a Client that cannot reach local or private addresses.
func New() *Client {
	return &Client{http: guardedClient(false), endpoint: searchEndpoint, wikiEndpoint: wikipediaEndpoint}
}

// Search returns the first results for query. No results is an empty list, not an error. When DuckDuckGo asks for a check the
// search goes to Wikipedia instead; when that finds nothing either, the error says what DuckDuckGo did, so the model reports it
// instead of answering as if it had looked. The same search within ten minutes is answered from memory, which keeps a model that
// repeats itself from counting against DuckDuckGo's limit.
func (c *Client) Search(ctx context.Context, query string) ([]Result, error) {
	query = strings.Join(strings.Fields(query), " ")
	if query == "" {
		return nil, errors.New("nothing to search for")
	}
	if r := []rune(query); len(r) > maxQuery {
		query = string(r[:maxQuery])
	}
	key := strings.ToLower(query)
	if results, ok := c.remembered(key); ok {
		return results, nil
	}
	ctx, cancel := context.WithTimeout(ctx, requestLimit)
	defer cancel()
	results, err := c.duckduckgo(ctx, query)
	if errors.Is(err, errBlocked) {
		if fallback, werr := c.wikipedia(ctx, query); werr == nil && len(fallback) > 0 {
			results, err = fallback, nil
		}
	}
	if err == nil {
		c.remember(key, results)
	}
	return results, err
}

func (c *Client) remembered(key string) ([]Result, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	hit, ok := c.cache[key]
	return hit.results, ok && time.Since(hit.at) < cacheFor
}

func (c *Client) remember(key string, results []Result) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.cache == nil || len(c.cache) >= cacheMax {
		c.cache = map[string]cached{}
	}
	c.cache[key] = cached{results: results, at: time.Now()}
}

func (c *Client) duckduckgo(ctx context.Context, query string) ([]Result, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.endpoint, strings.NewReader(url.Values{"q": {query}}.Encode()))
	if err != nil {
		return nil, fmt.Errorf("search the web: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("User-Agent", userAgent)
	req.Header.Set("Accept-Language", "en-US,en;q=0.8")
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("search the web: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		// 202 is how DuckDuckGo asks for a check (the "lite" page does it at once; the HTML one after many searches)
		return nil, fmt.Errorf("%w (HTTP %d); try again in a few minutes", errBlocked, resp.StatusCode)
	}
	doc, err := html.Parse(io.LimitReader(resp.Body, pageLimit))
	if err != nil {
		return nil, fmt.Errorf("read DuckDuckGo's answer: %w", err)
	}
	results, recognised := parseResults(doc)
	if !recognised {
		return nil, fmt.Errorf("%w, or its page changed: the answer was not a list of results", errBlocked)
	}
	return results, nil
}

// parseResults reads DuckDuckGo's HTML results. recognised is false when the page has neither results nor the "no results" notice.
func parseResults(doc *html.Node) (results []Result, recognised bool) {
	seen := map[string]bool{}
	walk(doc, func(n *html.Node) bool {
		if hasClass(n, "no-results") || attr(n, "id") == "links" {
			recognised = true
		}
		if !hasClass(n, "result") || hasClass(n, "result--ad") || len(results) >= maxResults {
			return true
		}
		link := find(n, func(a *html.Node) bool { return a.Data == "a" && hasClass(a, "result__a") })
		if link == nil {
			return true
		}
		address := unwrap(attr(link, "href"))
		if address == "" || seen[address] {
			return false
		}
		seen[address] = true
		r := Result{Title: squash(textOf(link)), URL: address}
		if snippet := find(n, func(s *html.Node) bool { return hasClass(s, "result__snippet") }); snippet != nil {
			r.Snippet = squash(textOf(snippet))
		}
		results = append(results, r)
		return false // the result's own parts are read; do not look for results inside it
	})
	return results, recognised || len(results) > 0
}

// unwrap returns the address a result points to: DuckDuckGo sometimes sends the link through its own redirect
// (//duckduckgo.com/l/?uddg=<address>), which would show the person a tracker instead of the page.
func unwrap(href string) string {
	href = strings.TrimSpace(href)
	if strings.HasPrefix(href, "//") {
		href = "https:" + href
	}
	u, err := url.Parse(href)
	if err != nil {
		return ""
	}
	if strings.HasSuffix(u.Hostname(), "duckduckgo.com") && u.Path == "/l/" {
		href = u.Query().Get("uddg")
		if u, err = url.Parse(href); err != nil {
			return ""
		}
	}
	if u.Scheme != "http" && u.Scheme != "https" || u.Host == "" {
		return ""
	}
	return u.String()
}
