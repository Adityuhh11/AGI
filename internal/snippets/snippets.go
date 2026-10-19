// Package snippets pastes the texts the user keeps (an address, a reply, a SQL fragment) from the search bar,
// with the date, the time and what is on the clipboard filled in.
package snippets

import (
	"fmt"
	"strings"
	"time"

	"agi/internal/config"
)

// Expand fills in {date} (2026-10-19), {time} (14:05) and {clipboard}, and turns "{{" into "{". Anything else
// in braces is left as typed, so a snippet of code with braces stays what it was. The clipboard is read
// only if the text asks for it, and then once.
func Expand(text string, now time.Time, clipboard func() string) string {
	var out strings.Builder
	var clip *string
	for i := 0; i < len(text); {
		rest := text[i:]
		switch {
		case strings.HasPrefix(rest, "{{"):
			out.WriteByte('{')
			i += 2
		case strings.HasPrefix(rest, "{date}"):
			out.WriteString(now.Format("2006-01-02"))
			i += len("{date}")
		case strings.HasPrefix(rest, "{time}"):
			out.WriteString(now.Format("15:04"))
			i += len("{time}")
		case strings.HasPrefix(rest, "{clipboard}"):
			if clip == nil {
				text := clipboard()
				clip = &text
			}
			out.WriteString(*clip)
			i += len("{clipboard}")
		default:
			out.WriteByte(text[i])
			i++
		}
	}
	return out.String()
}

// Typer puts text into the program the bar was opened from; clipboard.Paster does.
type Typer interface {
	Paste(text string) error
}

// Service pastes snippets by name.
type Service struct {
	list      func() []config.Snippet
	typer     Typer
	clipboard func() string
	now       func() time.Time
}

// New returns a Service. list gives the snippets as they are now (the settings change them while the launcher
// runs), and clipboard the text on the clipboard.
func New(list func() []config.Snippet, typer Typer, clipboard func() string) *Service {
	return &Service{list: list, typer: typer, clipboard: clipboard, now: time.Now}
}

// List returns the snippets, in the order they were written.
func (s *Service) List() []config.Snippet { return s.list() }

// Paste types the snippet with this name (in any case) into the program the bar came from.
func (s *Service) Paste(name string) error {
	for _, snippet := range s.list() {
		if strings.EqualFold(snippet.Name, name) {
			return s.typer.Paste(Expand(snippet.Text, s.now(), s.clipboard))
		}
	}
	return fmt.Errorf("there is no snippet called %q any more", name)
}
