package palette

import (
	"errors"
	"strings"
	"testing"
	"time"

	"omni/internal/chat"
)

type fakeAssistant struct {
	info   chat.Info
	list   []chat.Summary
	err    error
	lastQ  string
	warmed int
}

func (f *fakeAssistant) Info() chat.Info { return f.info }
func (f *fakeAssistant) List(q string) ([]chat.Summary, error) {
	f.lastQ = q
	return f.list, f.err
}
func (f *fakeAssistant) Warm() { f.warmed++ }

func askService(t *testing.T) (*Service, *fakeAssistant, *[]string, *int) {
	t.Helper()
	s, dismissed := newService(t)
	s.now = func() time.Time { return time.Date(2026, 10, 28, 12, 0, 0, 0, time.UTC) }
	a := &fakeAssistant{info: chat.Info{Enabled: true, Installed: true, State: "unloaded"}, list: []chat.Summary{
		{ID: "20261028-090000", Title: "git reset", Modified: time.Date(2026, 10, 28, 9, 5, 0, 0, time.UTC).UnixMilli()},
		{ID: "20261020-090000", Title: "a regex", Modified: time.Date(2026, 10, 20, 9, 5, 0, 0, time.UTC).UnixMilli()},
	}}
	var did []string
	With(s, Extras{
		Assistant: a,
		AskChat:   func(text string) { did = append(did, "ask:"+text) },
		OpenChat:  func(id string) { did = append(did, "open:"+id) },
	})
	return s, a, &did, dismissed
}

func TestAskPrefixesChooseTheAskModeButOrdinaryTextDoesNot(t *testing.T) {
	for _, tt := range []struct{ raw, mode, text string }{
		{"ask: why is the sky blue", ModeAsk, "why is the sky blue"},
		{"ai: hi", ModeAsk, "hi"},
		{"Chat:", ModeAsk, ""},
		{"ASK :x", ModeAsk, "x"},
		{"ask me anything", ModeAll, "ask me anything"},
		{"asking: x", ModeAll, "asking: x"},
		{"chatgpt: x", ModeAll, "chatgpt: x"},
	} {
		if mode, text := parse(tt.raw, ""); mode != tt.mode || text != tt.text {
			t.Errorf("parse(%q) = %q, %q; want %q, %q", tt.raw, mode, text, tt.mode, tt.text)
		}
	}
	if mode, text := parse("ask: stay", ModeAsk); mode != ModeAsk || text != "stay" {
		t.Errorf("with the chip already on Ask, parse = %q, %q", mode, text)
	}
}

func TestTheAskModeOffersTheQuestionFirstThenMatchingConversationsAndWarmsTheModel(t *testing.T) {
	s, a, _, _ := askService(t)
	answer := s.Query("ask: undo", ModeAll)
	if answer.Mode != ModeAsk || a.lastQ != "undo" || a.warmed != 1 {
		t.Fatalf("mode %q, searched %q, warmed %d", answer.Mode, a.lastQ, a.warmed)
	}
	var got []string
	for _, item := range answer.Items {
		got = append(got, item.ID+"|"+item.Title+"|"+item.Subtitle)
	}
	want := []string{"ask:new:undo|Ask “undo”|Omni AI", "ask:open:20261028-090000|git reset|09:05", "ask:open:20261020-090000|a regex|20 Oct"}
	if strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Fatalf("rows:\n%s\nwant:\n%s", strings.Join(got, "\n"), strings.Join(want, "\n"))
	}
	if first := s.Query("ask:", ModeAll).Items[0]; first.Title != "New chat" || first.ID != "ask:new:" {
		t.Errorf("first row with nothing typed = %+v", first)
	}
	a.info.State = "ready"
	s.Query("ask: again", ModeAll)
	if a.warmed != 2 { // the first of the two queries above warmed once; a ready model is not warmed again
		t.Errorf("warmed %d times; a ready model must not be warmed", a.warmed)
	}
}

func TestWithNothingTypedOnlyTheFiveNewestConversationsShow(t *testing.T) {
	s, a, _, _ := askService(t)
	a.list = nil
	for i := range 9 {
		a.list = append(a.list, chat.Summary{ID: "2026102" + string(rune('0'+i)) + "-090000", Title: "c"})
	}
	if n := len(s.Query("ask:", ModeAll).Items); n != 1+askRecent {
		t.Fatalf("rows = %d, want %d", n, 1+askRecent)
	}
	if n := len(s.Query("ask: c", ModeAll).Items); n != 1+9 {
		t.Fatalf("rows with words = %d, want 10", n)
	}
}

func TestWithOmniAIOffTheAskModeOnlyLeadsToSettingsAndWarmsNothing(t *testing.T) {
	for name, info := range map[string]chat.Info{
		"off":           {Enabled: false, Installed: true},
		"not installed": {Enabled: true, Installed: false},
	} {
		s, a, did, dismissed := askService(t)
		a.info = info
		answer := s.Query("ask: hello", ModeAll)
		if len(answer.Items) != 1 || answer.Items[0].ID != "cmd:settings:omni-ai" || a.warmed != 0 {
			t.Fatalf("%s: rows %+v, warmed %d", name, answer.Items, a.warmed)
		}
		var section string
		s.openSettings = func(sec string) { section = sec }
		if err := s.Do("cmd:settings:omni-ai", "open", "ask: hello"); err != nil || section != "omni-ai" || *dismissed != 1 || len(*did) != 0 {
			t.Fatalf("%s: Do = %v, section %q, dismissed %d, did %v", name, err, section, *dismissed, *did)
		}
	}
}

func TestChoosingAnAskRowClosesTheBarThenOpensTheChat(t *testing.T) {
	s, _, did, dismissed := askService(t)
	for _, id := range []string{"ask:new:why is the sky blue", "ask:open:20261028-090000", "ask:new:"} {
		if err := s.Do(id, "open", "ask: x"); err != nil {
			t.Fatal(err)
		}
	}
	want := []string{"ask:why is the sky blue", "open:20261028-090000", "ask:"}
	if strings.Join(*did, ",") != strings.Join(want, ",") || *dismissed != 3 {
		t.Fatalf("did %v dismissed %d", *did, *dismissed)
	}
	if err := s.Do("ask:new:x", "reveal", ""); err == nil {
		t.Error("an ask row accepted an action it does not have")
	}
	if err := s.Do("ask:bogus", "open", ""); err == nil {
		t.Error("an unknown ask row was accepted")
	}
}

func TestAnUnreadableHistoryStillOffersTheQuestion(t *testing.T) {
	s, a, _, _ := askService(t)
	a.err = errors.New("disk")
	answer := s.Query("ask: hi", ModeAll)
	if len(answer.Items) != 1 || !strings.Contains(answer.Note, "disk") {
		t.Fatalf("answer = %+v", answer)
	}
}

func TestTypingAskFindsTheCommandAndItFillsTheBarWithTheMode(t *testing.T) {
	s, _, _, _ := askService(t)
	for _, typed := range []string{"ask", "chat", "ai"} {
		var row *Item
		for _, item := range s.Query(typed, ModeAll).Items {
			if item.ID == "cmd:ask" {
				row = &item
			}
		}
		if row == nil || row.Fill != "ask: " {
			t.Errorf("typing %q: command row = %+v", typed, row)
		}
	}
}
