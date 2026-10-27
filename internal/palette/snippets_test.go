package palette

import (
	"strings"
	"testing"

	"omni/internal/config"
)

// fakeSnips is the user's snippets, recording what the bar pasted.
type fakeSnips struct {
	list   []config.Snippet
	pasted []string
	err    error
}

func (f *fakeSnips) List() []config.Snippet { return f.list }
func (f *fakeSnips) Paste(name string) error {
	f.pasted = append(f.pasted, name)
	return f.err
}

func snipService(t *testing.T, snips *fakeSnips, names ...string) *Service {
	t.Helper()
	s, _ := newService(t, names...)
	s.snips = snips
	return s
}

var userSnippets = []config.Snippet{
	{Name: "Home address", Keyword: "addr", Text: "12 Example Street\nSpringfield"},
	{Name: "Email signature", Text: "Best regards,\nAda"},
	{Name: "Meeting reply", Keyword: "mtg", Text: "Thanks, that time works for me and the whole team, see you there on the day."},
}

func TestTheSnipPrefixChoosesTheSnippetsMode(t *testing.T) {
	for _, tc := range []struct{ raw, mode, wantMode, wantText string }{
		{"snip: addr", ModeAll, ModeSnip, "addr"},
		{"snippets: addr", ModeAll, ModeSnip, "addr"},
		{"SNIPPET:", ModeAll, ModeSnip, ""},
		{"snip: addr", ModeSnip, ModeSnip, "addr"},
		{"snipe", ModeAll, ModeAll, "snipe"},
	} {
		if mode, text := parse(tc.raw, tc.mode); mode != tc.wantMode || text != tc.wantText {
			t.Errorf("parse(%q, %q) = %q, %q; want %q, %q", tc.raw, tc.mode, mode, text, tc.wantMode, tc.wantText)
		}
	}
}

func TestTheSnippetsModeListsThemAllThenNarrowsByName(t *testing.T) {
	s := snipService(t, &fakeSnips{list: userSnippets})
	all := s.Query("snip:", ModeAll)
	if len(all.Items) != 3 || all.Items[0].Title != "Home address" || all.Items[2].Title != "Meeting reply" {
		t.Fatalf("all: %+v", all.Items)
	}
	first := all.Items[0]
	if first.ID != "snippet:Home address" || first.Kind != "snippet" || first.Subtitle != "12 Example Street" {
		t.Errorf("a row is the name, with the first line of the text beside it: %+v", first)
	}
	if long := all.Items[2].Subtitle; len([]rune(long)) != snippetPreviewLength || !strings.HasSuffix(long, "…") {
		t.Errorf("a long first line is cut to %d characters, got %q", snippetPreviewLength, long)
	}
	if got := s.Query("snip: sig", ModeAll).Items; len(got) != 1 || got[0].Title != "Email signature" {
		t.Errorf("by a word's start: %+v", got)
	}
	if got := s.Query("snip: ha", ModeAll).Items; len(got) != 1 || got[0].Title != "Home address" {
		t.Errorf("by initials: %+v", got)
	}
}

func TestTheSnippetsModeSaysWhyThereIsNothingToList(t *testing.T) {
	none := snipService(t, &fakeSnips{})
	if a := none.Query("snip:", ModeAll); len(a.Items) != 0 || !strings.Contains(a.Note, "Settings") {
		t.Fatalf("no snippets: %+v", a)
	}
	some := snipService(t, &fakeSnips{list: userSnippets})
	if a := some.Query("snip: zzz", ModeAll); len(a.Items) != 0 || !strings.Contains(a.Note, "zzz") {
		t.Fatalf("no match: %+v", a)
	}
}

func TestAKeywordTypedExactlyShowsItsSnippetFirstAndNothingElseDoes(t *testing.T) {
	s := snipService(t, &fakeSnips{list: userSnippets}, "Addressbook")
	got := s.Query("ADDR", ModeAll)
	if len(got.Items) == 0 || got.Items[0].ID != "snippet:Home address" {
		t.Fatalf("items %v: the snippet must come before the app", kinds(got))
	}
	for _, typed := range []string{"add", "address", "home", "signature", "mtg now"} {
		if k := kinds(s.Query(typed, ModeAll)); strings.Contains(k, "snippet") {
			t.Errorf("typing %q showed a snippet (%s): only an exact keyword does", typed, k)
		}
	}
	if k := kinds(s.Query("addr", ModeApps)); strings.Contains(k, "snippet") {
		t.Errorf("the Apps mode showed a snippet: %s", k)
	}
}

func TestTheBuiltInCommandRowsAreFoundByTheirNamesInAllMode(t *testing.T) {
	s := snipService(t, &fakeSnips{})
	titlesOf := func(a Answer) string {
		var out []string
		for _, item := range a.Items {
			if item.Kind == "cmd" {
				out = append(out, item.Title)
			}
		}
		return strings.Join(out, "|")
	}
	for typed, want := range map[string]string{
		"clip": "Clipboard history", "clipb": "Clipboard history", "snip": "Snippets", "sn": "Snippets", "ch": "Clipboard history",
		"c": "", "zzz": "",
	} {
		if got := titlesOf(s.Query(typed, ModeAll)); got != want {
			t.Errorf("typing %q gave command rows %q, want %q", typed, got, want)
		}
	}
	if got := titlesOf(s.Query("clip", ModeApps)); got != "" {
		t.Errorf("the Apps mode listed commands: %q", got)
	}
	row := s.Query("clip", ModeAll).Items[0]
	if row.ID != "cmd:clipboard" || row.Fill != "clip: " {
		t.Errorf("choosing the row fills the bar with the mode's prefix: %+v", row)
	}
}

func TestSnippetsAndCommandsCanBePinnedAndAQuicklinkPinFillsItsTrigger(t *testing.T) {
	s := snipService(t, &fakeSnips{list: userSnippets})
	for _, id := range []string{"cmd:clipboard", "snippet:Home address", "quicklink:gh:", "cmd:snippets", "snippet:Gone"} {
		if on, err := s.TogglePin(id); err != nil || !on {
			t.Fatalf("pin %s = %v, %v", id, on, err)
		}
	}
	var got []string
	for _, item := range s.Pinned() {
		got = append(got, item.ID+"|"+item.Fill)
	}
	want := "cmd:clipboard|clip: , snippet:Home address|, quicklink:gh:|gh , cmd:snippets|snip: "
	if strings.Join(got, ", ") != want {
		t.Fatalf("pinned rows %q, want %q (a snippet that is gone is hidden, not unpinned)", strings.Join(got, ", "), want)
	}
	if len(s.PinIDs()) != 5 {
		t.Errorf("ids %v", s.PinIDs())
	}
	if _, err := s.TogglePin("cmd:nonsense"); err == nil {
		t.Error("an unknown command cannot be pinned")
	}
}

func TestEnterOnASnippetPastesItAndNothingElseIsAllowed(t *testing.T) {
	snips := &fakeSnips{list: userSnippets}
	s := snipService(t, snips)
	if err := s.Do("snippet:Home address", "open", ""); err != nil {
		t.Fatal(err)
	}
	if len(snips.pasted) != 1 || snips.pasted[0] != "Home address" {
		t.Fatalf("pasted %v", snips.pasted)
	}
	if err := s.Do("snippet:Home address", "reveal", ""); err == nil {
		t.Error("a snippet can only be pasted")
	}
}
