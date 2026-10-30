package config

import (
	"fmt"
	"strings"
	"time"
)

// Window is the daily stretch of the clock in which indexing may work (Search.IndexBetween, "22:00-07:00"). It may
// cross midnight. Outside it indexing waits, the way it does on battery; searching never does.
type Window struct{ From, To int } // minutes after midnight

// ParseWindow reads "HH:MM-HH:MM". An empty text is no window (nil): indexing may work at any hour.
func ParseWindow(text string) (*Window, error) {
	text = strings.TrimSpace(text)
	if text == "" {
		return nil, nil
	}
	from, to, ok := strings.Cut(text, "-")
	if !ok {
		return nil, fmt.Errorf("index_between %q must look like 22:00-07:00", text)
	}
	var w Window
	for i, part := range []string{from, to} {
		t, err := time.Parse("15:04", strings.TrimSpace(part))
		if err != nil {
			return nil, fmt.Errorf("index_between %q must look like 22:00-07:00", text)
		}
		minutes := t.Hour()*60 + t.Minute()
		if i == 0 {
			w.From = minutes
		} else {
			w.To = minutes
		}
	}
	if w.From == w.To {
		return nil, fmt.Errorf("index_between %q starts and ends at the same time", text)
	}
	return &w, nil
}

// Open says whether indexing may work at t (the end of the window is not part of it).
func (w *Window) Open(t time.Time) bool {
	if w == nil {
		return true
	}
	now := t.Hour()*60 + t.Minute()
	if w.From < w.To {
		return now >= w.From && now < w.To
	}
	return now >= w.From || now < w.To // across midnight
}

// Start is when the window opens, as "22:00".
func (w *Window) Start() string { return fmt.Sprintf("%02d:%02d", w.From/60, w.From%60) }
