package websearch

import (
	"context"
	"errors"
	"fmt"
	"io"
	"mime"
	"net/http"
	"net/url"
	"strings"
	"unicode/utf8"

	"golang.org/x/net/html"
)

// pageLimit is how much of a page is read; the rest is ignored.
const pageLimit = 1500 << 10

// Page is the readable part of a web page.
type Page struct {
	URL   string `json:"url"`
	Title string `json:"title"`
	Text  string `json:"text"`
	// Cut says the text was shortened to fit maxChars.
	Cut bool `json:"cut"`
}

// Read fetches address and returns its text, at most maxChars characters of it.
func (c *Client) Read(ctx context.Context, address string, maxChars int) (Page, error) {
	u, err := url.Parse(strings.TrimSpace(address))
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
		return Page{}, fmt.Errorf("%q is not a web address (it must start with http:// or https://)", address)
	}
	ctx, cancel := context.WithTimeout(ctx, requestLimit)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return Page{}, fmt.Errorf("open the page: %w", err)
	}
	req.Header.Set("User-Agent", userAgent)
	req.Header.Set("Accept", "text/html,application/xhtml+xml,text/plain;q=0.8")
	req.Header.Set("Accept-Language", "en-US,en;q=0.8")
	resp, err := c.http.Do(req)
	if err != nil {
		return Page{}, fmt.Errorf("open the page: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return Page{}, fmt.Errorf("the site answered HTTP %d", resp.StatusCode)
	}
	kind, _, _ := mime.ParseMediaType(resp.Header.Get("Content-Type"))
	body := io.LimitReader(resp.Body, pageLimit)
	page := Page{URL: resp.Request.URL.String()}
	switch kind {
	case "text/html", "application/xhtml+xml", "":
		// ponytail: pages are read as UTF-8; another charset comes out garbled. Add golang.org/x/net/html/charset if it matters.
		doc, err := html.Parse(body)
		if err != nil {
			return Page{}, fmt.Errorf("read the page: %w", err)
		}
		page.Title, page.Text = extract(doc)
	case "text/plain":
		raw, err := io.ReadAll(body)
		if err != nil {
			return Page{}, fmt.Errorf("read the page: %w", err)
		}
		page.Text = tidy(strings.ToValidUTF8(string(raw), ""))
	default:
		return Page{}, fmt.Errorf("that is not a web page (%s); only pages and plain text can be read", kind)
	}
	if page.Text == "" {
		return Page{}, errors.New("the page has no text to read (it may need a browser to show its content)")
	}
	page.Text, page.Cut = clip(page.Text, maxChars)
	return page, nil
}

// skipped are the elements that hold no article: scripts, menus, forms and the rest of a page's furniture.
var skipped = map[string]bool{
	"script": true, "style": true, "noscript": true, "template": true, "svg": true, "iframe": true, "nav": true, "header": true,
	"footer": true, "aside": true, "form": true, "button": true, "select": true, "dialog": true, "head": true,
}

// breaks are the elements that end a line of text.
var breaks = map[string]bool{
	"p": true, "div": true, "li": true, "br": true, "h1": true, "h2": true, "h3": true, "h4": true, "h5": true, "h6": true,
	"tr": true, "section": true, "article": true, "main": true, "pre": true, "blockquote": true, "table": true, "ul": true,
	"ol": true, "dt": true, "dd": true, "figcaption": true,
}

// extract returns the page's title and its readable text: the article or main part when the page marks one and it holds a real
// amount of text, else the body (a small <article> on a page is often a card, not the page).
func extract(doc *html.Node) (title, text string) {
	if t := find(doc, func(n *html.Node) bool { return n.Data == "title" }); t != nil {
		title = squash(textOf(t))
	}
	for _, tag := range []string{"article", "main", "body"} {
		root := find(doc, func(n *html.Node) bool { return n.Data == tag })
		if root == nil {
			continue
		}
		if got := readable(root); len(got) > len(text) {
			text = got
		}
		if len(text) >= enoughText {
			break
		}
	}
	return title, text
}

// enoughText is how much text (in bytes) makes an <article> or <main> the page's content rather than a part of it.
const enoughText = 400

// readable is the text under root without the furniture, one paragraph to a line.
func readable(root *html.Node) string {
	var b strings.Builder
	var emit func(*html.Node, bool)
	emit = func(n *html.Node, pre bool) {
		switch {
		case n.Type == html.TextNode:
			if pre { // source line breaks are layout, except in code
				b.WriteString(n.Data)
			} else {
				b.WriteString(strings.ReplaceAll(n.Data, "\n", " "))
			}
		case n.Type == html.ElementNode && skipped[n.Data]:
			return
		case n.Type == html.ElementNode && breaks[n.Data]:
			b.WriteByte('\n')
			defer b.WriteByte('\n')
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			emit(c, pre || n.Data == "pre")
		}
	}
	emit(root, false)
	return tidy(b.String())
}

// tidy squashes the spaces of every line, drops empty lines and repeated neighbours.
func tidy(s string) string {
	var out []string
	for line := range strings.SplitSeq(s, "\n") {
		line = squash(line)
		if line == "" || (len(out) > 0 && out[len(out)-1] == line) {
			continue
		}
		out = append(out, line)
	}
	return strings.Join(out, "\n")
}

// clip cuts s to at most max characters at the end of a line, or of a word if one line is longer.
func clip(s string, max int) (string, bool) {
	if max <= 0 || utf8.RuneCountInString(s) <= max {
		return s, false
	}
	cut := string([]rune(s)[:max])
	if i := strings.LastIndexAny(cut, "\n"); i > max/2 {
		cut = cut[:i]
	} else if i := strings.LastIndexByte(cut, ' '); i > max/2 {
		cut = cut[:i]
	}
	return cut + "…", true
}
