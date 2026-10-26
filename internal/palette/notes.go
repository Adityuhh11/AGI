package palette

import (
	"fmt"
	"strings"
	"time"

	"agi/internal/notes"
)

// NoteBook is the notes as the bar uses them; *notes.Store satisfies it.
type NoteBook interface {
	Search(words string, limit int) ([]notes.Note, error)
	Save(id, text string) (notes.Note, error)
}

// noteRows is how many notes the Notes mode lists.
const noteRows = 12

// noteItems answers the Notes mode: first a row that makes a note (with the text typed as its first line), then the notes
// that contain every word typed, newest first.
func (s *Service) noteItems(text string) ([]Item, string) {
	if s.x.Notes == nil || s.x.OpenNote == nil {
		return nil, "Notes are not available."
	}
	title := "New note"
	if text != "" {
		title = "New note “" + text + "”"
	}
	items := []Item{{ID: "note:new:" + text, Kind: "note", Title: title, Subtitle: "Notes"}}
	found, err := s.x.Notes.Search(text, noteRows)
	if err != nil {
		return items, "The notes could not be read: " + err.Error()
	}
	for _, n := range found {
		items = append(items, Item{ID: "note:open:" + n.ID, Kind: "note", Title: n.Title, Subtitle: s.noteDate(n.Modified)})
	}
	return items, ""
}

// noteDate is the right-hand label of a note: the time today, else the day and month.
func (s *Service) noteDate(ms int64) string {
	then, now := time.UnixMilli(ms).In(s.now().Location()), s.now()
	if then.Year() == now.Year() && then.YearDay() == now.YearDay() {
		return then.Format("15:04")
	}
	return then.Format("2 Jan")
}

// doNote makes a note or opens one in the notes window, and closes the bar.
func (s *Service) doNote(rest, action string) error {
	if action != "open" {
		return fmt.Errorf("a note row can only be opened, not %q", action)
	}
	if s.x.Notes == nil || s.x.OpenNote == nil {
		return fmt.Errorf("notes are not available")
	}
	switch {
	case strings.HasPrefix(rest, "new:"):
		text := strings.TrimPrefix(rest, "new:")
		id := "new" // the window opens on an empty note; nothing is saved until something is typed
		if strings.TrimSpace(text) != "" {
			n, err := s.x.Notes.Save("", text+"\n")
			if err != nil {
				return fmt.Errorf("make the note: %w", err)
			}
			id = n.ID
		}
		s.dismiss() // the bar goes first: the notes window is built next, and the bar should not wait for that
		s.x.OpenNote(id)
	case strings.HasPrefix(rest, "open:"):
		s.dismiss()
		s.x.OpenNote(strings.TrimPrefix(rest, "open:"))
	default:
		return fmt.Errorf("do not know how to run note:%s", rest)
	}
	return nil
}
