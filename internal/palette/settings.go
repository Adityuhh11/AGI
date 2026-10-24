package palette

import (
	"fmt"
	"strings"
)

// settingsPage is a row that opens the Settings window on one of its pages. The section ids are the page's own
// (frontend/src/Settings.svelte).
type settingsPage struct {
	id, title, section string
}

var settingsPages = []settingsPage{
	{"cmd:settings:general", "Settings", "general"},
	{"cmd:settings:quicklinks", "Settings: Quicklinks", "quicklinks"},
	{"cmd:settings:snippets", "Settings: Snippets", "snippets"},
	{"cmd:settings:search", "Settings: File search", "search"},
	{"cmd:settings:clipboard", "Settings: Clipboard", "clipboard"},
	{"cmd:settings:about", "Settings: About", "about"},
}

// settingsMinLength is the shortest text that offers the settings rows: `se` is the start of too many things.
const settingsMinLength = 3

func (p settingsPage) item() Item {
	return Item{ID: p.id, Kind: "cmd", Title: p.title, Subtitle: "Settings"}
}

func settingsPageFor(id string) (settingsPage, bool) {
	for _, p := range settingsPages {
		if p.id == id {
			return p, true
		}
	}
	return settingsPage{}, false
}

// words lowers text and drops punctuation, so `settings file` is the start of “Settings: File search”.
func words(text string) string {
	return strings.Join(strings.Fields(strings.ToLower(strings.ReplaceAll(text, ":", " "))), " ")
}

// settingsItems are the Settings rows whose title starts with the text. Only the start counts, unlike the
// other commands: typing `clip` or `snip` must keep finding the Clipboard and Snippets modes, not their settings.
func settingsItems(text string) []Item {
	typed := words(text)
	if len(typed) < settingsMinLength {
		return nil
	}
	var items []Item
	for _, p := range settingsPages {
		if strings.HasPrefix(words(p.title), typed) {
			items = append(items, p.item())
		}
	}
	return items
}

// doSettings opens the Settings window on a page and closes the bar; that is all a settings row can do.
func (s *Service) doSettings(page settingsPage, action string) error {
	if action != "open" {
		return fmt.Errorf("a settings row can only be opened, not %q", action)
	}
	s.openSettings(page.section)
	s.dismiss()
	return nil
}
