package survey

import (
	"context"
	"errors"
	"net/http"
	"sync"
	"testing"
	"time"

	"omni/internal/sidecar"
)

// fakeEngine plays the engine's /survey: what it was asked, and what it answers.
type fakeEngine struct {
	mu     sync.Mutex
	asked  []string
	excl   map[string][]string
	answer func(root string) (sidecar.Survey, error)
}

func (f *fakeEngine) Survey(_ context.Context, root string, exclude []string) (sidecar.Survey, error) {
	f.mu.Lock()
	f.asked = append(f.asked, root)
	if f.excl == nil {
		f.excl = map[string][]string{}
	}
	f.excl[root] = exclude
	f.mu.Unlock()
	return f.answer(root)
}

func (f *fakeEngine) count() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return len(f.asked)
}

func counted(files int) func(string) (sidecar.Survey, error) {
	return func(string) (sidecar.Survey, error) {
		return sidecar.Survey{Routes: map[string]sidecar.RouteCount{"code": {Files: files, Bytes: 100}}}, nil
	}
}

func run(t *testing.T, e *fakeEngine, ready func() bool) *Manager {
	t.Helper()
	m := New(e, ready)
	m.poll, m.retry = time.Millisecond, 20*time.Millisecond
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() { m.Run(ctx); close(done) }()
	t.Cleanup(func() { cancel(); <-done })
	return m
}

func eventually(t *testing.T, what string, ok func() bool) {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for !ok() {
		if time.Now().After(deadline) {
			t.Fatalf("timed out waiting for %s", what)
		}
		time.Sleep(2 * time.Millisecond)
	}
}

func TestEveryFolderIsCountedOnceTheEngineIsUp(t *testing.T) {
	e := &fakeEngine{answer: counted(7)}
	var mu sync.Mutex
	up := false
	m := run(t, e, func() bool { mu.Lock(); defer mu.Unlock(); return up })
	m.Set([]string{`C:\a`, `C:\b`}, nil)
	time.Sleep(20 * time.Millisecond)
	if got := m.Results()[`C:\a`].State; got != Pending || e.count() != 0 {
		t.Fatalf("before the engine is up: %v, asked %d", got, e.count())
	}
	mu.Lock()
	up = true
	mu.Unlock()
	eventually(t, "both folders", func() bool { r := m.Results(); return r[`C:\a`].State == Done && r[`C:\b`].State == Done })
	if got := m.Results()[`C:\a`].Survey.Routes["code"].Files; got != 7 {
		t.Fatalf("files = %d", got)
	}
	if e.count() != 2 {
		t.Fatalf("asked %d times, want once per folder", e.count())
	}
}

func TestChangingWhatIsLeftOutCountsOnlyThatFolderAgainAndRemovingForgets(t *testing.T) {
	e := &fakeEngine{answer: counted(3)}
	m := run(t, e, func() bool { return true })
	m.Set([]string{`C:\a`, `C:\b`}, nil)
	eventually(t, "first pass", func() bool { return e.count() == 2 })
	m.Set([]string{`C:\a`, `C:\b`}, []string{`C:\a\big`})
	eventually(t, "the changed folder again", func() bool { return e.count() == 3 })
	time.Sleep(30 * time.Millisecond)
	if e.count() != 3 {
		t.Fatalf("asked %d times: the untouched folder was counted again", e.count())
	}
	e.mu.Lock()
	got := e.excl[`C:\a`]
	e.mu.Unlock()
	if len(got) != 1 || got[0] != "big" {
		t.Fatalf("left out = %v, want the folder relative to its root", got)
	}
	m.Set([]string{`C:\b`}, nil)
	if _, there := m.Results()[`C:\a`]; there {
		t.Fatal("a removed folder still has a result")
	}
}

func TestAnOlderEngineIsUnavailableAndAFailureIsTriedAgain(t *testing.T) {
	var mu sync.Mutex
	calls := 0
	e := &fakeEngine{answer: func(root string) (sidecar.Survey, error) {
		mu.Lock()
		defer mu.Unlock()
		calls++
		if root == `C:\old` {
			return sidecar.Survey{}, &sidecar.APIError{Status: http.StatusNotFound, Detail: "Not Found"}
		}
		if calls < 4 {
			return sidecar.Survey{}, errors.New("disk on fire")
		}
		return sidecar.Survey{}, nil
	}}
	m := run(t, e, func() bool { return true })
	m.Set([]string{`C:\old`}, nil)
	eventually(t, "unavailable", func() bool { return m.Results()[`C:\old`].State == Unavailable })
	m.Set([]string{`C:\flaky`}, nil)
	eventually(t, "a failure", func() bool { return m.Results()[`C:\flaky`].State == Failed })
	if m.Results()[`C:\flaky`].Message == "" {
		t.Error("a failure must say why")
	}
	eventually(t, "the retry", func() bool { return m.Results()[`C:\flaky`].State == Done })
}
