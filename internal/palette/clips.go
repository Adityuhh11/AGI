package palette

import (
	"fmt"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"agi/internal/clipboard"
)

// clipItems answers the Clipboard mode: what was copied, newest first, narrowed by the words typed. When
// there is nothing to list the second result says why.
func (s *Service) clipItems(text string) ([]Item, string) {
	if !s.clips.Enabled() {
		return nil, "Clipboard history is off. Turn it on in Settings, under Clipboard."
	}
	entries := s.clips.Search(text, clipRows)
	if len(entries) == 0 {
		if text == "" {
			return nil, "Nothing has been copied since the history was switched on."
		}
		return nil, "Nothing copied matches “" + text + "”."
	}
	now := s.now()
	items := make([]Item, len(entries))
	for i, e := range entries {
		items[i] = Item{
			ID:       "clip:" + strconv.FormatInt(e.ID, 10),
			Kind:     "clip",
			Title:    clipTitle(e.Text),
			Subtitle: clipMeta(e, now),
			Actions:  []string{"copy", "delete"},
		}
	}
	return items, ""
}

// doClip carries out an action on a copied text: "open" pastes it, "copy" only copies it, "delete" forgets it.
func (s *Service) doClip(rawID, action string) error {
	id, err := strconv.ParseInt(rawID, 10, 64)
	if err != nil {
		return fmt.Errorf("do not know how to run %q", "clip:"+rawID)
	}
	switch action {
	case "open":
		return s.clips.Paste(id)
	case "copy":
		return s.clips.Copy(id)
	case "delete":
		return s.clips.Delete(id)
	}
	return fmt.Errorf("unknown action %q", action)
}

// clipTitle is the first line of a copied text with its spaces tidied, cut to fit a row.
func clipTitle(text string) string {
	for _, line := range strings.Split(text, "\n") {
		if line = strings.Join(strings.Fields(line), " "); line != "" {
			if utf8.RuneCountInString(line) > clipTitleLength {
				return string([]rune(line)[:clipTitleLength-1]) + "…"
			}
			return line
		}
	}
	return ""
}

// clipMeta says where a copy came from and how long ago ("Code · 3 min ago"), and how long it is when the row
// shows only the beginning of it.
func clipMeta(e clipboard.Entry, now time.Time) string {
	var parts []string
	if e.Source != "" {
		parts = append(parts, e.Source)
	}
	parts = append(parts, ago(now.Sub(e.At)))
	if n := utf8.RuneCountInString(e.Text); n > clipTitleLength || strings.Contains(strings.TrimSpace(e.Text), "\n") {
		parts = append(parts, fmt.Sprintf("%d characters", n))
	}
	return strings.Join(parts, " · ")
}

func ago(d time.Duration) string {
	switch {
	case d < time.Minute:
		return "just now"
	case d < time.Hour:
		return fmt.Sprintf("%d min ago", int(d/time.Minute))
	case d < 24*time.Hour:
		return fmt.Sprintf("%d h ago", int(d/time.Hour))
	}
	return fmt.Sprintf("%d d ago", int(d/(24*time.Hour)))
}
