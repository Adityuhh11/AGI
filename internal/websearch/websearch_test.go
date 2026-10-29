package websearch

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"golang.org/x/net/html"
)

// resultsPage has DuckDuckGo's markup (saved 2026-10-29): an ad, a result behind its own redirect, a repeated result.
const resultsPage = `<html><body><div id="links" class="results">
<div class="result results_links result--ad"><h2 class="result__title"><a class="result__a" href="https://ads.example/x">Buy now</a></h2></div>
<div class="result results_links web-result"><div class="result__body">
  <h2 class="result__title"><a rel="nofollow" class="result__a" href="https://go.dev/doc/go1.24">Go 1.24 Release Notes - The Go Programming Language</a></h2>
  <a class="result__snippet" href="https://go.dev/doc/go1.24">Go 1.24 was <b>released</b> in   February 2025.</a>
</div></div>
<div class="result results_links web-result"><h2><a class="result__a" href="//duckduckgo.com/l/?uddg=https%3A%2F%2Fgo.dev%2Fblog%2Fgo1.24&amp;rut=abc">Go 1.24 is released!</a></h2></div>
<div class="result results_links web-result"><h2><a class="result__a" href="https://go.dev/doc/go1.24">Same page again</a></h2></div>
<div class="result results_links web-result"><h2><a class="result__a" href="javascript:alert(1)">Not a page</a></h2></div>
</div></body></html>`

func parse(t *testing.T, s string) *html.Node {
	t.Helper()
	doc, err := html.Parse(strings.NewReader(s))
	if err != nil {
		t.Fatal(err)
	}
	return doc
}

func TestParseResultsSkipsAdsUnwrapsRedirectsAndDropsRepeatsAndNonPages(t *testing.T) {
	results, recognised := parseResults(parse(t, resultsPage))
	if !recognised || len(results) != 2 {
		t.Fatalf("recognised=%v results=%+v", recognised, results)
	}
	if got := results[0]; got.URL != "https://go.dev/doc/go1.24" || got.Title != "Go 1.24 Release Notes - The Go Programming Language" ||
		got.Snippet != "Go 1.24 was released in February 2025." {
		t.Errorf("first result: %+v", got)
	}
	if results[1].URL != "https://go.dev/blog/go1.24" {
		t.Errorf("a result behind DuckDuckGo's redirect should point at the page: %+v", results[1])
	}
}

func TestParseResultsTellsNoResultsFromAPageThatIsNotResults(t *testing.T) {
	for name, tc := range map[string]struct {
		page       string
		recognised bool
	}{
		"no results":     {`<html><body><div class="no-results">No results.</div></body></html>`, true},
		"challenge page": {`<html><body><form>Select all squares with a duck</form></body></html>`, false},
	} {
		results, recognised := parseResults(parse(t, tc.page))
		if recognised != tc.recognised || len(results) != 0 {
			t.Errorf("%s: recognised=%v results=%v", name, recognised, results)
		}
	}
}

// local is a Client that may reach httptest's 127.0.0.1 servers.
func local(endpoint string) *Client { return &Client{http: guardedClient(true), endpoint: endpoint} }

func TestSearchPostsTheQueryAndReturnsAtMostFiveResults(t *testing.T) {
	var got string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = r.ParseForm()
		got = r.PostForm.Get("q")
		w.Write([]byte(`<div id="links">`))
		for _, n := range []string{"a", "b", "c", "d", "e", "f", "g"} {
			w.Write([]byte(`<div class="result"><a class="result__a" href="https://example.com/` + n + `">T` + n + `</a></div>`))
		}
	}))
	defer srv.Close()
	results, err := local(srv.URL).Search(context.Background(), "  go release  ")
	if err != nil || got != "go release" || len(results) != maxResults {
		t.Fatalf("query %q, %d results, err %v", got, len(results), err)
	}
}

func TestSearchSaysSoWhenDuckDuckGoAsksForACheck(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusAccepted) }))
	defer srv.Close()
	if _, err := local(srv.URL).Search(context.Background(), "x"); err == nil || !strings.Contains(err.Error(), "202") {
		t.Fatalf("err = %v", err)
	}
	if _, err := local(srv.URL).Search(context.Background(), "  "); err == nil {
		t.Fatal("an empty search should be refused")
	}
}

var article = `<html><head><title> The Page </title><script>var a = "<p>not text</p>";</script></head><body>
<nav>Home About Contact</nav>
<article><h1>Heading</h1><p>First   paragraph
with a break.</p><style>p{color:red}</style><p>Second paragraph.</p><p>Second paragraph.</p>
<button>Share</button><p>` + longText + `</p></article><footer>Copyright</footer></body></html>`

var longText = strings.Repeat("Words fill the article so that it counts as the page. ", 10)

func TestReadKeepsTheArticleAndLeavesTheFurniture(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Write([]byte(article))
	}))
	defer srv.Close()
	page, err := local("").Read(context.Background(), srv.URL, 10000)
	if err != nil {
		t.Fatal(err)
	}
	if page.Title != "The Page" {
		t.Errorf("title %q", page.Title)
	}
	for _, want := range []string{"Heading\nFirst paragraph with a break.\nSecond paragraph.\n"} {
		if !strings.Contains(page.Text, want) {
			t.Errorf("text %q lacks %q", page.Text, want)
		}
	}
	for _, unwanted := range []string{"not text", "Home About", "Copyright", "Share", "color:red", "Second paragraph.\nSecond paragraph."} {
		if strings.Contains(page.Text, unwanted) {
			t.Errorf("text should not contain %q: %q", unwanted, page.Text)
		}
	}
}

func TestReadCutsToTheSizeAskedAtALineEnd(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		w.Write([]byte("line one\nline two\nline three is the long one\n"))
	}))
	defer srv.Close()
	page, err := local("").Read(context.Background(), srv.URL, 30)
	if err != nil || !page.Cut || page.Text != "line one\nline two…" {
		t.Fatalf("%+v %v", page, err)
	}
}

func TestReadRefusesWhatIsNotAReadablePage(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/pdf":
			w.Header().Set("Content-Type", "application/pdf")
			w.Write([]byte("%PDF"))
		case "/gone":
			http.NotFound(w, r)
		default:
			w.Header().Set("Content-Type", "text/html")
			w.Write([]byte(`<html><body><script>app()</script></body></html>`))
		}
	}))
	defer srv.Close()
	c := local("")
	for path, want := range map[string]string{"/pdf": "not a web page", "/gone": "404", "/empty": "no text"} {
		if _, err := c.Read(context.Background(), srv.URL+path, 1000); err == nil || !strings.Contains(err.Error(), want) {
			t.Errorf("%s: err = %v, want %q", path, err, want)
		}
	}
	for _, address := range []string{"file:///C:/Windows/win.ini", "ftp://example.com/x", "example.com", "http://"} {
		if _, err := c.Read(context.Background(), address, 1000); err == nil {
			t.Errorf("%q should be refused", address)
		}
	}
}

func TestTheGuardRefusesTheComputerAndTheLocalNetworkAndAllowsThePublicInternet(t *testing.T) {
	for address, refused := range map[string]bool{
		"127.0.0.1:80": true, "[::1]:80": true, "10.1.2.3:80": true, "192.168.0.10:443": true, "172.16.5.5:80": true,
		"169.254.169.254:80": true, "[fe80::1]:80": true, "[fd00::1]:80": true, "100.64.0.1:80": true, "0.0.0.0:80": true,
		"[::ffff:127.0.0.1]:80": true, "224.0.0.1:80": true,
		"93.184.216.34:443": false, "[2606:2800:220:1:248:1893:25c8:1946]:443": false, "8.8.8.8:53": false,
	} {
		err := refuseLocal("tcp", address, nil)
		if refused != errors.Is(err, errLocal) {
			t.Errorf("%s: refused=%v, err=%v", address, refused, err)
		}
	}
}

func TestTheRealClientCannotReadALocalServer(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.Write([]byte("secret")) }))
	defer srv.Close()
	_, err := New().Read(context.Background(), srv.URL, 1000)
	if err == nil || !strings.Contains(err.Error(), errLocal.Error()) {
		t.Fatalf("err = %v", err)
	}
}

func TestWhenDuckDuckGoAsksForACheckWikipediaAnswersAndTheSearchIsRemembered(t *testing.T) {
	var duck, wiki int
	blocked := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { duck++; w.WriteHeader(http.StatusAccepted) }))
	defer blocked.Close()
	encyclopedia := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		wiki++
		if r.URL.Query().Get("srsearch") != "Go language" || !strings.HasPrefix(r.Header.Get("User-Agent"), "Omni/") {
			t.Errorf("wikipedia was asked %q by %q", r.URL.RawQuery, r.Header.Get("User-Agent"))
		}
		w.Write([]byte(`{"query":{"search":[{"title":"Go (programming language)","snippet":"<span class=\"searchmatch\">Go</span> is a statically typed &amp; compiled language."}]}}`))
	}))
	defer encyclopedia.Close()
	c := &Client{http: guardedClient(true), endpoint: blocked.URL, wikiEndpoint: encyclopedia.URL}
	for range 2 {
		results, err := c.Search(context.Background(), "  Go   language ")
		if err != nil || len(results) != 1 {
			t.Fatalf("results %+v, err %v", results, err)
		}
		want := Result{Title: "Go (programming language) - Wikipedia", URL: "https://en.wikipedia.org/wiki/Go_%28programming_language%29",
			Snippet: "Go is a statically typed & compiled language."}
		if results[0] != want {
			t.Fatalf("result = %+v, want %+v", results[0], want)
		}
	}
	if duck != 1 || wiki != 1 {
		t.Fatalf("DuckDuckGo asked %d times and Wikipedia %d; the second search should come from memory", duck, wiki)
	}
}

func TestWhenNeitherAnswersTheErrorSaysDuckDuckGoAskedForACheck(t *testing.T) {
	blocked := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusAccepted) }))
	defer blocked.Close()
	empty := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.Write([]byte(`{"query":{"search":[]}}`)) }))
	defer empty.Close()
	c := &Client{http: guardedClient(true), endpoint: blocked.URL, wikiEndpoint: empty.URL}
	if _, err := c.Search(context.Background(), "x"); !errors.Is(err, errBlocked) {
		t.Fatalf("err = %v", err)
	}
	if _, ok := c.remembered("x"); ok {
		t.Error("a failed search was remembered")
	}
}
