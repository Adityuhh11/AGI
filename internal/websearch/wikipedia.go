package websearch

import (
	"context"
	"encoding/json"
	"fmt"
	"html"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"
)

const (
	wikipediaEndpoint = "https://en.wikipedia.org/w/api.php"
	// wikipediaAgent is what Wikimedia asks of a program that uses its API: say what it is and where to read about it.
	wikipediaAgent = "Omni/1.0 (https://github.com/Adityuhh11/Omni; local launcher)"
)

var markup = regexp.MustCompile(`<[^>]*>`)

// wikipedia searches English Wikipedia through its API. It is the fallback for when DuckDuckGo asks for a check (it does after
// a burst of searches from one address): an official, keyless API that does not block, though it only knows what an encyclopedia does.
func (c *Client) wikipedia(ctx context.Context, query string) ([]Result, error) {
	params := url.Values{"action": {"query"}, "list": {"search"}, "srsearch": {query}, "srlimit": {fmt.Sprint(maxResults)},
		"srprop": {"snippet"}, "format": {"json"}, "utf8": {"1"}}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.wikiEndpoint+"?"+params.Encode(), nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", wikipediaAgent)
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("search Wikipedia: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("Wikipedia did not answer the search (HTTP %d)", resp.StatusCode)
	}
	var out struct {
		Query struct {
			Search []struct{ Title, Snippet string } `json:"search"`
		} `json:"query"`
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, pageLimit)).Decode(&out); err != nil {
		return nil, fmt.Errorf("read Wikipedia's answer: %w", err)
	}
	var results []Result
	for _, hit := range out.Query.Search {
		results = append(results, Result{
			Title:   hit.Title + " - Wikipedia",
			URL:     "https://en.wikipedia.org/wiki/" + url.PathEscape(strings.ReplaceAll(hit.Title, " ", "_")),
			Snippet: squash(html.UnescapeString(markup.ReplaceAllString(hit.Snippet, ""))),
		})
	}
	return results, nil
}
