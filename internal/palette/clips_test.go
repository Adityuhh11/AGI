package palette

import (
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"omni/internal/clipboard"
)

// fakeClips is a clipboard history that records what the bar did with it.
type fakeClips struct {
	enabled bool
	entries []clipboard.Entry // newest first
	pasted  []int64
	copied  []int64
	deleted []int64
	err     error
}

func (f *fakeClips) Enabled() bool { return f.enabled }
func (f *fakeClips) Search(words string, limit int) []clipboard.Entry {
	var out []clipboard.Entry
	for _, e := range f.entries {
		match := true
		for _, w := range strings.Fields(strings.ToLower(words)) {
			match = match && strings.Contains(strings.ToLower(e.Text), w)
		}
		if match && len(out) < limit {
			out = append(out, e)
		}
	}
	return out
}
func (f *fakeClips) Paste(id int64) error  { f.pasted = append(f.pasted, id); return f.err }
func (f *fakeClips) Copy(id int64) error   { f.copied = append(f.copied, id); return f.err }
func (f *fakeClips) Delete(id int64) error { f.deleted = append(f.deleted, id); return f.err }

func clipService(t *testing.T, clips *fakeClips) *Service {
	t.Helper()
	s, _ := newService(t)
	s.clips = clips
	s.now = func() time.Time { return t0.Add(10 * time.Minute) }
	return s
}

var t0 = time.Date(2026, 10, 19, 12, 0, 0, 0, time.UTC)

func TestTheClipPrefixAndChipChooseTheClipboardMode(t *testing.T) {
	for _, tc := range []struct{ raw, mode, wantMode, wantText string }{
		{"clip: invoice", ModeAll, ModeClip, "invoice"},
		{"clipboard: invoice", ModeAll, ModeClip, "invoice"},
		{"CLIP:", ModeAll, ModeClip, ""},
		{"invoice", ModeClip, ModeClip, "invoice"},
		{"clip: invoice", ModeClip, ModeClip, "invoice"},
		{"clippy", ModeAll, ModeAll, "clippy"},
	} {
		if mode, text := parse(tc.raw, tc.mode); mode != tc.wantMode || text != tc.wantText {
			t.Errorf("parse(%q, %q) = %q, %q; want %q, %q", tc.raw, tc.mode, mode, text, tc.wantMode, tc.wantText)
		}
	}
}

func TestTheClipboardModeSaysWhyThereIsNothingToList(t *testing.T) {
	off := clipService(t, &fakeClips{})
	if a := off.Query("", ModeClip); len(a.Items) != 0 || !strings.Contains(a.Note, "off") {
		t.Fatalf("history off: %+v", a)
	}
	on := clipService(t, &fakeClips{enabled: true})
	if a := on.Query("", ModeClip); len(a.Items) != 0 || !strings.Contains(a.Note, "Nothing has been copied") {
		t.Fatalf("nothing copied: %+v", a)
	}
	if a := on.Query("clip: zebra", ModeAll); a.Mode != ModeClip || !strings.Contains(a.Note, "zebra") {
		t.Fatalf("no match: %+v", a)
	}
}

func TestCopiedTextsAreRowsWithTheirSourceAndAge(t *testing.T) {
	long := strings.Repeat("word ", 40)
	s := clipService(t, &fakeClips{enabled: true, entries: []clipboard.Entry{
		{ID: 3, Text: "  SELECT *\n\tFROM  orders  ", Source: "Code", At: t0.Add(7 * time.Minute)},
		{ID: 2, Text: long, Source: "chrome", At: t0.Add(-3 * time.Hour)},
		{ID: 1, Text: "plain", At: t0.Add(-50 * time.Hour)},
	}})
	a := s.Query("clip:", ModeAll)
	if len(a.Items) != 3 || a.Note != "" {
		t.Fatalf("answer %+v", a)
	}
	first, second, third := a.Items[0], a.Items[1], a.Items[2]
	if first.ID != "clip:3" || first.Kind != "clip" || first.Title != "SELECT *" {
		t.Errorf("first row %+v: the title is the first line with its spaces tidied", first)
	}
	if first.Subtitle != "Code · 3 min ago · 26 characters" {
		t.Errorf("first subtitle %q (a text of several lines says how long it is)", first.Subtitle)
	}
	if n := len([]rune(second.Title)); n != clipTitleLength || !strings.HasSuffix(second.Title, "…") {
		t.Errorf("a long text is cut to %d characters with an ellipsis, got %d: %q", clipTitleLength, n, second.Title)
	}
	if !strings.HasPrefix(second.Subtitle, "chrome · 3 h ago · 200 characters") {
		t.Errorf("second subtitle %q", second.Subtitle)
	}
	if third.Subtitle != "2 d ago" {
		t.Errorf("a text from an unknown program says only its age, got %q", third.Subtitle)
	}
	if got := strings.Join(first.Actions, ","); got != "copy,delete" {
		t.Errorf("actions %q", got)
	}
}

func TestTypingNarrowsTheClipboardListByWords(t *testing.T) {
	s := clipService(t, &fakeClips{enabled: true, entries: []clipboard.Entry{
		{ID: 2, Text: "kubernetes pod yaml"}, {ID: 1, Text: "invoice march"},
	}})
	a := s.Query("clip: POD yaml", ModeAll)
	if len(a.Items) != 1 || a.Items[0].ID != "clip:2" {
		t.Fatalf("items %+v", a.Items)
	}
}

func TestEnterPastesCopyCopiesDeleteForgets(t *testing.T) {
	clips := &fakeClips{enabled: true}
	s := clipService(t, clips)
	for action, record := range map[string]func() []int64{
		"open":   func() []int64 { return clips.pasted },
		"copy":   func() []int64 { return clips.copied },
		"delete": func() []int64 { return clips.deleted },
	} {
		if err := s.Do("clip:42", action, ""); err != nil {
			t.Fatalf("%s: %v", action, err)
		}
		if got := record(); len(got) != 1 || got[0] != 42 {
			t.Errorf("%s reached %v", action, got)
		}
	}
	if err := s.Do("clip:42", "reveal", ""); err == nil {
		t.Error("a copied text cannot be revealed")
	}
	if err := s.Do("clip:abc", "open", ""); err == nil {
		t.Error("an id that is not a number must be refused")
	}
}

func TestAPasteThatCannotBeTypedComesBackAsTheReason(t *testing.T) {
	s := clipService(t, &fakeClips{enabled: true, err: clipboard.ErrBlocked})
	if err := s.Do("clip:1", "open", ""); !errors.Is(err, clipboard.ErrBlocked) {
		t.Fatalf("err = %v: the page shows it above the list and the bar stays open", err)
	}
}

func TestHowLongAgoIsInTheLargestUnitThatFits(t *testing.T) {
	for d, want := range map[time.Duration]string{
		10 * time.Second: "just now", time.Minute: "1 min ago", 59 * time.Minute: "59 min ago",
		time.Hour: "1 h ago", 23 * time.Hour: "23 h ago", 24 * time.Hour: "1 d ago", 100 * time.Hour: fmt.Sprintf("%d d ago", 4),
	} {
		if got := ago(d); got != want {
			t.Errorf("ago(%v) = %q, want %q", d, got, want)
		}
	}
}
