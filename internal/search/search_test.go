package search

import (
	"context"
	"errors"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"agi/internal/sidecar"
)

type fakeEngine struct {
	respond func(ctx context.Context, req sidecar.SearchFilesRequest) (sidecar.SearchFilesResponse, error)
	asked   []sidecar.SearchFilesRequest
}

func (f *fakeEngine) SearchFiles(ctx context.Context, req sidecar.SearchFilesRequest) (sidecar.SearchFilesResponse, error) {
	f.asked = append(f.asked, req)
	return f.respond(ctx, req)
}

func ready() sidecar.Status { return sidecar.Status{State: sidecar.StateReady} }

func chunk(kind, locator, snippet string) sidecar.Chunk {
	return sidecar.Chunk{Kind: kind, Locator: locator, Snippet: snippet}
}

func TestRowsAreMappedOntoDiskAndDescribeTheirMatches(t *testing.T) {
	root := filepath.Join(`C:\`, "Data")
	engine := &fakeEngine{respond: func(context.Context, sidecar.SearchFilesRequest) (sidecar.SearchFilesResponse, error) {
		return sidecar.SearchFilesResponse{Confident: true, Files: []sidecar.File{
			{SourceID: "s1", URI: "Backend/src/jwt.js", Kind: "code", Score: 0.81, Chunks: []sidecar.Chunk{
				chunk("code", `{"lines":[12,52]}`, "function sign()"),
				chunk("code", `{"lines":[90,90]}`, "export"),
			}},
			{SourceID: "s1", URI: "Week 10 Prep.pdf", Kind: "pdf_page", Score: 0.7, Chunks: []sidecar.Chunk{
				chunk("pdf_page", `{"page":3}`, ""),
			}},
			{SourceID: "s1", URI: "pics/logo.png", Kind: "image", Score: 0.6, Chunks: []sidecar.Chunk{
				chunk("image", `{}`, "pics/logo.png"),
			}},
			{SourceID: "s1", URI: "NextPlore/README.md", Kind: "markdown", Score: 0.5, Chunks: []sidecar.Chunk{
				chunk("markdown", `{"lines":[1,40]}`, "# NextPlore"),
			}},
			{SourceID: "gone", URI: "orphan.txt", Kind: "text", Score: 0.4},
		}}, nil
	}}
	got := New(engine, map[string]string{"s1": root}, ready).Search("jwt", 1)

	if got.State != StateResults || !got.Confident || got.Stale || got.Token != 1 {
		t.Fatalf("results = %+v", got)
	}
	if len(got.Rows) != 4 {
		t.Fatalf("got %d rows, want 4 (the file of an unknown source is dropped): %+v", len(got.Rows), got.Rows)
	}

	first := got.Rows[0]
	if first.Path != filepath.Join(root, "Backend", "src", "jwt.js") || first.Name != "jwt.js" ||
		first.Folder != filepath.Join(root, "Backend", "src") || first.Kind != "code" || first.Line != 12 {
		t.Fatalf("first row = %+v", first)
	}
	if len(first.Matches) != 2 || first.Matches[0].Location != "lines 12-52" || first.Matches[1].Location != "line 90" ||
		first.Matches[0].Snippet != "function sign()" || first.Matches[0].Line != 12 || first.Matches[1].Line != 90 {
		t.Fatalf("matches = %+v", first.Matches)
	}

	kinds := []string{got.Rows[1].Kind, got.Rows[2].Kind, got.Rows[3].Kind}
	if kinds[0] != "pdf" || kinds[1] != "image" || kinds[2] != "text" {
		t.Fatalf("kinds = %v", kinds)
	}
	if got.Rows[1].Matches[0].Location != "page 3" || got.Rows[1].Line != 0 {
		t.Fatalf("pdf row = %+v", got.Rows[1])
	}
	if got.Rows[2].Matches[0].Location != "" {
		t.Fatalf("an image has no location, got %q", got.Rows[2].Matches[0].Location)
	}
}

func TestTheEngineIsAskedForTenFilesWithThreeChunksEach(t *testing.T) {
	engine := &fakeEngine{respond: func(context.Context, sidecar.SearchFilesRequest) (sidecar.SearchFilesResponse, error) {
		return sidecar.SearchFilesResponse{}, nil
	}}
	New(engine, nil, ready).Search("  find me  ", 1)
	if len(engine.asked) != 1 {
		t.Fatalf("asked %d times", len(engine.asked))
	}
	if req := engine.asked[0]; req.Query != "find me" || req.K != 10 || req.PerFile != 3 {
		t.Fatalf("request = %+v", req)
	}
}

func TestABlankQueryDoesNotBotherTheEngine(t *testing.T) {
	engine := &fakeEngine{respond: func(context.Context, sidecar.SearchFilesRequest) (sidecar.SearchFilesResponse, error) {
		t.Fatal("the engine must not be asked about a blank query")
		return sidecar.SearchFilesResponse{}, nil
	}}
	svc := New(engine, nil, ready)
	for _, query := range []string{"", "   ", "\t\n"} {
		if got := svc.Search(query, 1); got.State != StateResults || len(got.Rows) != 0 {
			t.Fatalf("Search(%q) = %+v", query, got)
		}
	}
}

func TestSearchReportsWhyItCannotAnswer(t *testing.T) {
	never := &fakeEngine{respond: func(context.Context, sidecar.SearchFilesRequest) (sidecar.SearchFilesResponse, error) {
		t.Fatal("the engine must not be called while it is not ready")
		return sidecar.SearchFilesResponse{}, nil
	}}
	tests := []struct {
		name      string
		status    sidecar.Status
		wantState string
		wantMsg   string
	}{
		{"starting", sidecar.Status{State: sidecar.StateStarting}, StateWarming, "starting up"},
		{"warming", sidecar.Status{State: sidecar.StateWarming}, StateWarming, "starting up"},
		{"restarting", sidecar.Status{State: sidecar.StateRestarting, Message: "it crashed"}, StateWarming, "starting up"},
		{"failed", sidecar.Status{State: sidecar.StateFailed, Message: "run `uv sync`"}, StateUnavailable, "run `uv sync`"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc := New(never, nil, func() sidecar.Status { return tt.status })
			got := svc.Search("x", 5)
			if got.State != tt.wantState || got.Token != 5 || !strings.Contains(got.Message, tt.wantMsg) {
				t.Fatalf("got %+v, want state %q with %q in the message", got, tt.wantState, tt.wantMsg)
			}
		})
	}
}

func TestEngineErrorsBecomeStatesNotCrashes(t *testing.T) {
	tests := []struct {
		name      string
		err       error
		wantState string
	}{
		{"model still loading", &sidecar.APIError{Status: 503, Detail: "the model is still loading"}, StateWarming},
		{"a failing search", &sidecar.APIError{Status: 500, Detail: "boom"}, StateError},
		{"a dead connection", errors.New("connection refused"), StateError},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			engine := &fakeEngine{respond: func(context.Context, sidecar.SearchFilesRequest) (sidecar.SearchFilesResponse, error) {
				return sidecar.SearchFilesResponse{}, tt.err
			}}
			if got := New(engine, nil, ready).Search("x", 1); got.State != tt.wantState {
				t.Fatalf("state = %q, want %q (%+v)", got.State, tt.wantState, got)
			}
		})
	}
}

func TestAnOlderSearchIsCancelledAndItsAnswerMarkedStale(t *testing.T) {
	started := make(chan struct{})
	engine := &fakeEngine{}
	engine.respond = func(ctx context.Context, req sidecar.SearchFilesRequest) (sidecar.SearchFilesResponse, error) {
		if req.Query == "slow" {
			close(started)
			<-ctx.Done() // a request that is only ever cancelled
			return sidecar.SearchFilesResponse{}, ctx.Err()
		}
		return sidecar.SearchFilesResponse{Confident: true}, nil
	}
	svc := New(engine, nil, ready)

	old := make(chan Results, 1)
	go func() { old <- svc.Search("slow", 1) }()
	<-started
	fresh := svc.Search("fast", 2)

	select {
	case got := <-old:
		if !got.Stale || got.Token != 1 {
			t.Fatalf("the cancelled search returned %+v, want a stale answer", got)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("the older search was not cancelled by the newer one")
	}
	if fresh.Stale || fresh.Token != 2 || !fresh.Confident {
		t.Fatalf("the newer search returned %+v", fresh)
	}
}

func TestAnAnswerThatArrivesAfterANewerRequestIsStaleEvenIfNotCancelled(t *testing.T) {
	release := make(chan struct{})
	engine := &fakeEngine{respond: func(_ context.Context, req sidecar.SearchFilesRequest) (sidecar.SearchFilesResponse, error) {
		if req.Query == "old" {
			<-release // ignores cancellation, like a client that finished just in time
		}
		return sidecar.SearchFilesResponse{Confident: true}, nil
	}}
	svc := New(engine, nil, ready)
	old := make(chan Results, 1)
	go func() { old <- svc.Search("old", 1) }()
	for i := 0; i < 200; i++ { // wait until the old request is in flight
		svc.mu.Lock()
		inFlight := svc.latest == 1
		svc.mu.Unlock()
		if inFlight {
			break
		}
		time.Sleep(5 * time.Millisecond)
	}
	svc.Search("new", 2)
	close(release)
	if got := <-old; !got.Stale {
		t.Fatalf("the answer to an outdated token must be stale, got %+v", got)
	}
}

func TestLocations(t *testing.T) {
	tests := []struct {
		locator string
		want    string
		line    int
	}{
		{`{"lines":[12,52]}`, "lines 12-52", 12},
		{`{"lines":[7,7]}`, "line 7", 7},
		{`{"page":3}`, "page 3", 0},
		{`{}`, "", 0},
		{``, "", 0},
		{`not json`, "", 0},
		{`{"lines":[1]}`, "", 0},
	}
	for _, tt := range tests {
		if got := location(tt.locator); got != tt.want {
			t.Errorf("location(%q) = %q, want %q", tt.locator, got, tt.want)
		}
		if got := firstLine(tt.locator); got != tt.line {
			t.Errorf("firstLine(%q) = %d, want %d", tt.locator, got, tt.line)
		}
	}
}

func TestDisplayKinds(t *testing.T) {
	want := map[string]string{
		"code": "code", "markdown": "text", "text": "text",
		"pdf_text": "pdf", "pdf_page": "pdf", "image": "image", "something new": "text",
	}
	for kind, expected := range want {
		if got := displayKind(kind); got != expected {
			t.Errorf("displayKind(%q) = %q, want %q", kind, got, expected)
		}
	}
}
