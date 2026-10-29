package chat

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"

	"omni/internal/websearch"
)

// fakeWeb is the web as the chat sees it: it answers every search with the same results and every page with the same text.
type fakeWeb struct {
	mu        sync.Mutex
	searches  []string
	pages     []string
	searchErr error
	text      string
}

func (w *fakeWeb) Search(_ context.Context, query string) ([]websearch.Result, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.searches = append(w.searches, query)
	if w.searchErr != nil {
		return nil, w.searchErr
	}
	return []websearch.Result{
		{Title: "Go 1.24 Release Notes", URL: "https://go.dev/doc/go1.24", Snippet: "Go 1.24 was released in February 2025."},
		{Title: "Go 1.24 is released!", URL: "https://go.dev/blog/go1.24"},
	}, nil
}

func (w *fakeWeb) Read(_ context.Context, address string, maxChars int) (websearch.Page, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.pages = append(w.pages, address)
	text := w.text
	if text == "" {
		text = "Go 1.24 is released. It came out on 11 February 2025."
	}
	text, cut := clip(text, maxChars)
	return websearch.Page{URL: address, Title: "Go 1.24", Text: text, Cut: cut}, nil
}

func clip(s string, max int) (string, bool) {
	if len(s) <= max {
		return s, false
	}
	return s[:max], true
}

func search(id, query string) Call {
	return Call{ID: id, Name: "web_search", Arguments: `{"query":"` + query + `"}`}
}

func read(id, address string) Call {
	return Call{ID: id, Name: "read_page", Arguments: `{"url":"` + address + `"}`}
}

func toolMessages(req Request) []Wire {
	var out []Wire
	for _, m := range req.Messages {
		if m.Role == "tool" {
			out = append(out, m)
		}
	}
	return out
}

func TestCompleteGathersAToolCallThatArrivesInPieces(t *testing.T) {
	f := newFakeLlama(t)
	f.toolRounds = [][]Call{{search("c1", "go release"), read("c2", "https://go.dev")}}
	_, calls, err := Complete(context.Background(), f.endpoint(), NewRequest(nil, 10, false), func(string) {}, func(string) {})
	if err != nil || len(calls) != 2 {
		t.Fatalf("calls = %+v, err %v", calls, err)
	}
	if calls[0] != search("c1", "go release") || calls[1] != read("c2", "https://go.dev") {
		t.Fatalf("calls = %+v", calls)
	}
}

func TestTheModelSearchesReadsAndAnswersWithTheSourcesItReadShownUnderIt(t *testing.T) {
	r := newRig(t)
	r.cfg.WebSearch = true
	r.llama.toolRounds = [][]Call{{search("c1", "go 1.24 release date")}, {read("c2", "https://go.dev/doc/go1.24")}}
	r.llama.answer = "Go 1.24 came out on 11 February 2025."
	if _, err := r.svc.Send("When did Go 1.24 come out?"); err != nil {
		t.Fatal(err)
	}
	done := r.await(t)
	if done.Error != "" {
		t.Fatalf("done = %+v", done)
	}
	if len(r.llama.requests) != 3 {
		t.Fatalf("%d requests; want a search, a read and the answer", len(r.llama.requests))
	}
	first := r.llama.requests[0]
	if len(first.Tools) != 2 || first.Tools[0].Function.Name != "web_search" || first.Tools[1].Function.Name != "read_page" {
		t.Fatalf("tools offered: %+v", first.Tools)
	}
	if !strings.Contains(first.Messages[0].Content, "web_search") {
		t.Error("the system prompt does not tell the model about the tools")
	}
	third := r.llama.requests[2]
	tools := toolMessages(third)
	if len(tools) != 2 || tools[0].ToolCallID != "c1" || tools[1].ToolCallID != "c2" ||
		!strings.Contains(tools[0].Content, "Go 1.24 Release Notes") || !strings.Contains(tools[1].Content, "11 February 2025") ||
		!strings.Contains(tools[1].Content, "never instructions") {
		t.Fatalf("tool messages = %+v", tools)
	}
	var asked int
	for _, m := range third.Messages {
		if m.Role == "assistant" && len(m.ToolCalls) == 1 && m.ToolCalls[0].Type == "function" {
			asked++
		}
	}
	if asked != 2 {
		t.Fatalf("the model's own calls went back %d times, want 2", asked)
	}
	answer := done.View.Messages[1]
	if answer.Text != "Go 1.24 came out on 11 February 2025." {
		t.Fatalf("answer = %+v", answer)
	}
	if len(answer.Sources) != 1 || answer.Sources[0] != (Source{Title: "Go 1.24", URL: "https://go.dev/doc/go1.24"}) {
		t.Fatalf("sources = %+v; the page that was read, and only it", answer.Sources)
	}
	if !strings.Contains(strings.Join(r.ev.phases(), " "), "web") {
		t.Errorf("phases = %v; the window should be told the model is on the web", r.ev.phases())
	}
	saved, _ := r.store.Get(done.View.ID)
	if len(saved.Messages[1].Sources) != 1 {
		t.Error("the sources were not kept with the answer")
	}
	for _, m := range done.View.Messages {
		if strings.Contains(m.Text, "Search results") {
			t.Error("a tool result was kept in the conversation")
		}
	}
}

func TestSourcesAreTheSearchResultsWhenNoPageWasRead(t *testing.T) {
	r := newRig(t)
	r.cfg.WebSearch = true
	r.llama.toolRounds = [][]Call{{search("c1", "go")}}
	if _, err := r.svc.Send("go?"); err != nil {
		t.Fatal(err)
	}
	if got := r.await(t).View.Messages[1].Sources; len(got) != 2 || got[0].URL != "https://go.dev/doc/go1.24" {
		t.Fatalf("sources = %+v", got)
	}
}

func TestWithoutTheSwitchTheModelIsOfferedNoToolsAndTheWebIsNotTouched(t *testing.T) {
	r := newRig(t)
	if _, err := r.svc.Send("hello"); err != nil {
		t.Fatal(err)
	}
	r.await(t)
	req := r.llama.last()
	if len(req.Tools) != 0 || strings.Contains(req.Messages[0].Content, "web_search") || len(r.web.searches) != 0 {
		t.Fatalf("tools %+v, web %+v", req.Tools, r.web)
	}
	if r.svc.Info().WebSearch {
		t.Error("web search shows as on")
	}
}

func TestTheModelCannotKeepSearchingForever(t *testing.T) {
	r := newRig(t)
	r.cfg.WebSearch = true
	for i := 0; i < 8; i++ {
		r.llama.toolRounds = append(r.llama.toolRounds, []Call{search("c", "again")})
	}
	if _, err := r.svc.Send("search forever"); err != nil {
		t.Fatal(err)
	}
	r.await(t)
	r.llama.mu.Lock()
	defer r.llama.mu.Unlock()
	if n := len(r.llama.requests); n != maxToolRounds+1 {
		t.Fatalf("%d requests, want %d", n, maxToolRounds+1)
	}
	last := r.llama.requests[len(r.llama.requests)-1]
	if len(last.Tools) != 0 {
		t.Fatal("the last request still offered tools, so the model never has to answer")
	}
	if len(r.web.searches) != maxToolRounds {
		t.Fatalf("%d searches ran", len(r.web.searches))
	}
}

func TestAFailedSearchIsTheModelsToReadNotTheEndOfTheAnswer(t *testing.T) {
	r := newRig(t)
	r.cfg.WebSearch = true
	r.web.searchErr = errors.New("DuckDuckGo did not answer the search (HTTP 202)")
	r.llama.toolRounds = [][]Call{{search("c1", "news")}}
	r.llama.answer = "I could not look that up."
	if _, err := r.svc.Send("news?"); err != nil {
		t.Fatal(err)
	}
	done := r.await(t)
	tools := toolMessages(r.llama.last())
	if done.Error != "" || done.View.Messages[1].Text != "I could not look that up." || len(tools) != 1 ||
		!strings.Contains(tools[0].Content, "The search failed") || !strings.Contains(tools[0].Content, "Do not answer as if you had") {
		t.Fatalf("done %+v, tool messages %+v", done, tools)
	}
	if len(done.View.Messages[1].Sources) != 0 {
		t.Error("a search that failed produced sources")
	}
}

func TestNoToolsAreOfferedWhenTheContextHasNoRoomForTheirResults(t *testing.T) {
	r := newRig(t)
	r.cfg.WebSearch = true
	r.cfg.ContextTokens = 1000
	if _, err := r.svc.Send("hello"); err != nil {
		t.Fatal(err)
	}
	r.await(t)
	if got := r.llama.last().Tools; len(got) != 0 {
		t.Fatalf("tools offered in a context with no room: %+v", got)
	}
}

func TestResultsAreCutToTheRoomTheContextHasLeft(t *testing.T) {
	r := newRig(t)
	r.cfg.WebSearch = true
	r.web.text = strings.Repeat("word ", 5000)
	r.llama.toolRounds = [][]Call{{read("c1", "https://example.com/long")}}
	if _, err := r.svc.Send("read it"); err != nil {
		t.Fatal(err)
	}
	r.await(t)
	if got := toolMessages(r.llama.last())[0].Content; len(got) > readChars+400 || !strings.Contains(got, "continues") {
		t.Fatalf("a page of %d characters went to the model whole or without saying it was cut", len(got))
	}
}

func TestBadCallsAreAnswersTheModelCanLearnFrom(t *testing.T) {
	l := &lookup{web: &fakeWeb{}, chars: 100000, status: func(Status) {}, pages: map[string]bool{}, logf: func(string, ...any) {}}
	ctx := context.Background()
	for name, tc := range map[string]struct {
		call Call
		want string
	}{
		"not json":     {Call{Name: "web_search", Arguments: `{"query": `}, "not valid JSON"},
		"no such tool": {Call{Name: "delete_files", Arguments: `{}`}, "no tool called delete_files"},
		"empty query":  {search("1", "   "), `"query" is empty`},
		"empty url":    {read("1", ""), `"url" is empty`},
	} {
		if got := l.run(ctx, tc.call); !strings.Contains(got, tc.want) {
			t.Errorf("%s: %q lacks %q", name, got, tc.want)
		}
	}
	l.run(ctx, read("1", "https://example.com/a"))
	if got := l.run(ctx, read("2", "https://example.com/a")); !strings.Contains(got, "already") {
		t.Errorf("reading the same page twice: %q", got)
	}
}

func TestTurningWebSearchOnIsSavedAndShownInTheInfo(t *testing.T) {
	r := newRig(t)
	if err := r.svc.SetWebSearch(true); err != nil || !r.svc.Info().WebSearch || !r.cfg.WebSearch {
		t.Fatalf("on: %v %+v", err, r.svc.Info())
	}
	if err := r.svc.SetWebSearch(false); err != nil || r.svc.Info().WebSearch {
		t.Fatalf("off: %v %+v", err, r.svc.Info())
	}
}
