package state

import (
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

var t0 = time.Date(2026, 10, 17, 12, 0, 0, 0, time.UTC)

func open(t *testing.T) (*Store, string) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "AGI", "state.json")
	s, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	return s, path
}

func TestRecordedUsesAndLearnedPrefixesSurviveARestart(t *testing.T) {
	s, path := open(t)
	if err := s.Record("app:code", "VSC ", t0); err != nil {
		t.Fatal(err)
	}
	if err := s.Record("app:code", "vsc", t0.Add(time.Hour)); err != nil {
		t.Fatal(err)
	}
	again, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	uses := again.Uses("app:code")
	if len(uses) != 2 || !uses[0].Equal(t0) || !uses[1].Equal(t0.Add(time.Hour)) {
		t.Fatalf("uses = %v", uses)
	}
	if got := again.Learned(" Vsc", "app:code"); got != 2 {
		t.Fatalf("learned = %d, want 2 (case and edges ignored)", got)
	}
	if got := again.Learned("vsc", "app:other"); got != 0 {
		t.Fatalf("an unrelated thing learned %d", got)
	}
}

func TestOnlyTheRecentUsesAreKeptAndLongTextIsNotLearned(t *testing.T) {
	s, _ := open(t)
	for i := 0; i < 30; i++ {
		if err := s.Record("a", "", t0.Add(time.Duration(i)*time.Minute)); err != nil {
			t.Fatal(err)
		}
	}
	uses := s.Uses("a")
	if len(uses) != maxUses || !uses[len(uses)-1].Equal(t0.Add(29*time.Minute)) {
		t.Fatalf("kept %d uses, last %v", len(uses), uses[len(uses)-1])
	}
	long := strings.Repeat("x", maxTyped+1)
	if err := s.Record("a", long, t0); err != nil {
		t.Fatal(err)
	}
	if s.Learned(long, "a") != 0 {
		t.Fatal("a typed text longer than a shortcut must not be learned")
	}
	if s.Learned("", "a") != 0 {
		t.Fatal("nothing typed must not be learned")
	}
}

func TestADamagedFileIsSetAsideAndTheStoreStartsEmpty(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state.json")
	if err := os.WriteFile(path, []byte("{broken"), 0o644); err != nil {
		t.Fatal(err)
	}
	s, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(s.Uses("anything")) != 0 {
		t.Fatal("a damaged file must give an empty store")
	}
	aside, _ := filepath.Glob(path + ".corrupt-*")
	if len(aside) != 1 {
		t.Fatalf("the damaged file was not kept aside: %v", aside)
	}
	if err := s.Record("a", "", t0); err != nil {
		t.Fatalf("the store must work after a damaged file: %v", err)
	}
}

func TestConcurrentRecordsAreAllKept(t *testing.T) {
	s, path := open(t)
	var wg sync.WaitGroup
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := s.Record("a", "x", t0); err != nil {
				t.Error(err)
			}
		}()
	}
	wg.Wait()
	if got := s.Learned("x", "a"); got != 20 {
		t.Fatalf("learned %d of 20", got)
	}
	again, err := Open(path)
	if err != nil || again.Learned("x", "a") != 20 {
		t.Fatalf("after a restart: %v, %d", err, again.Learned("x", "a"))
	}
}
