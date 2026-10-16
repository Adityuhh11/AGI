package indexing

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"agi/internal/sidecar"
)

// fakeEngine answers like the real one: each job needs two checks to finish.
type fakeEngine struct {
	mu        sync.Mutex
	sources   map[string]sidecar.Source
	jobs      map[string]int // job id -> checks so far
	submitted []string       // roots, in order
	deleted   []string
	busyWith  string // while set, AddSource answers 409 for this job once
	failRoot  string // jobs for this root fail
	jobRoot   map[string]string
}

func newFake(existing ...sidecar.Source) *fakeEngine {
	f := &fakeEngine{sources: map[string]sidecar.Source{}, jobs: map[string]int{}, jobRoot: map[string]string{}}
	for _, s := range existing {
		f.sources[s.ID] = s
	}
	return f
}

func (f *fakeEngine) Sources(context.Context) ([]sidecar.Source, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	var out []sidecar.Source
	for _, s := range f.sources {
		out = append(out, s)
	}
	return out, nil
}

func (f *fakeEngine) AddSource(_ context.Context, root, id string) (string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.busyWith != "" {
		job := f.busyWith
		f.busyWith = ""
		f.jobs[job] = 0
		return "", &sidecar.BusyError{JobID: job, Message: job + " is still running"}
	}
	f.submitted = append(f.submitted, root)
	job := "job-" + id
	f.jobs[job], f.jobRoot[job] = 0, root
	f.sources[id] = sidecar.Source{ID: id, Root: root, Files: 3, Chunks: 10}
	return job, nil
}

func (f *fakeEngine) Job(_ context.Context, id string) (sidecar.Job, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.jobs[id]++
	job := sidecar.Job{ID: id, State: sidecar.JobRunning, FilesDone: f.jobs[id], FilesTotal: 3}
	if f.jobs[id] >= 3 {
		job.State = sidecar.JobDone
		if f.jobRoot[id] == f.failRoot && f.failRoot != "" {
			job.State, job.Error = sidecar.JobFailed, new(string)
			*job.Error = "disk on fire"
		}
	}
	return job, nil
}

func (f *fakeEngine) DeleteSource(_ context.Context, id string) (int, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.deleted = append(f.deleted, id)
	delete(f.sources, id)
	return 0, nil
}

// runOnce runs one cycle with fast polling.
func runOnce(t *testing.T, engine Engine, roots []string) (*Manager, bool) {
	t.Helper()
	m := New(engine, roots, func() bool { return true }, nil)
	m.poll = time.Millisecond
	return m, m.cycle(context.Background())
}

func TestCycleSyncsRootsInOrderAndTotalsThem(t *testing.T) {
	fake := newFake()
	roots := []string{`C:\a`, `C:\b`}
	m, ok := runOnce(t, fake, roots)
	if !ok {
		t.Fatalf("cycle failed: %v", m.Status())
	}
	if got := fake.submitted; len(got) != 2 || got[0] != roots[0] || got[1] != roots[1] {
		t.Errorf("submitted %v, want %v", got, roots)
	}
	want := Status{State: UpToDate, Files: 6, Chunks: 20}
	if m.Status() != want {
		t.Errorf("status %+v, want %+v", m.Status(), want)
	}
}

func TestCycleForgetsSourcesThatAreNoLongerConfigured(t *testing.T) {
	fake := newFake(sidecar.Source{ID: "root-old", Root: `C:\old`, Files: 9, Chunks: 99})
	m, ok := runOnce(t, fake, []string{`C:\a`})
	if !ok || len(fake.deleted) != 1 || fake.deleted[0] != "root-old" {
		t.Fatalf("deleted %v, status %+v", fake.deleted, m.Status())
	}
	if got := m.Status().Chunks; got != 10 {
		t.Errorf("totals include the forgotten source: %d chunks", got)
	}
}

func TestCycleWaitsForARunningJobThenSubmits(t *testing.T) {
	fake := newFake()
	fake.busyWith = "job-leftover"
	m, ok := runOnce(t, fake, []string{`C:\a`})
	if !ok || len(fake.submitted) != 1 {
		t.Fatalf("submitted %v, status %+v", fake.submitted, m.Status())
	}
}

func TestOneFailingRootDoesNotStopTheOthers(t *testing.T) {
	fake := newFake()
	fake.failRoot = `C:\a`
	m, ok := runOnce(t, fake, []string{`C:\a`, `C:\b`})
	if ok {
		t.Fatal("cycle reported success")
	}
	if len(fake.submitted) != 2 {
		t.Errorf("submitted %v, want both roots", fake.submitted)
	}
	if s := m.Status(); s.State != Failed || s.Message != "a: disk on fire" {
		t.Errorf("status %+v", s)
	}
}

func TestRunSyncsAgainOnRescan(t *testing.T) {
	fake := newFake()
	m := New(fake, []string{`C:\a`}, func() bool { return true }, nil)
	m.poll = time.Millisecond
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() { m.Run(ctx); close(done) }()
	submitted := func() int {
		fake.mu.Lock()
		defer fake.mu.Unlock()
		return len(fake.submitted)
	}
	waitFor(t, func() bool { return submitted() == 1 })
	m.Rescan()
	waitFor(t, func() bool { return submitted() == 2 })
	cancel()
	<-done
}

func TestStatusText(t *testing.T) {
	root := `C:\dev\Sample Data`
	tests := []struct {
		status Status
		want   string
	}{
		{Status{State: Waiting}, "Waiting for the search engine"},
		{Status{State: Indexing, Root: root}, "Scanning Sample Data"},
		{Status{State: Indexing, Root: root, FilesDone: 40, FilesTotal: 96}, "Indexing Sample Data: 40 / 96 files"},
		{Status{State: UpToDate, Files: 96, Chunks: 431}, "Up to date: 431 chunks in 96 files"},
		{Status{State: Failed, Message: "boom"}, "Indexing failed: boom"},
	}
	for _, tt := range tests {
		if got := tt.status.String(); got != tt.want {
			t.Errorf("%+v: got %q, want %q", tt.status, got, tt.want)
		}
	}
}

func waitFor(t *testing.T, condition func() bool) {
	t.Helper()
	for deadline := time.Now().Add(2 * time.Second); time.Now().Before(deadline); time.Sleep(time.Millisecond) {
		if condition() {
			return
		}
	}
	t.Fatal(errors.New("condition not met in time"))
}
