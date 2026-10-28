package chat

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"omni/internal/llm"
)

// fakeLlama plays llama-server's API: it counts a word as a token and streams a scripted answer word by word.
type fakeLlama struct {
	*httptest.Server
	mu            sync.Mutex
	requests      []Request
	reasons       string // streamed as reasoning_content before the answer
	answer        string
	status        int           // when not 0, the completions route answers with it
	statusMessage string        // the error text of that answer (default: "the context is full")
	statusOnce    bool          // the status is used for one request only
	drop          int           // how many completions requests have their connection cut
	pause         time.Duration // between the pieces of the answer
	hold          chan struct{} // when set, the answer waits for it after the first piece
}

func newFakeLlama(t *testing.T) *fakeLlama {
	t.Helper()
	f := &fakeLlama{answer: "Hello there from the model"}
	f.Server = httptest.NewServer(http.HandlerFunc(f.handle))
	t.Cleanup(f.Close)
	return f
}

func (f *fakeLlama) endpoint() llm.Endpoint { return llm.Endpoint{URL: f.URL, Key: "k", Device: "gpu"} }

func (f *fakeLlama) last() Request {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.requests[len(f.requests)-1]
}

func (f *fakeLlama) handle(w http.ResponseWriter, r *http.Request) {
	if r.Header.Get("Authorization") != "Bearer k" {
		w.WriteHeader(http.StatusUnauthorized)
		return
	}
	switch r.URL.Path {
	case "/tokenize":
		var in struct{ Content string }
		_ = json.NewDecoder(r.Body).Decode(&in)
		tokens := make([]int, len(strings.Fields(in.Content)))
		_ = json.NewEncoder(w).Encode(map[string]any{"tokens": tokens})
	case "/v1/chat/completions":
		var req Request
		_ = json.NewDecoder(r.Body).Decode(&req)
		f.mu.Lock()
		f.requests = append(f.requests, req)
		status, message, reasons, answer, pause, hold := f.status, f.statusMessage, f.reasons, f.answer, f.pause, f.hold
		cut := f.drop > 0
		if cut {
			f.drop--
		}
		if status != 0 && f.statusOnce {
			f.status = 0
		}
		f.mu.Unlock()
		if cut {
			conn, _, _ := w.(http.Hijacker).Hijack()
			_ = conn.Close()
			return
		}
		if status != 0 {
			if message == "" {
				message = "the context is full"
			}
			w.WriteHeader(status)
			body, _ := json.Marshal(map[string]any{"error": map[string]string{"message": message}})
			_, _ = w.Write(body)
			return
		}
		w.Header().Set("Content-Type", "text/event-stream")
		flusher := w.(http.Flusher)
		send := func(delta map[string]string) {
			b, _ := json.Marshal(map[string]any{"choices": []any{map[string]any{"delta": delta}}})
			fmt.Fprintf(w, "data: %s\n\n", b)
			flusher.Flush()
			time.Sleep(pause)
		}
		for _, word := range strings.SplitAfter(reasons, " ") {
			if word != "" {
				send(map[string]string{"reasoning_content": word})
			}
		}
		for i, word := range strings.SplitAfter(answer, " ") {
			send(map[string]string{"content": word})
			if i == 0 && hold != nil {
				select {
				case <-hold:
				case <-r.Context().Done():
					return
				}
			}
		}
		fmt.Fprintf(w, "data: {\"choices\":[],\"usage\":{\"prompt_tokens\":%d,\"completion_tokens\":%d}}\n\n", 42, len(strings.Fields(answer)))
		fmt.Fprint(w, "data: [DONE]\n\n")
	default:
		http.NotFound(w, r)
	}
}

func TestCompleteStreamsTheAnswerAndTheReasoningSeparately(t *testing.T) {
	f := newFakeLlama(t)
	f.reasons, f.answer = "let me think ", "forty two"
	var text, thought strings.Builder
	usage, err := Complete(context.Background(), f.endpoint(), NewRequest([]Wire{{"user", "q"}}, 100, true),
		func(s string) { text.WriteString(s) }, func(s string) { thought.WriteString(s) })
	if err != nil {
		t.Fatal(err)
	}
	if text.String() != "forty two" || thought.String() != "let me think " || usage.Prompt != 42 || usage.Completion != 2 {
		t.Fatalf("text %q thinking %q usage %+v", text.String(), thought.String(), usage)
	}
}

func TestTheRequestSwitchesThinkingPerRequestAndStopsAtTheStrayTagOnlyWhenItIsOff(t *testing.T) {
	f := newFakeLlama(t)
	for _, thinking := range []bool{false, true} {
		if _, err := Complete(context.Background(), f.endpoint(), NewRequest([]Wire{{"user", "q"}}, 123, thinking), func(string) {}, func(string) {}); err != nil {
			t.Fatal(err)
		}
		got := f.last()
		if got.Template["enable_thinking"] != thinking || got.MaxTokens != 123 || !got.Stream || !got.StreamOptions["include_usage"] {
			t.Fatalf("thinking=%v: request %+v", thinking, got)
		}
		if (len(got.Stop) == 1 && got.Stop[0] == "</think>") != !thinking {
			t.Fatalf("thinking=%v: stop = %v", thinking, got.Stop)
		}
	}
}

func TestCompleteReturnsTheServersReasonForARefusal(t *testing.T) {
	f := newFakeLlama(t)
	f.status = http.StatusBadRequest
	_, err := Complete(context.Background(), f.endpoint(), NewRequest(nil, 10, false), func(string) {}, func(string) {})
	var se *StatusError
	if !asStatus(err, &se) || se.Status != 400 || !strings.Contains(se.Message, "context is full") {
		t.Fatalf("err = %v", err)
	}
}

func asStatus(err error, target **StatusError) bool {
	se, ok := err.(*StatusError)
	if ok {
		*target = se
	}
	return ok
}

func TestCancellingKeepsWhatArrivedAndIsNotAnError(t *testing.T) {
	f := newFakeLlama(t)
	f.hold = make(chan struct{}) // the answer stops after its first word until released
	ctx, cancel := context.WithCancel(context.Background())
	var text strings.Builder
	got := make(chan error, 1)
	go func() {
		_, err := Complete(ctx, f.endpoint(), NewRequest(nil, 10, false), func(s string) {
			text.WriteString(s)
			cancel()
		}, func(string) {})
		got <- err
	}()
	select {
	case err := <-got:
		if err != nil {
			t.Fatalf("a cancelled answer returned %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("Complete did not return after the context was cancelled")
	}
	if text.String() != "Hello " {
		t.Fatalf("kept %q, want the first word", text.String())
	}
}

func TestTokensCountsWhatTheServerSays(t *testing.T) {
	f := newFakeLlama(t)
	n, err := Tokens(context.Background(), f.endpoint(), "one two three")
	if err != nil || n != 3 {
		t.Fatalf("Tokens = %d, %v", n, err)
	}
	if _, err := Tokens(context.Background(), llm.Endpoint{URL: f.URL, Key: "wrong"}, "x"); err == nil {
		t.Fatal("a wrong key was accepted")
	}
}
