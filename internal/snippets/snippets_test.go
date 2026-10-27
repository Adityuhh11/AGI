package snippets

import (
	"errors"
	"testing"
	"time"

	"omni/internal/config"
)

var now = time.Date(2026, 10, 19, 14, 5, 9, 0, time.UTC)

func TestExpandFillsInTheDateTheTimeAndTheClipboard(t *testing.T) {
	reads := 0
	clipboard := func() string { reads++; return "COPIED" }
	for _, tc := range []struct{ text, want string }{
		{"Hello", "Hello"},
		{"{date}", "2026-10-19"},
		{"At {time} on {date}.", "At 14:05 on 2026-10-19."},
		{"> {clipboard}\n> {clipboard}", "> COPIED\n> COPIED"},
		{"{{date}} and {{", "{date}} and {"},
		{"func main() {\n\treturn\n}", "func main() {\n\treturn\n}"},
		{"{unknown} {Date} {clipboard", "{unknown} {Date} {clipboard"},
		{"héllo {date} wörld", "héllo 2026-10-19 wörld"},
		{"", ""},
	} {
		if got := Expand(tc.text, now, clipboard); got != tc.want {
			t.Errorf("Expand(%q) = %q, want %q", tc.text, got, tc.want)
		}
	}
	if reads != 1 {
		t.Errorf("the clipboard was read %d times: once for the one text that asks for it (twice), and for no other", reads)
	}
}

type typer struct {
	pasted []string
	err    error
}

func (t *typer) Paste(text string) error { t.pasted = append(t.pasted, text); return t.err }

func service(list []config.Snippet, clipboard string) (*Service, *typer) {
	ty := &typer{}
	s := New(func() []config.Snippet { return list }, ty, func() string { return clipboard })
	s.now = func() time.Time { return now }
	return s, ty
}

func TestPastingASnippetTypesItsExpandedTextWhateverTheCaseOfTheName(t *testing.T) {
	s, ty := service([]config.Snippet{{Name: "Reply", Text: "Thanks, {clipboard}! ({date})"}}, "Ada")
	if err := s.Paste("reply"); err != nil {
		t.Fatal(err)
	}
	if len(ty.pasted) != 1 || ty.pasted[0] != "Thanks, Ada! (2026-10-19)" {
		t.Fatalf("typed %q", ty.pasted)
	}
}

func TestASnippetThatIsGoneIsAnErrorAndNothingIsTyped(t *testing.T) {
	s, ty := service(nil, "")
	if err := s.Paste("gone"); err == nil || len(ty.pasted) != 0 {
		t.Fatalf("err %v, typed %q", err, ty.pasted)
	}
}

func TestAPasteThatCannotBeTypedReturnsTheReason(t *testing.T) {
	s, ty := service([]config.Snippet{{Name: "A", Text: "x"}}, "")
	ty.err = errors.New("that window runs as administrator")
	if err := s.Paste("A"); !errors.Is(err, ty.err) {
		t.Fatalf("err = %v", err)
	}
}

func TestTheListIsWhatTheSettingsHoldNow(t *testing.T) {
	list := []config.Snippet{{Name: "A", Text: "x"}}
	s, _ := service(list, "")
	if got := s.List(); len(got) != 1 || got[0].Name != "A" {
		t.Fatalf("list %v", got)
	}
}
