package palette

import (
	"fmt"
	"sort"
	"strings"
	"unicode/utf8"

	"agi/internal/config"
	"agi/internal/rank"
)

// Snippets is the user's snippets as the bar uses them; *snippets.Service satisfies it.
type Snippets interface {
	List() []config.Snippet
	// Paste types the snippet into the program the bar came from, and closes the bar when it has worked.
	Paste(name string) error
}

// snippetPreviewLength is how much of a snippet's first line its row shows.
const snippetPreviewLength = 40

// command is a built-in row: choosing it puts its prefix in the bar, which opens a mode. They are what the pins
// are seeded with, and they can be found by typing their names.
type command struct {
	id, title, prefix string
}

var commands = []command{
	{"cmd:clipboard", "Clipboard history", "clip: "},
	{"cmd:snippets", "Snippets", "snip: "},
	{"cmd:notes", "Notes", "note: "},
}

func commandFor(id string) (command, bool) {
	for _, c := range commands {
		if c.id == id {
			return c, true
		}
	}
	return command{}, false
}

func (c command) item() Item {
	return Item{ID: c.id, Kind: "cmd", Title: c.title, Subtitle: "Command", Fill: c.prefix}
}

// commandItems are the built-in rows whose names the text starts or abbreviates (`clip`, `snip`).
func commandItems(text string) []Item {
	if utf8.RuneCountInString(text) < 2 {
		return nil
	}
	var items []Item
	for _, c := range commands {
		if rank.Match(text, c.title).Strong {
			items = append(items, c.item())
		}
	}
	return items
}

// snippetItems answers the Snippets mode: the snippets whose names match the text (all of them for no text), best
// first. When there is nothing to list the second result says why.
func (s *Service) snippetItems(text string) ([]Item, string) {
	list := s.snips.List()
	if len(list) == 0 {
		return nil, "There are no snippets yet. Add one in Settings, under Snippets."
	}
	type scored struct {
		snippet config.Snippet
		score   float64
	}
	var found []scored
	for _, snippet := range list {
		if text == "" {
			found = append(found, scored{snippet, 0})
		} else if m := rank.Match(text, snippet.Name); m.Score > 0 {
			found = append(found, scored{snippet, m.Score})
		}
	}
	if len(found) == 0 {
		return nil, "No snippet is called like “" + text + "”."
	}
	sort.SliceStable(found, func(i, j int) bool { return found[i].score > found[j].score })
	items := make([]Item, len(found))
	for i, f := range found {
		items[i] = snippetItem(f.snippet)
	}
	return items, ""
}

// keywordItems are the snippets whose keyword is exactly what was typed: like a quicklink's trigger, a keyword
// is a shortcut, and snippets stay out of the results otherwise.
func (s *Service) keywordItems(text string) []Item {
	var items []Item
	for _, snippet := range s.snips.List() {
		if snippet.Keyword != "" && strings.EqualFold(snippet.Keyword, text) {
			items = append(items, snippetItem(snippet))
		}
	}
	return items
}

func snippetItem(snippet config.Snippet) Item {
	return Item{ID: "snippet:" + snippet.Name, Kind: "snippet", Title: snippet.Name, Subtitle: snippetPreview(snippet.Text)}
}

// snippetPreview is the first line of a snippet with its spaces tidied, cut to fit beside its name.
func snippetPreview(text string) string {
	for _, line := range strings.Split(text, "\n") {
		if line = strings.Join(strings.Fields(line), " "); line != "" {
			if utf8.RuneCountInString(line) > snippetPreviewLength {
				return string([]rune(line)[:snippetPreviewLength-1]) + "…"
			}
			return line
		}
	}
	return ""
}

// doSnippet pastes a snippet; that is all a snippet row can do.
func (s *Service) doSnippet(name, action string) error {
	if action != "open" {
		return fmt.Errorf("a snippet can only be pasted, not %q", action)
	}
	return s.snips.Paste(name)
}
