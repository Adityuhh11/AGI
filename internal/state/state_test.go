package state

import (
	"fmt"
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

func TestNormaliseTag(t *testing.T) {
	tests := []struct {
		typed, want string
		ok          bool
	}{
		{"work", "work", true},
		{"#Work", "work", true},
		{"  My Project ", "my-project", true},
		{"q3 / budget!!", "q3-budget", true},
		{"café", "café", true},
		{"", "", false},
		{"#", "", false},
		{"!!!", "", false},
		{"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", "", false}, // longer than a tag may be
	}
	for _, tt := range tests {
		if got, ok := NormaliseTag(tt.typed); got != tt.want || ok != tt.ok {
			t.Errorf("NormaliseTag(%q) = %q, %v; want %q, %v", tt.typed, got, ok, tt.want, tt.ok)
		}
	}
}

func TestTagsToggleOnAndOffAndSurviveARestart(t *testing.T) {
	s, path := open(t)
	on, err := s.ToggleTag("file:C:\\a.pdf", "#Work")
	if err != nil || !on {
		t.Fatalf("first toggle = %v, %v", on, err)
	}
	if _, err := s.ToggleTag("app:code", "work"); err != nil {
		t.Fatal(err)
	}
	if _, err := s.ToggleTag("app:code", "dev tools"); err != nil {
		t.Fatal(err)
	}
	again, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	if got := strings.Join(again.Tagged("WORK"), "|"); got != "file:C:\\a.pdf|app:code" {
		t.Fatalf("tagged work = %q (in the order tagged, any spelling of the tag)", got)
	}
	if got := strings.Join(again.TagsOf("app:code"), "|"); got != "dev-tools|work" {
		t.Fatalf("tags of the app = %q", got)
	}
	on, err = again.ToggleTag("file:C:\\a.pdf", "work")
	if err != nil || on {
		t.Fatalf("second toggle = %v, %v; it must take the tag off", on, err)
	}
	if got := strings.Join(again.Tagged("work"), "|"); got != "app:code" {
		t.Fatalf("after removing: %q", got)
	}
	if _, err := again.ToggleTag("x", "!!!"); err == nil {
		t.Fatal("an unusable tag must be refused")
	}
}

func TestAnEmptiedTagDisappearsAndTagsAreListedByUse(t *testing.T) {
	s, _ := open(t)
	for _, id := range []string{"a", "b", "c"} {
		if _, err := s.ToggleTag(id, "common"); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := s.ToggleTag("a", "rare"); err != nil {
		t.Fatal(err)
	}
	if _, err := s.ToggleTag("a", "gone"); err != nil {
		t.Fatal(err)
	}
	if _, err := s.ToggleTag("a", "gone"); err != nil {
		t.Fatal(err)
	}
	var got []string
	for _, tag := range s.Tags() {
		got = append(got, fmt.Sprintf("%s:%d", tag.Name, tag.Count))
	}
	if want := "common:3|rare:1"; strings.Join(got, "|") != want {
		t.Fatalf("tags = %v, want %s (most used first, an emptied tag gone)", got, want)
	}
}

func TestPinsKeepTheirOrderToggleOffAndSurviveARestart(t *testing.T) {
	s, path := open(t)
	for _, id := range []string{"app:code", "file:C:\a.pdf", "quicklink:gh:"} {
		if on, err := s.TogglePin(id); err != nil || !on {
			t.Fatalf("pin %s = %v, %v", id, on, err)
		}
	}
	if on, err := s.TogglePin("file:C:\a.pdf"); err != nil || on {
		t.Fatalf("pinning a pinned thing = %v, %v; it must unpin", on, err)
	}
	again, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	if got := strings.Join(again.Pins(), "|"); got != "app:code|quicklink:gh:" {
		t.Fatalf("pins = %q, want the order pinned without the one taken off", got)
	}
}

func TestOnlyMaxPinsCanBePinnedAndTheRestStay(t *testing.T) {
	s, _ := open(t)
	for i := 0; i < MaxPins; i++ {
		if _, err := s.TogglePin(fmt.Sprintf("app:%d", i)); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := s.TogglePin("app:one-too-many"); err == nil {
		t.Fatal("a ninth pin must be refused")
	}
	if got := len(s.Pins()); got != MaxPins {
		t.Fatalf("%d pins stayed, want %d", got, MaxPins)
	}
	if on, err := s.TogglePin("app:3"); err != nil || on {
		t.Fatalf("unpinning at the limit = %v, %v", on, err)
	}
}

func TestAStateFileFromBeforePinsLoadsWithNoPins(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state.json")
	if err := os.WriteFile(path, []byte(`{"version":1,"uses":{"app:x":[1]},"learned":{},"tags":{"work":["app:x"]}}`), 0o644); err != nil {
		t.Fatal(err)
	}
	old, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(old.Pins()) != 0 || len(old.Tagged("work")) != 1 {
		t.Fatalf("pins %v, tagged %v", old.Pins(), old.Tagged("work"))
	}
}
