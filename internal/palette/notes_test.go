package palette

import (
	"errors"
	"strings"
	"testing"
	"time"

	"omni/internal/notes"
)

type fakeNotes struct {
	list   []notes.Note
	saved  []string
	err    error
	lastQ  string
	nextID string
}

func (f *fakeNotes) Search(words string, limit int) ([]notes.Note, error) {
	f.lastQ = words
	return f.list, f.err
}

func (f *fakeNotes) Save(id, text string) (notes.Note, error) {
	f.saved = append(f.saved, id+"|"+text)
	return notes.Note{ID: f.nextID, Text: text}, f.err
}

func notesService(t *testing.T) (*Service, *fakeNotes, *[]string, *int) {
	t.Helper()
	s, dismissed := newService(t)
	s.now = func() time.Time { return time.Date(2026, 10, 26, 12, 0, 0, 0, time.UTC) }
	book := &fakeNotes{nextID: "20261026-120000", list: []notes.Note{
		{ID: "20261026-090000", Title: "Standup", Modified: time.Date(2026, 10, 26, 9, 5, 0, 0, time.UTC).UnixMilli()},
		{ID: "20261020-090000", Title: "Trip", Modified: time.Date(2026, 10, 20, 9, 5, 0, 0, time.UTC).UnixMilli()},
	}}
	var opened []string
	With(s, Extras{Notes: book, OpenNote: func(id string) { opened = append(opened, id) }})
	return s, book, &opened, dismissed
}

func TestNotePrefixesChooseTheNotesModeButOrdinaryTextWithAColonDoesNot(t *testing.T) {
	tests := []struct {
		raw, mode, text string
	}{
		{"note: standup", ModeNotes, "standup"},
		{"note:", ModeNotes, ""},
		{"NOTE:x", ModeNotes, "x"},
		{"notes: meeting", ModeAll, "notes: meeting"}, // the file search's Documents scope keeps working
		{"note to self: milk", ModeAll, "note to self: milk"},
		{"notebook: x", ModeAll, "notebook: x"},
	}
	for _, tt := range tests {
		if mode, text := parse(tt.raw, ""); mode != tt.mode || text != tt.text {
			t.Errorf("parse(%q) = %q, %q; want %q, %q", tt.raw, mode, text, tt.mode, tt.text)
		}
	}
}

func TestTheNotesModeOffersANewNoteFirstThenTheMatchingNotes(t *testing.T) {
	s, book, _, _ := notesService(t)
	answer := s.Query("note: stand", ModeAll)
	if answer.Mode != ModeNotes || book.lastQ != "stand" {
		t.Fatalf("mode %q, searched %q", answer.Mode, book.lastQ)
	}
	var got []string
	for _, item := range answer.Items {
		got = append(got, item.ID+"|"+item.Title+"|"+item.Subtitle)
	}
	want := []string{
		"note:new:stand|New note “stand”|Notes",
		"note:open:20261026-090000|Standup|09:05",
		"note:open:20261020-090000|Trip|20 Oct",
	}
	if strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Fatalf("rows:\n%s\nwant:\n%s", strings.Join(got, "\n"), strings.Join(want, "\n"))
	}
	if first := s.Query("note:", ModeAll).Items[0]; first.Title != "New note" || first.ID != "note:new:" {
		t.Errorf("first row with nothing typed = %+v", first)
	}
}

func TestChoosingANewNoteSavesTheTextAndOpensIt(t *testing.T) {
	s, book, opened, dismissed := notesService(t)
	if err := s.Do("note:new:buy milk", "open", ""); err != nil {
		t.Fatal(err)
	}
	if len(book.saved) != 1 || book.saved[0] != "|buy milk\n" || len(*opened) != 1 || (*opened)[0] != "20261026-120000" || *dismissed != 1 {
		t.Errorf("saved %v, opened %v, dismissed %d", book.saved, *opened, *dismissed)
	}
}

func TestANewNoteWithNoTextOpensAnEmptyEditorAndSavesNothing(t *testing.T) {
	s, book, opened, _ := notesService(t)
	if err := s.Do("note:new:  ", "open", ""); err != nil {
		t.Fatal(err)
	}
	if len(book.saved) != 0 || len(*opened) != 1 || (*opened)[0] != "new" {
		t.Errorf("saved %v, opened %v", book.saved, *opened)
	}
}

func TestChoosingANoteOpensIt(t *testing.T) {
	s, _, opened, _ := notesService(t)
	if err := s.Do("note:open:20261020-090000", "open", ""); err != nil || (*opened)[0] != "20261020-090000" {
		t.Errorf("opened %v, err %v", *opened, err)
	}
	if err := s.Do("note:open:x", "reveal", ""); err == nil {
		t.Error("a note row can only be opened")
	}
	if err := s.Do("note:wat", "open", ""); err == nil {
		t.Error("an unknown note id was run")
	}
}

func TestNotesThatCannotBeReadOrSavedSayWhy(t *testing.T) {
	s, book, _, _ := notesService(t)
	book.err = errors.New("disk full")
	if answer := s.Query("note: x", ModeAll); !strings.Contains(answer.Note, "disk full") || len(answer.Items) != 1 {
		t.Errorf("answer = %+v", answer)
	}
	if err := s.Do("note:new:x", "open", ""); err == nil || !strings.Contains(err.Error(), "disk full") {
		t.Errorf("err = %v", err)
	}
	bare, _ := newService(t)
	if answer := bare.Query("note:", ModeAll); answer.Note == "" {
		t.Error("a bar without notes should say so")
	}
}

func TestTheNotesCommandIsFoundByName(t *testing.T) {
	s, _, _, _ := notesService(t)
	found := false
	for _, item := range s.Query("note", ModeAll).Items {
		found = found || (item.ID == "cmd:notes" && item.Fill == "note: ")
	}
	if !found {
		t.Errorf("typing note did not offer the Notes command: %+v", s.Query("note", ModeAll).Items)
	}
}
