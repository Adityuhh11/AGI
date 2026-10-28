package chat

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func newStore(t *testing.T) *Store {
	t.Helper()
	s, err := Open(filepath.Join(t.TempDir(), "chats"))
	if err != nil {
		t.Fatal(err)
	}
	return s
}

func conv(question, answer string) *Conversation {
	return &Conversation{Messages: []Message{{Role: "user", Text: question}, {Role: "assistant", Text: answer}}}
}

func TestSaveGivesANewConversationAnIdAndATitleAndGetReadsItBack(t *testing.T) {
	s := newStore(t)
	c := conv("How do I undo a commit?\nI mean the last one.", "git reset --soft HEAD~1")
	if err := s.Save(c); err != nil {
		t.Fatal(err)
	}
	if !idPattern.MatchString(c.ID) || c.Title != "How do I undo a commit?" || c.Created == 0 || c.Modified == 0 {
		t.Fatalf("saved conversation = %+v", c)
	}
	got, err := s.Get(c.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Messages) != 2 || got.Messages[1].Text != "git reset --soft HEAD~1" || got.Title != c.Title {
		t.Fatalf("read back = %+v", got)
	}
	entries, _ := os.ReadDir(s.Dir())
	for _, e := range entries {
		if strings.HasSuffix(e.Name(), ".tmp") {
			t.Fatalf("a temporary file %s was left behind", e.Name())
		}
	}
}

func TestTwoConversationsInTheSameSecondGetDifferentIds(t *testing.T) {
	s := newStore(t)
	s.now = func() time.Time { return time.Date(2026, 10, 28, 12, 0, 0, 0, time.UTC) }
	a, b := conv("one", "x"), conv("two", "y")
	if err := s.Save(a); err != nil {
		t.Fatal(err)
	}
	if err := s.Save(b); err != nil {
		t.Fatal(err)
	}
	if a.ID == b.ID || b.ID != "20261028-120000-2" {
		t.Fatalf("ids = %q and %q", a.ID, b.ID)
	}
}

func TestAnIdThatNamesAFileOutsideTheFolderIsRefused(t *testing.T) {
	s := newStore(t)
	for _, id := range []string{"../config", `..\config`, "20261028-120000/../x", "", "notanid", "20261028-120000.json"} {
		if _, err := s.Get(id); err == nil {
			t.Errorf("Get(%q) was accepted", id)
		}
		if err := s.Delete(id); err == nil {
			t.Errorf("Delete(%q) was accepted", id)
		}
		if err := s.Save(&Conversation{ID: id, Messages: []Message{{Role: "user", Text: "x"}}}); err == nil && id != "" { // an empty id means a new one
			t.Errorf("Save with id %q was accepted", id)
		}
	}
}

func TestSearchFindsEveryWordInTitleOrTextNewestFirst(t *testing.T) {
	s := newStore(t)
	clock := time.Date(2026, 10, 1, 9, 0, 0, 0, time.UTC)
	s.now = func() time.Time { clock = clock.Add(time.Minute); return clock }
	for _, c := range []*Conversation{conv("regex for phone numbers", "use \\d{3}"), conv("a poem", "roses are red"), conv("git help", "reset the Regex? no")} {
		if err := s.Save(c); err != nil {
			t.Fatal(err)
		}
	}
	all, _ := s.Search("", 0)
	if len(all) != 3 || all[0].Title != "git help" {
		t.Fatalf("all = %+v, want three, newest first", all)
	}
	found, _ := s.Search("REGEX", 0)
	if len(found) != 2 || found[0].Title != "git help" || found[1].Title != "regex for phone numbers" {
		t.Fatalf("search regex = %+v", found)
	}
	if found, _ := s.Search("regex phone", 0); len(found) != 1 {
		t.Fatalf("both words must be present: %+v", found)
	}
	if limited, _ := s.Search("", 2); len(limited) != 2 {
		t.Fatalf("limit ignored: %+v", limited)
	}
}

func TestDeleteClearAndExpire(t *testing.T) {
	s := newStore(t)
	old, fresh := conv("old", "x"), conv("fresh", "y")
	s.now = func() time.Time { return time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC) }
	if err := s.Save(old); err != nil {
		t.Fatal(err)
	}
	s.now = func() time.Time { return time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC) }
	if err := s.Save(fresh); err != nil {
		t.Fatal(err)
	}
	// Expire goes by the file's own time, so give the files the times the clock above stood for.
	for c, at := range map[*Conversation]time.Time{old: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC), fresh: time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC)} {
		if err := os.Chtimes(filepath.Join(s.Dir(), c.ID+extension), at, at); err != nil {
			t.Fatal(err)
		}
	}
	if err := s.Expire(0); err != nil {
		t.Fatal(err)
	}
	if list, _ := s.Search("", 0); len(list) != 2 {
		t.Fatalf("0 days must keep everything: %+v", list)
	}
	if err := s.Expire(90); err != nil {
		t.Fatal(err)
	}
	if list, _ := s.Search("", 0); len(list) != 1 || list[0].Title != "fresh" {
		t.Fatalf("after Expire(90) = %+v, want only the fresh one", list)
	}
	if err := s.Delete(fresh.ID); err != nil || s.Delete(fresh.ID) != nil {
		t.Fatalf("deleting twice must not fail: %v", err)
	}
	for _, q := range []string{"a", "b", "c"} {
		if err := s.Save(conv(q, q)); err != nil {
			t.Fatal(err)
		}
	}
	if err := s.Clear(); err != nil {
		t.Fatal(err)
	}
	if list, _ := s.Search("", 0); len(list) != 0 {
		t.Fatalf("Clear left %+v", list)
	}
}

func TestTitleOfCutsLongFirstLinesAndSkipsEmptyOnes(t *testing.T) {
	long := strings.Repeat("é", 100)
	for name, tc := range map[string]struct {
		messages []Message
		want     string
	}{
		"no messages": {nil, untitled},
		"blank":       {[]Message{{Role: "user", Text: "  \n "}}, untitled},
		"assistant":   {[]Message{{Role: "assistant", Text: "hi"}, {Role: "user", Text: "second"}}, "second"},
		"long, runes": {[]Message{{Role: "user", Text: long}}, strings.Repeat("é", titleLength-1) + "…"},
		"first line":  {[]Message{{Role: "user", Text: "line one\nline two"}}, "line one"},
	} {
		if got := TitleOf(tc.messages); got != tc.want {
			t.Errorf("%s: TitleOf = %q, want %q", name, got, tc.want)
		}
	}
}

func TestAFileThatIsNotAConversationIsSkippedByTheList(t *testing.T) {
	s := newStore(t)
	if err := os.WriteFile(filepath.Join(s.Dir(), "20261028-120000.json"), []byte("{not json"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(s.Dir(), "readme.txt"), []byte("hi"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := s.Save(conv("real", "one")); err != nil {
		t.Fatal(err)
	}
	if list, err := s.Search("", 0); err != nil || len(list) != 1 {
		t.Fatalf("list = %+v, %v", list, err)
	}
}
