package config

import (
	"testing"
	"time"
)

func TestWindowOpenAcrossMidnightAndWithin(t *testing.T) {
	at := func(h, m int) time.Time { return time.Date(2026, 10, 30, h, m, 0, 0, time.UTC) }
	tests := []struct {
		text string
		when time.Time
		open bool
	}{
		{"", at(12, 0), true},
		{"22:00-07:00", at(23, 30), true},
		{"22:00-07:00", at(3, 0), true},
		{"22:00-07:00", at(7, 0), false}, // the end is not part of it
		{"22:00-07:00", at(12, 0), false},
		{"22:00-07:00", at(22, 0), true},
		{"09:00-17:30", at(12, 0), true},
		{"09:00-17:30", at(17, 30), false},
		{"09:00-17:30", at(8, 59), false},
	}
	for _, tt := range tests {
		w, err := ParseWindow(tt.text)
		if err != nil {
			t.Fatalf("%q: %v", tt.text, err)
		}
		if got := w.Open(tt.when); got != tt.open {
			t.Errorf("%q at %s: open = %v, want %v", tt.text, tt.when.Format("15:04"), got, tt.open)
		}
	}
}

func TestParseWindowRefusesWhatIsNotAWindow(t *testing.T) {
	for _, text := range []string{"22:00", "22:00-", "25:00-07:00", "22:00-22:00", "night"} {
		if _, err := ParseWindow(text); err == nil {
			t.Errorf("ParseWindow(%q) worked, want an error", text)
		}
	}
}
