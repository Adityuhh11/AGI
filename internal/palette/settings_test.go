package palette

import (
	"strings"
	"testing"
)

func TestTypingSettingsOffersThePagesAndOnlyTheStartOfATitleCounts(t *testing.T) {
	s, _ := newService(t)
	titlesOf := func(typed string) string {
		var out []string
		for _, item := range s.Query(typed, ModeAll).Items {
			if item.Subtitle == "Settings" {
				out = append(out, item.Title)
			}
		}
		return strings.Join(out, "|")
	}
	for typed, want := range map[string]string{
		"settings":         "Settings|Settings: Quicklinks|Settings: Snippets|Settings: File search|Settings: Clipboard|Settings: About",
		"set":              "Settings|Settings: Quicklinks|Settings: Snippets|Settings: File search|Settings: Clipboard|Settings: About",
		"settings file":    "Settings: File search",
		"Settings: Clip":   "Settings: Clipboard",
		"settings   about": "Settings: About",
		"se":               "", // too short to be a request
		"clip":             "", // the Clipboard mode, not its settings
		"snip":             "",
		"file search":      "", // only the start of a title counts
	} {
		if got := titlesOf(typed); got != want {
			t.Errorf("typing %q gave settings rows %q, want %q", typed, got, want)
		}
	}
	if got := s.Query("settings", ModeApps).Items; len(got) != 0 {
		t.Errorf("the Apps mode listed settings rows: %+v", got)
	}
}

func TestChoosingASettingsRowOpensThatPageAndClosesTheBar(t *testing.T) {
	s, dismissed := newService(t)
	var opened []string
	s.openSettings = func(section string) { opened = append(opened, section) }
	if err := s.Do("cmd:settings:search", "open", "settings file"); err != nil {
		t.Fatal(err)
	}
	if strings.Join(opened, "|") != "search" || *dismissed != 1 {
		t.Fatalf("opened %v, dismissed %d", opened, *dismissed)
	}
	if err := s.Do("cmd:settings:search", "reveal", ""); err == nil {
		t.Fatal("a settings row can only be opened")
	}
	if err := s.Do("cmd:settings:nonsense", "open", ""); err == nil {
		t.Fatal("an unknown page must not open anything")
	}
	if len(opened) != 1 {
		t.Fatalf("opened %v", opened)
	}
}

func TestASettingsPageCanBePinned(t *testing.T) {
	s, _ := newService(t)
	if on, err := s.TogglePin("cmd:settings:search"); err != nil || !on {
		t.Fatalf("pin = %v, %v", on, err)
	}
	if pinned := s.Pinned(); len(pinned) != 1 || pinned[0].Title != "Settings: File search" {
		t.Fatalf("pinned = %+v", pinned)
	}
}
