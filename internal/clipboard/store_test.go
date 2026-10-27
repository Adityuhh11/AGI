package clipboard

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

var t0 = time.Date(2026, 10, 19, 12, 0, 0, 0, time.UTC)

func open(t *testing.T) (*Store, string) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "Omni", "clipboard.json")
	s, err := OpenStore(path)
	if err != nil {
		t.Fatal(err)
	}
	return s, path
}

func texts(entries []Entry) string {
	var out []string
	for _, e := range entries {
		out = append(out, e.Text)
	}
	return strings.Join(out, "|")
}

func add(t *testing.T, s *Store, text string, minutes int) {
	t.Helper()
	if err := s.Add(text, "Code", t0.Add(time.Duration(minutes)*time.Minute)); err != nil {
		t.Fatal(err)
	}
}

func TestEntriesComeBackNewestFirstAndSurviveARestart(t *testing.T) {
	s, path := open(t)
	add(t, s, "first", 0)
	add(t, s, "second", 1)
	add(t, s, "third", 2)
	again, err := OpenStore(path)
	if err != nil {
		t.Fatal(err)
	}
	if got := texts(again.Search("", 10)); got != "third|second|first" {
		t.Fatalf("history %q", got)
	}
	if got := again.Search("", 2); len(got) != 2 {
		t.Fatalf("the limit is not honoured: %d entries", len(got))
	}
}

func TestCopyingTheSameTextAgainMovesItToTheTopInsteadOfAddingIt(t *testing.T) {
	s, _ := open(t)
	add(t, s, "a", 0)
	add(t, s, "b", 1)
	if err := s.Add("a", "chrome", t0.Add(2*time.Minute)); err != nil {
		t.Fatal(err)
	}
	got := s.Search("", 10)
	if texts(got) != "a|b" || got[0].Source != "chrome" {
		t.Fatalf("history %q, source of the newest %q", texts(got), got[0].Source)
	}
	if s.Len() != 2 {
		t.Fatalf("%d entries, want 2", s.Len())
	}
}

func TestSearchNeedsEveryWordAnywhereInAnyCase(t *testing.T) {
	s, _ := open(t)
	add(t, s, "Invoice for March, Kubernetes training", 0)
	add(t, s, "kubernetes pod yaml", 1)
	add(t, s, "lunch plans", 2)
	for _, tc := range []struct{ query, want string }{
		{"kubernetes", "kubernetes pod yaml|Invoice for March, Kubernetes training"},
		{"INVOICE kubernetes", "Invoice for March, Kubernetes training"},
		{"pod march", ""},
		{"  ", "lunch plans|kubernetes pod yaml|Invoice for March, Kubernetes training"},
		{"nothing like it", ""},
	} {
		if got := texts(s.Search(tc.query, 10)); got != tc.want {
			t.Errorf("Search(%q) = %q, want %q", tc.query, got, tc.want)
		}
	}
}

func TestTextThatIsOnlySpacesIsNotKept(t *testing.T) {
	s, _ := open(t)
	add(t, s, "  \n\t ", 0)
	if s.Len() != 0 {
		t.Fatal("a blank copy was kept")
	}
}

func TestTheOldestGoFirstWhenThereAreTooManyOrTooMuchOrTheyAreTooOld(t *testing.T) {
	s, _ := open(t)
	for i := 0; i < MaxEntries+5; i++ {
		add(t, s, fmt.Sprintf("entry %d", i), i)
	}
	got := s.Search("", MaxEntries+10)
	if len(got) != MaxEntries || got[0].Text != fmt.Sprintf("entry %d", MaxEntries+4) || got[len(got)-1].Text != "entry 5" {
		t.Fatalf("%d entries from %q to %q", len(got), got[0].Text, got[len(got)-1].Text)
	}

	big, _ := open(t)
	chunk := strings.Repeat("x", MaxText)
	for i := 0; i < 5; i++ {
		add(t, big, fmt.Sprintf("%d%s", i, chunk), i) // each a little over the 100 KB the watcher allows
	}
	if err := big.Add(strings.Repeat("y", MaxBytes), "", t0.Add(time.Hour)); err != nil {
		t.Fatal(err)
	}
	if kept := big.Search("", 100); len(kept) != 1 || !strings.HasPrefix(kept[0].Text, "yyyy") {
		t.Fatalf("%d entries kept; the byte limit must drop the older ones first", len(kept))
	}

	old, _ := open(t)
	add(t, old, "ancient", 0)
	add(t, old, "recent", int(MaxAge/time.Minute)+10)
	if got := texts(old.Search("", 10)); got != "recent" {
		t.Fatalf("history %q: entries older than %v must go", got, MaxAge)
	}
}

func TestDeleteTouchAndClear(t *testing.T) {
	s, path := open(t)
	add(t, s, "a", 0)
	add(t, s, "b", 1)
	add(t, s, "c", 2)
	a := s.Search("a", 1)[0]
	if err := s.Touch(a.ID, t0.Add(time.Hour)); err != nil {
		t.Fatal(err)
	}
	if got := texts(s.Search("", 10)); got != "a|c|b" {
		t.Fatalf("after touching a: %q", got)
	}
	b := s.Search("b", 1)[0]
	if err := s.Delete(b.ID); err != nil {
		t.Fatal(err)
	}
	if _, ok := s.Get(b.ID); ok || s.Len() != 2 {
		t.Fatalf("b is still there (%d entries)", s.Len())
	}
	if err := s.Delete(12345); err != nil {
		t.Fatalf("deleting what is not there: %v", err)
	}
	if err := s.Clear(); err != nil {
		t.Fatal(err)
	}
	again, err := OpenStore(path)
	if err != nil || again.Len() != 0 {
		t.Fatalf("after Clear: %d entries, %v", again.Len(), err)
	}
}

func TestADamagedFileIsSetAsideAndTheHistoryStartsEmpty(t *testing.T) {
	path := filepath.Join(t.TempDir(), "clipboard.json")
	if err := os.WriteFile(path, []byte(`{"version":1,"entries":[{"id":`), 0o600); err != nil {
		t.Fatal(err)
	}
	s, err := OpenStore(path)
	if err != nil || s.Len() != 0 {
		t.Fatalf("%d entries, %v", s.Len(), err)
	}
	aside, _ := filepath.Glob(path + ".corrupt-*")
	if len(aside) != 1 {
		t.Fatalf("the damaged file was not kept aside: %v", aside)
	}
}

func TestIdsAreUniqueEvenForCopiesInTheSameMillisecond(t *testing.T) {
	s, _ := open(t)
	for _, text := range []string{"a", "b", "c"} {
		if err := s.Add(text, "", t0); err != nil {
			t.Fatal(err)
		}
	}
	seen := map[int64]bool{}
	for _, e := range s.Search("", 10) {
		if seen[e.ID] {
			t.Fatalf("id %d twice", e.ID)
		}
		seen[e.ID] = true
	}
}
