package indexing

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"omni/internal/config"
	"omni/internal/sidecar"
)

// fakeEngine answers like the real one: each job needs two checks to finish.
type fakeEngine struct {
	mu         sync.Mutex
	sources    map[string]sidecar.Source
	jobs       map[string]int         // job id -> checks so far
	submitted  []string               // roots, in order
	excluded   map[string][]string    // what each root was told to leave out
	options    []sidecar.IndexOptions // what each sync was told, in order
	deleted    []string
	busyWith   string // while set, AddSource answers 409 for this job once
	failRoot   string // jobs for this root fail
	jobRoot    map[string]string
	duties     []float64 // what the engine was told, in order
	paused     bool
	pauseCalls int
}

func (f *fakeEngine) SetDuty(_ context.Context, duty float64) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.duties = append(f.duties, duty)
	return nil
}

func (f *fakeEngine) Pause(context.Context) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.paused = true
	f.pauseCalls++
	return nil
}

func (f *fakeEngine) Resume(context.Context) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.paused = false
	return nil
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

func (f *fakeEngine) AddSource(_ context.Context, root, id string, exclude []string, options sidecar.IndexOptions) (string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.busyWith != "" {
		job := f.busyWith
		f.busyWith = ""
		f.jobs[job] = 0
		return "", &sidecar.BusyError{JobID: job, Message: job + " is still running"}
	}
	f.submitted = append(f.submitted, root)
	if f.excluded == nil {
		f.excluded = map[string][]string{}
	}
	f.excluded[root] = exclude
	f.options = append(f.options, options)
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
	m.deleted = func(string) bool { return false } // the fake roots do not exist on disk
	return m, m.cycle(context.Background(), nil)
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

func TestARootThatWasDeletedIsForgottenAndNotSynced(t *testing.T) {
	fake := newFake(sidecar.Source{ID: config.SourceID(`C:\gone`), Root: `C:\gone`, Files: 9, Chunks: 99})
	m := New(fake, []string{`C:\gone`, `C:\kept`}, func() bool { return true }, nil)
	m.poll = time.Millisecond
	m.deleted = func(root string) bool { return root == `C:\gone` }
	if !m.cycle(context.Background(), nil) {
		t.Fatalf("cycle failed: %v", m.Status())
	}
	if len(fake.deleted) != 1 || fake.deleted[0] != config.SourceID(`C:\gone`) {
		t.Errorf("deleted sources %v, want the removed folder's", fake.deleted)
	}
	if len(fake.submitted) != 1 || fake.submitted[0] != `C:\kept` {
		t.Errorf("submitted %v, want only the kept root", fake.submitted)
	}
	if got := m.Status().Chunks; got != 10 {
		t.Errorf("totals count %d chunks, want the kept root's 10", got)
	}
}

func TestWasDeleted(t *testing.T) {
	dir := t.TempDir()
	here := filepath.Join(dir, "here")
	if err := os.Mkdir(here, 0o755); err != nil {
		t.Fatal(err)
	}
	tests := []struct {
		name string
		root string
		want bool
	}{
		{"a folder that exists", here, false},
		{"a folder removed from a disk that is still there", filepath.Join(dir, "removed"), true},
		{"a folder whose parent is missing too (an unplugged drive)", filepath.Join(dir, "no-drive", "removed"), false},
	}
	for _, tt := range tests {
		if got := wasDeleted(tt.root); got != tt.want {
			t.Errorf("%s: got %v, want %v", tt.name, got, tt.want)
		}
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
	if s := m.Status(); s.State != Failed || !strings.HasPrefix(s.Message, "a: ") {
		t.Errorf("status %+v", s)
	}
}

func TestRunSyncsAgainOnRescan(t *testing.T) {
	fake := newFake()
	m := New(fake, []string{`C:\a`}, func() bool { return true }, nil)
	m.poll = time.Millisecond
	m.deleted = func(string) bool { return false }
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

func TestAChangedFolderIsSyncedAloneAndAnUnchangedOneIsLeftBe(t *testing.T) {
	fake := newFake()
	roots := []string{`C:\a`, `C:\b`, `C:\c`}
	m := New(fake, roots, func() bool { return true }, nil)
	m.poll = time.Millisecond
	m.deleted = func(string) bool { return false }
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() { m.Run(ctx); close(done) }()
	defer func() { cancel(); <-done }()

	submitted := func() []string {
		fake.mu.Lock()
		defer fake.mu.Unlock()
		return append([]string(nil), fake.submitted...)
	}
	waitFor(t, func() bool { return len(submitted()) == 3 }) // the first, full sync

	m.Changed(`C:\b`)
	waitFor(t, func() bool { return len(submitted()) == 4 })
	m.Changed(`C:\b`)
	m.Changed(`C:\c`)
	waitFor(t, func() bool { return len(submitted()) >= 5 })
	time.Sleep(50 * time.Millisecond) // nothing else may follow
	got := submitted()
	if got[3] != `C:\b` || len(got) > 6 {
		t.Fatalf("submitted %v: after the full sync only the changed roots may be synced", got)
	}
	for _, root := range got[3:] {
		if root == `C:\a` {
			t.Fatalf("the unchanged root was synced again: %v", got)
		}
	}
	if s := m.Status(); s.State != UpToDate {
		t.Errorf("status after a background sync = %+v", s)
	}
}

func TestABackgroundSyncDoesNotFlashWaitingForTheEngine(t *testing.T) {
	fake := newFake()
	var states []State
	var mu sync.Mutex
	m := New(fake, []string{`C:\a`}, func() bool { return true }, func(s Status) {
		mu.Lock()
		defer mu.Unlock()
		states = append(states, s.State)
	})
	m.poll = time.Millisecond
	m.deleted = func(string) bool { return false }
	m.cycle(context.Background(), nil) // the first sync, once the engine is ready
	mu.Lock()
	states = nil
	mu.Unlock()
	m.cycle(context.Background(), map[string]bool{`C:\a`: true})
	mu.Lock()
	defer mu.Unlock()
	for _, s := range states {
		if s == Waiting {
			t.Fatalf("states during a background sync: %v", states)
		}
	}
}

func TestAFullSyncCoversWhatWasReportedBeforeIt(t *testing.T) {
	m := New(newFake(), []string{`C:\a`}, func() bool { return true }, nil)
	m.poll = time.Millisecond
	m.deleted = func(string) bool { return false }
	m.Changed(`C:\a`)
	m.cycle(context.Background(), nil)
	if left := m.takeDirty(); len(left) != 0 {
		t.Errorf("a full sync left %v marked as changed", left)
	}
}

func TestExcludedFoldersGoToTheEngineRelativeToTheirOwnRoot(t *testing.T) {
	fake := newFake()
	m := New(fake, []string{`C:\a`, `C:\b`}, func() bool { return true }, nil)
	m.poll = time.Millisecond
	m.deleted = func(string) bool { return false }
	m.SetFolders([]string{`C:\a`, `C:\b`}, []string{`C:\a\vendor\skills`, `C:\b\old`})
	if !m.cycle(context.Background(), nil) {
		t.Fatalf("cycle failed: %v", m.Status())
	}
	if got := fake.excluded[`C:\a`]; len(got) != 1 || got[0] != "vendor/skills" {
		t.Errorf("root a was told to exclude %v", got)
	}
	if got := fake.excluded[`C:\b`]; len(got) != 1 || got[0] != "old" {
		t.Errorf("root b was told to exclude %v", got)
	}
}

func TestEverySyncCarriesTheIndexOptions(t *testing.T) {
	fake := newFake()
	m := New(fake, []string{`C:\a`, `C:\b`}, func() bool { return true }, nil)
	m.poll = time.Millisecond
	m.deleted = func(string) bool { return false }
	want := sidecar.IndexOptions{PDFPages: "text", Photos: "name", Videos: "content"}
	m.SetOptions(want)
	if !m.cycle(context.Background(), nil) {
		t.Fatalf("cycle failed: %v", m.Status())
	}
	if len(fake.options) != 2 || fake.options[0] != want || fake.options[1] != want {
		t.Errorf("the engine was told %v, want %v for each root", fake.options, want)
	}
}

func TestTheRetryPauseDoublesWithEachFailureUpToALimit(t *testing.T) {
	m := New(newFake(), nil, func() bool { return true }, nil)
	tests := []struct {
		failures int
		want     time.Duration
	}{
		{1, time.Minute}, {2, 2 * time.Minute}, {3, 4 * time.Minute}, {4, 8 * time.Minute},
		{5, 15 * time.Minute}, {40, 15 * time.Minute},
	}
	for _, tt := range tests {
		if got := m.retryPause(tt.failures); got != tt.want {
			t.Errorf("after %d failures the pause is %v, want %v", tt.failures, got, tt.want)
		}
	}
}

func TestSetFoldersChangesWhatIsIndexedAndDropsWhatIsNoLongerListed(t *testing.T) {
	fake := newFake()
	m := New(fake, []string{`C:\a`, `C:\b`}, func() bool { return true }, nil)
	m.poll = time.Millisecond
	m.deleted = func(string) bool { return false }
	if !m.cycle(context.Background(), nil) {
		t.Fatalf("first cycle failed: %v", m.Status())
	}
	m.SetFolders([]string{`C:\b`, `C:\c`}, nil)
	select {
	case <-m.rescan:
	default:
		t.Fatal("changing the folders did not ask for a sync")
	}
	if !m.cycle(context.Background(), nil) {
		t.Fatalf("second cycle failed: %v", m.Status())
	}
	if _, kept := fake.sources[config.SourceID(`C:\a`)]; kept {
		t.Error("the folder that was taken out of the list is still in the index")
	}
	if _, added := fake.sources[config.SourceID(`C:\c`)]; !added {
		t.Error("the folder that was added was not indexed")
	}
}

func TestTheSpeedsAreMeasuredDutiesAndAnUnknownOneIsBalanced(t *testing.T) {
	for pace, want := range map[string]float64{"fast": 1.0, "balanced": 0.5, "gentle": 0.2, "": 0.5, "turbo": 0.5} {
		if got := Duty(pace); got != want {
			t.Errorf("Duty(%q) = %v, want %v", pace, got, want)
		}
	}
}

func TestPausingAndResumingTellTheEngineAndReportInTheStatus(t *testing.T) {
	fake := newFake()
	var changes int
	m := New(fake, []string{`C:\a`}, func() bool { return true }, func(Status) { changes++ })
	m.Pause()
	m.Pause() // asking again changes nothing
	if !m.Status().Paused || !fake.paused || fake.pauseCalls != 1 || changes != 1 {
		t.Fatalf("paused %v, engine paused %v, %d pause calls, %d changes", m.Status().Paused, fake.paused, fake.pauseCalls, changes)
	}
	if got := (Status{State: Indexing, Root: `C:\a`, FilesTotal: 3, Paused: true}).String(); got != "Indexing paused" {
		t.Fatalf("status line = %q", got)
	}
	m.Resume()
	if m.Status().Paused || fake.paused {
		t.Fatal("resuming left it paused")
	}
}

func TestAnEngineThatIsNotUpYetIsToldTheSpeedAndThePauseWhenTheSyncStarts(t *testing.T) {
	fake := newFake()
	up := false
	m := New(fake, []string{`C:\a`}, func() bool { return up }, nil)
	m.poll = time.Millisecond
	m.deleted = func(string) bool { return false }
	m.SetPace("gentle")
	m.Pause()
	if len(fake.duties) != 0 || fake.pauseCalls != 0 {
		t.Fatalf("the engine was told while it was not ready: %v, %d", fake.duties, fake.pauseCalls)
	}
	up = true
	if !m.cycle(context.Background(), nil) {
		t.Fatalf("cycle failed: %v", m.Status())
	}
	if len(fake.duties) == 0 || fake.duties[len(fake.duties)-1] != 0.2 || !fake.paused {
		t.Fatalf("after the sync started the engine has duties %v, paused %v", fake.duties, fake.paused)
	}
}

func TestEachFolderSaysWhereItIs(t *testing.T) {
	fake := newFake()
	fake.failRoot = `C:\bad`
	m := New(fake, []string{`C:\good`, `C:\bad`}, func() bool { return true }, nil)
	m.poll = time.Millisecond
	m.deleted = func(string) bool { return false }
	before := m.Folders()
	if len(before) != 2 || before[0].State != Waiting || before[1].State != Waiting {
		t.Fatalf("before the first sync: %+v", before)
	}
	m.cycle(context.Background(), nil)
	got := m.Folders()
	if got[0].Root != `C:\good` || got[0].State != UpToDate || got[0].Files != 3 || got[0].Chunks != 10 {
		t.Errorf("the good folder: %+v", got[0])
	}
	if got[1].Root != `C:\bad` || got[1].State != Failed || got[1].Message == "" {
		t.Errorf("the failing folder: %+v", got[1])
	}
	m.SetFolders([]string{`C:\good`}, nil)
	if after := m.Folders(); len(after) != 1 || after[0].Root != `C:\good` {
		t.Errorf("after removing a folder: %+v", after)
	}
}
