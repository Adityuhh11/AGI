package clipboard

import (
	"errors"
	"strings"
	"testing"
	"time"
)

// fakeSource is a clipboard that a test copies to.
type fakeSource struct {
	seq   uint32
	clip  Clip
	err   error
	reads int
}

func (f *fakeSource) Sequence() uint32 { return f.seq }
func (f *fakeSource) Read() (Clip, error) {
	f.reads++
	return f.clip, f.err
}

// copyTo plays a program copying: the counter moves and the content changes.
func (f *fakeSource) copyTo(c Clip) { f.seq++; f.clip, f.err = c, nil }

type harness struct {
	src     *fakeSource
	store   *Store
	watcher *Watcher
	enabled bool
	ignore  []string
	clock   time.Time
}

func newHarness(t *testing.T) *harness {
	t.Helper()
	store, _ := open(t)
	h := &harness{src: &fakeSource{seq: 7}, store: store, enabled: true, clock: t0}
	h.watcher = NewWatcher(h.src, store, func() bool { return h.enabled }, func() []string { return h.ignore }, "agi")
	h.watcher.now = func() time.Time { h.clock = h.clock.Add(time.Minute); return h.clock }
	return h
}

func (h *harness) kept() string { return texts(h.store.Search("", 100)) }

func TestWhatIsCopiedIsKeptWithItsProgram(t *testing.T) {
	h := newHarness(t)
	h.src.copyTo(Clip{Text: "hello", Owner: "Code"})
	h.watcher.Poll()
	got := h.store.Search("", 10)
	if len(got) != 1 || got[0].Text != "hello" || got[0].Source != "Code" {
		t.Fatalf("history %+v", got)
	}
}

func TestCopiesThatStayOutOfTheHistory(t *testing.T) {
	long := strings.Repeat("é", MaxText+1)
	for _, tc := range []struct {
		name string
		clip Clip
	}{
		{"marked private by the program (a password manager's copy)", Clip{Text: "hunter2", Owner: "somepm", Private: true}},
		{"only spaces", Clip{Text: " \n\t ", Owner: "Code"}},
		{"not text", Clip{Text: "", Owner: "Photoshop"}},
		{"longer than the limit, counted in characters", Clip{Text: long, Owner: "Code"}},
		{"a default password manager", Clip{Text: "pw", Owner: "1Password"}},
		{"a default password manager in another case", Clip{Text: "pw", Owner: "keepassxc"}},
		{"a program the user listed, written with .exe", Clip{Text: "pw", Owner: "MyVault"}},
		{"this program's own copy", Clip{Text: "C:\a\b.txt", Owner: "agi"}},
	} {
		h := newHarness(t)
		h.ignore = []string{" myvault.exe "}
		h.src.copyTo(tc.clip)
		h.watcher.Poll()
		if got := h.kept(); got != "" {
			t.Errorf("%s: kept %.20q", tc.name, got)
		}
	}
}

func TestAnUnknownProgramIsKeptAndTheLimitIsInCharactersNotBytes(t *testing.T) {
	h := newHarness(t)
	h.src.copyTo(Clip{Text: strings.Repeat("é", MaxText), Owner: ""})
	h.watcher.Poll()
	if h.store.Len() != 1 {
		t.Fatal("a text of exactly the limit, from a program that could not be named, was not kept")
	}
}

func TestNothingIsReadWhileTheCounterHasNotMoved(t *testing.T) {
	h := newHarness(t)
	h.src.copyTo(Clip{Text: "a"})
	h.watcher.Poll()
	h.watcher.Poll()
	h.watcher.Poll()
	if h.src.reads != 1 {
		t.Fatalf("%d reads, want 1", h.src.reads)
	}
}

func TestWhatWasOnTheClipboardBeforeTheWatcherStartedIsNotHistory(t *testing.T) {
	store, _ := open(t)
	src := &fakeSource{seq: 41, clip: Clip{Text: "was already there"}}
	w := NewWatcher(src, store, func() bool { return true }, func() []string { return nil }, "agi")
	w.Poll()
	if store.Len() != 0 || src.reads != 0 {
		t.Fatalf("%d entries, %d reads", store.Len(), src.reads)
	}
}

func TestCopiesMadeWhileHistoryIsOffAreNeverPickedUpLater(t *testing.T) {
	h := newHarness(t)
	h.enabled = false
	h.src.copyTo(Clip{Text: "private while off", Owner: "Code"})
	h.watcher.Poll()
	h.enabled = true
	h.watcher.Poll()
	if h.kept() != "" || h.src.reads != 0 {
		t.Fatalf("kept %q after %d reads", h.kept(), h.src.reads)
	}
	h.src.copyTo(Clip{Text: "after switching on", Owner: "Code"})
	h.watcher.Poll()
	if h.kept() != "after switching on" {
		t.Fatalf("kept %q", h.kept())
	}
}

func TestAClipboardHeldByAnotherProgramIsTriedAgainAtTheNextPoll(t *testing.T) {
	h := newHarness(t)
	h.src.copyTo(Clip{Text: "late"})
	h.src.err = errors.New("the clipboard is held open by another program")
	h.watcher.Poll()
	h.watcher.Poll()
	if h.kept() != "" || h.src.reads != 2 {
		t.Fatalf("kept %q after %d reads: a failed read must be retried", h.kept(), h.src.reads)
	}
	h.src.err = nil
	h.watcher.Poll()
	if h.kept() != "late" {
		t.Fatalf("kept %q", h.kept())
	}
}

func TestATextThatIsTooLongForTheSourceIsSkippedOnceNotRetried(t *testing.T) {
	h := newHarness(t)
	h.src.copyTo(Clip{})
	h.src.err = ErrTooLong
	h.watcher.Poll()
	h.watcher.Poll()
	if h.src.reads != 1 || h.kept() != "" {
		t.Fatalf("%d reads, kept %q", h.src.reads, h.kept())
	}
}
