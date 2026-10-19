package rank

import (
	"testing"
	"time"
)

func TestMatchTiers(t *testing.T) {
	tests := []struct {
		query, title  string
		match, strong bool
	}{
		{"vsc", "Visual Studio Code", true, true},    // initials
		{"vscode", "Visual Studio Code", true, true}, // initials, the last word whole
		{"code", "Visual Studio Code", true, true},   // the start of a later word
		{"vis code", "Visual Studio Code", true, true},
		{"visual", "Visual Studio Code", true, true}, // prefix
		{"VISUAL STUDIO CODE", "visual studio code", true, true},
		{"chr", "Chrome", true, true},
		{"ppt", "PowerPoint", true, false},    // letters in order from a word start: weak
		{"chr", "Character Map", true, false}, // weak, and below Chrome (see the next test)
		{"zzz", "Visual Studio Code", false, false},
		{"", "Chrome", false, false},
		{"   ", "Chrome", false, false},
		{"x", "Visual Studio Code", false, false}, // one letter that starts no word
		{"zip", "7-Zip File Manager", true, true}, // a word after a digit
		{"café", "Café Notes", true, true},
		{"pp", "PowerPoint", true, true}, // camel-case humps are words
	}
	for _, tt := range tests {
		got := Match(tt.query, tt.title)
		if (got.Score > 0) != tt.match || got.Strong != tt.strong {
			t.Errorf("Match(%q, %q) = %+v, want match=%v strong=%v", tt.query, tt.title, got, tt.match, tt.strong)
		}
	}
}

func TestBetterMatchesScoreHigher(t *testing.T) {
	order := []struct{ query, better, worse string }{
		{"chr", "Chrome", "Character Map"},
		{"note", "Notes", "Notepad"},            // the shorter title wins the tie
		{"vs", "Vs Tool", "Visual Studio Code"}, // a prefix beats initials
		{"code", "Code", "Visual Studio Code"},  // the whole title beats a later word
		{"visual studio", "Visual Studio Code", "Microsoft Visual Studio Installer"},
	}
	for _, tt := range order {
		better, worse := Match(tt.query, tt.better).Score, Match(tt.query, tt.worse).Score
		if better <= worse {
			t.Errorf("query %q: %q scored %.3f, %q scored %.3f; the first must be higher", tt.query, tt.better, better, tt.worse, worse)
		}
	}
}

func TestWordsSplitOnPunctuationDigitsAndCamelCase(t *testing.T) {
	var got []string
	for _, w := range wordsOf("7-Zip PowerPoint_2024 (x64)") {
		got = append(got, string(w.text))
	}
	want := []string{"7", "zip", "power", "point", "2024", "x64"}
	if len(got) != len(want) {
		t.Fatalf("words = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("words = %v, want %v", got, want)
		}
	}
}

func TestFrecencyDecaysByHalfLife(t *testing.T) {
	now := time.Date(2026, 10, 17, 12, 0, 0, 0, time.UTC)
	near := func(got, want float64) bool { return got > want-0.001 && got < want+0.001 }
	if got := Frecency([]time.Time{now}, now); !near(got, 1) {
		t.Errorf("a use just now counts %.3f, want 1", got)
	}
	if got := Frecency([]time.Time{now.Add(-halfLife)}, now); !near(got, 0.5) {
		t.Errorf("a use one half-life ago counts %.3f, want 0.5", got)
	}
	if got := Frecency([]time.Time{now.Add(time.Hour)}, now); !near(got, 1) {
		t.Errorf("a use dated in the future (a clock change) counts %.3f, want 1", got)
	}
	if got := Frecency(nil, now); got != 0 {
		t.Errorf("no uses count %.3f, want 0", got)
	}
	recent := Frecency([]time.Time{now.Add(-time.Hour)}, now)
	stale := Frecency([]time.Time{now.Add(-60 * 24 * time.Hour), now.Add(-61 * 24 * time.Hour)}, now)
	if recent <= stale {
		t.Errorf("one recent use (%.3f) must beat two stale ones (%.3f)", recent, stale)
	}
}

func TestCombineLiftsAMatchWithoutLettingUseBuryEverythingElse(t *testing.T) {
	if Combine(0.8, 0) != 0.8 {
		t.Errorf("no history must leave the match score alone")
	}
	if Combine(0.8, 3) <= Combine(0.8, 0) {
		t.Errorf("a favourite must beat an equal match")
	}
	// A weak match used a handful of times must not beat a clearly better match used once; only
	// steady daily use (the learned favourite) should overturn the match quality.
	if weak, strong := Combine(0.4, 5), Combine(0.9, 1); weak >= strong {
		t.Errorf("some use of a weak match (%.2f) beat a strong match used once (%.2f)", weak, strong)
	}
}
