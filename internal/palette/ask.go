package palette

import (
	"fmt"
	"strings"
	"time"

	"omni/internal/chat"
)

// Assistant is Omni AI as the bar uses it; *chat.Service satisfies it.
type Assistant interface {
	Info() chat.Info
	List(query string) ([]chat.Summary, error)
	// Warm starts loading the model in the background.
	Warm()
}

const (
	// askRecent is how many past conversations the Ask mode lists with nothing typed, askFound with words typed.
	askRecent = 5
	askFound  = 12
	// turnOnID is the row that sends a person who has not turned Omni AI on to the Settings page that does.
	turnOnID = "cmd:settings:omni-ai"
)

// askItems answers the Ask mode: a row that asks the typed question (or starts a new chat), then the conversations that
// contain the typed words, newest first. With Omni AI off, one row leads to Settings; the bar never starts a download.
func (s *Service) askItems(text string) ([]Item, string) {
	if s.x.Assistant == nil || s.x.AskChat == nil {
		return nil, "Omni AI is not available."
	}
	info := s.x.Assistant.Info()
	if !info.Enabled || !info.Installed {
		return []Item{{ID: turnOnID, Kind: "cmd", Title: "Turn on Omni AI…", Subtitle: "Settings"}}, ""
	}
	if info.State == "unloaded" || info.State == "failed" {
		s.x.Assistant.Warm() // by the time the question is typed and sent, the model is often ready
	}
	title := "New chat"
	if text != "" {
		title = "Ask “" + text + "”"
	}
	items := []Item{{ID: "ask:new:" + text, Kind: "ask", Title: title, Subtitle: "Omni AI"}}
	found, err := s.x.Assistant.List(text)
	if err != nil {
		return items, "The conversations could not be read: " + err.Error()
	}
	limit := askFound
	if text == "" {
		limit = askRecent
	}
	for _, c := range found[:min(len(found), limit)] {
		items = append(items, Item{ID: "ask:open:" + c.ID, Kind: "ask", Title: c.Title, Subtitle: s.askDate(c.Modified)})
	}
	return items, ""
}

// askDate is the right-hand label of a conversation: the time today, else the day and month.
func (s *Service) askDate(ms int64) string {
	then, now := time.UnixMilli(ms).In(s.now().Location()), s.now()
	if then.Year() == now.Year() && then.YearDay() == now.YearDay() {
		return then.Format("15:04")
	}
	return then.Format("2 Jan")
}

// doAsk asks a question in the chat window or reopens a conversation, and closes the bar.
func (s *Service) doAsk(rest, action string) error {
	if action != "open" {
		return fmt.Errorf("an ask row can only be opened, not %q", action)
	}
	if s.x.AskChat == nil || s.x.OpenChat == nil {
		return fmt.Errorf("Omni AI is not available")
	}
	switch {
	case strings.HasPrefix(rest, "new:"):
		s.dismiss() // the bar goes first: the window is built next, and the bar should not wait for that
		s.x.AskChat(strings.TrimPrefix(rest, "new:"))
	case strings.HasPrefix(rest, "open:"):
		s.dismiss()
		s.x.OpenChat(strings.TrimPrefix(rest, "open:"))
	default:
		return fmt.Errorf("do not know how to run ask:%s", rest)
	}
	return nil
}
