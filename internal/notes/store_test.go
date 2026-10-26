package notes

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func newStore(t *testing.T) *Store {
	t.Helper()
	s, err := Open(filepath.Join(t.TempDir(), "notes"))
	if err != nil {
		t.Fatal(err)
	}
	clock := time.Date(2026, 10, 26, 23, 15, 0, 0, time.Local)
	s.now = func() time.Time { clock = clock.Add(time.Second); return clock }
	return s
}

func TestANewNoteIsSavedAsAFileAndComesBack(t *testing.T) {
	s := newStore(t)
	n, err := s.Save("", "# Shopping\n\n- milk\n- eggs\n")
	if err != nil {
		t.Fatal(err)
	}
	if n.ID != "20261026-231501" || n.Title != "Shopping" || n.Preview != "milk eggs" {
		t.Fatalf("note = %+v", n)
	}
	data, err := os.ReadFile(filepath.Join(s.Dir(), n.ID+".md"))
	if err != nil || string(data) != "# Shopping\n\n- milk\n- eggs\n" {
		t.Fatalf("file = %q, %v", data, err)
	}
	got, err := s.Get(n.ID)
	if err != nil || got.Text != "# Shopping\n\n- milk\n- eggs\n" {
		t.Fatalf("Get = %+v, %v", got, err)
	}
}

func TestSavingAgainReplacesTheTextAndLeavesNoTempFile(t *testing.T) {
	s := newStore(t)
	n, _ := s.Save("", "first")
	if _, err := s.Save(n.ID, "second"); err != nil {
		t.Fatal(err)
	}
	got, _ := s.Get(n.ID)
	if got.Text != "second" {
		t.Fatalf("text = %q", got.Text)
	}
	entries, _ := os.ReadDir(s.Dir())
	if len(entries) != 1 {
		t.Fatalf("folder holds %d files, want 1", len(entries))
	}
}

func TestANoteOfOnlyWhitespaceIsNotMade(t *testing.T) {
	s := newStore(t)
	n, err := s.Save("", " \n\t\n")
	if err != nil || n.ID != "" {
		t.Fatalf("note = %+v, err %v", n, err)
	}
	if list, _ := s.List(); len(list) != 0 {
		t.Fatalf("listed %v", list)
	}
}

func TestTwoNotesInTheSameSecondGetDifferentIds(t *testing.T) {
	s := newStore(t)
	s.now = func() time.Time { return time.Date(2026, 1, 2, 3, 4, 5, 0, time.Local) }
	a, _ := s.Save("", "a")
	b, _ := s.Save("", "b")
	c, _ := s.Save("", "c")
	if a.ID != "20260102-030405" || b.ID != "20260102-030405-2" || c.ID != "20260102-030405-3" {
		t.Fatalf("ids %s %s %s", a.ID, b.ID, c.ID)
	}
}

func TestListIsNewestChangeFirstAndWithoutText(t *testing.T) {
	s := newStore(t)
	a, _ := s.Save("", "alpha")
	b, _ := s.Save("", "beta")
	older := time.Now().Add(-time.Hour)
	if err := os.Chtimes(filepath.Join(s.Dir(), a.ID+".md"), older, older); err != nil {
		t.Fatal(err)
	}
	list, err := s.List()
	if err != nil || len(list) != 2 || list[0].ID != b.ID || list[1].ID != a.ID {
		t.Fatalf("list = %+v, %v", list, err)
	}
	if list[0].Text != "" {
		t.Error("List must not carry every note's text")
	}
	if _, err := s.Save(a.ID, "alpha, edited"); err != nil {
		t.Fatal(err)
	}
	list, _ = s.List()
	if list[0].ID != a.ID {
		t.Errorf("an edited note should come first: %+v", list)
	}
}

func TestDeleteRemovesTheFileAndForgivesAMissingOne(t *testing.T) {
	s := newStore(t)
	n, _ := s.Save("", "gone soon")
	if err := s.Delete(n.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Get(n.ID); err == nil {
		t.Error("the note is still there")
	}
	if err := s.Delete(n.ID); err != nil {
		t.Errorf("deleting twice: %v", err)
	}
}

func TestAnIdCannotLeaveTheFolder(t *testing.T) {
	s := newStore(t)
	outside := filepath.Join(filepath.Dir(s.Dir()), "secret.md")
	if err := os.WriteFile(outside, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	for _, id := range []string{"../secret", `..\secret`, "secret", "", "20261026-231501/../../secret", "C:/x", "20261026-231501.md"} {
		if _, err := s.Get(id); err == nil {
			t.Errorf("Get(%q) was allowed", id)
		}
		if _, err := s.Save(id, "overwritten"); id != "" && err == nil {
			t.Errorf("Save(%q) was allowed", id)
		}
		if err := s.Delete(id); err == nil {
			t.Errorf("Delete(%q) was allowed", id)
		}
	}
	if data, _ := os.ReadFile(outside); string(data) != "x" {
		t.Fatalf("a file outside the folder was changed: %q", data)
	}
}

func TestSearchNeedsEveryWordAnywhereInTheNote(t *testing.T) {
	s := newStore(t)
	s.Save("", "# Trip\nflights to Lisbon in May")
	s.Save("", "# Groceries\nmilk and eggs")
	s.Save("", "# Reading\nthe Lisbon earthquake, 1755")
	tests := []struct {
		words string
		want  int
	}{
		{"lisbon", 2}, {"LISBON may", 1}, {"milk eggs", 1}, {"nothing here", 0}, {"", 3}, {"   ", 3},
	}
	for _, tt := range tests {
		got, err := s.Search(tt.words, 0)
		if err != nil || len(got) != tt.want {
			t.Errorf("Search(%q) found %d, want %d (%v)", tt.words, len(got), tt.want, err)
		}
	}
	if got, _ := s.Search("", 2); len(got) != 2 {
		t.Errorf("the limit was ignored: %d", len(got))
	}
}

func TestOtherFilesInTheFolderAreIgnored(t *testing.T) {
	s := newStore(t)
	os.WriteFile(filepath.Join(s.Dir(), "readme.txt"), []byte("x"), 0o644)
	os.WriteFile(filepath.Join(s.Dir(), "20261026-231501.md.tmp"), []byte("x"), 0o644)
	os.Mkdir(filepath.Join(s.Dir(), "20261026-999999.md"), 0o755)
	if list, err := s.List(); err != nil || len(list) != 0 {
		t.Fatalf("list = %v, %v", list, err)
	}
}

func TestTitlesAndPreviews(t *testing.T) {
	tests := []struct {
		text, title, preview string
	}{
		{"Hello", "Hello", ""},
		{"## Heading\nbody line", "Heading", "body line"},
		{"\n\n  - [ ] first task\n- [x] done task\n", "first task", "done task"},
		{"> quoted\r\nmore\r\n", "quoted", "more"},
		{"***", "Untitled note", ""},
		{strings.Repeat("a", 200), strings.Repeat("a", 80) + "…", ""},
		{"t\n" + strings.Repeat("b ", 100), "t", strings.Repeat("b ", 54) + "b…"},
		{"日本語のノート\n二行目", "日本語のノート", "二行目"},
	}
	for _, tt := range tests {
		title, preview := summarise(tt.text)
		if title != tt.title || preview != tt.preview {
			t.Errorf("summarise(%q) = %q, %q; want %q, %q", tt.text, title, preview, tt.title, tt.preview)
		}
	}
}
