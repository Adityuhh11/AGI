// Package palette answers what is typed in the bar with the things that can be answered from memory in
// microseconds (applications for now), decides which of them are worth showing, and carries out what
// the user does with one. Files are not here: they come from the memory engine, a moment later, through
// internal/search, and the page puts the two lists together.
//
// The service is bound to the frontend; its exported methods become TypeScript functions.
package palette

import (
	"errors"
	"fmt"
	"log"
	"net/url"
	"strings"
	"time"
	"unicode/utf8"

	"agi/internal/apps"
	"agi/internal/state"
)

// The modes of the bar: what a search covers. The page cycles them with Tab; a prefix typed in the text
// (`app: code`) chooses one too, so the chip and the text never disagree.
const (
	ModeAll   = "all"
	ModeApps  = "apps"
	ModeFiles = "files"
)

const (
	// strongRows is how many strong app matches the bar shows beside the files.
	strongRows = 5
	// weakRows is how many weak matches show when nothing matches strongly.
	weakRows = 3
	// shortcutLength is the longest text that may show a weak match: `ppt` finding PowerPoint is a
	// shortcut being typed, but a long query matching an app's letters in order is a coincidence.
	shortcutLength = 5
	// appsModeRows is how many matches the Apps mode lists, weak ones included.
	appsModeRows = 8
)

// Item is one row the bar can show.
type Item struct {
	// ID names the thing: the kind, a colon and the kind's own id.
	ID       string
	Kind     string
	Title    string
	Subtitle string
	// Icon is a URL the page can load the row's icon from.
	Icon string
	// Actions are the extra things that can be done with it ("reveal", "admin"); opening is always possible.
	Actions []string
}

// Answer is what the bar shows for the text typed.
type Answer struct {
	// Mode is the mode the text is searched in once any prefix is taken off.
	Mode string
	// Text is what is searched for.
	Text  string
	Items []Item
}

// Service answers queries and runs the chosen row.
type Service struct {
	catalog *apps.Catalog
	history *state.Store
	dismiss func()
	now     func() time.Time
}

// New returns a service. dismiss closes the bar after something was started, without giving the
// keyboard back (the program started owns it now).
func New(catalog *apps.Catalog, history *state.Store, dismiss func()) *Service {
	return &Service{catalog: catalog, history: history, dismiss: dismiss, now: time.Now}
}

// Query answers raw (what is in the input) in the mode the page is in.
func (s *Service) Query(raw, mode string) Answer {
	mode, text := parse(raw, mode)
	answer := Answer{Mode: mode, Text: text}
	if text == "" || mode == ModeFiles {
		return answer
	}
	matches := s.catalog.Index.Query(text, appsModeRows+strongRows, s.history, s.now())
	for _, m := range pick(matches, mode, text) {
		answer.Items = append(answer.Items, s.item(m.App))
	}
	return answer
}

// parse takes a mode prefix off the text. `app:` and `apps:` choose the Apps mode whatever the chip says.
func parse(raw, mode string) (string, string) {
	if mode == "" {
		mode = ModeAll
	}
	if name, rest, found := strings.Cut(raw, ":"); found {
		if n := strings.ToLower(strings.TrimSpace(name)); n == "app" || n == "apps" {
			return ModeApps, strings.TrimSpace(rest)
		}
	}
	return mode, strings.TrimSpace(raw)
}

// pick chooses which matches to show. Beside files only strong matches show (so a question about
// meeting notes is not answered with an app called Notes), unless nothing matched strongly and what was
// typed is short enough to be a shortcut; the Apps mode lists everything that matched.
func pick(matches []apps.Match, mode, text string) []apps.Match {
	var strong []apps.Match
	for _, m := range matches {
		if m.Strong {
			strong = append(strong, m)
		}
	}
	switch {
	case mode == ModeApps:
		return matches[:min(len(matches), appsModeRows)]
	case len(strong) > 0:
		return strong[:min(len(strong), strongRows)]
	case !strings.Contains(text, " ") && utf8.RuneCountInString(text) <= shortcutLength:
		return matches[:min(len(matches), weakRows)]
	}
	return nil
}

func (s *Service) item(app apps.App) Item {
	item := Item{
		ID:       "app:" + app.ID,
		Kind:     "app",
		Title:    app.Name,
		Subtitle: "Application",
		Icon:     "/icon?id=" + url.QueryEscape(app.ID),
	}
	if _, ok := apps.Path(app); ok {
		item.Actions = []string{"reveal", "admin"}
	}
	return item
}

// Do carries out an action on a row: "open", or one of its Actions. typed is what was in the input, so a
// launch teaches the bar what you reach for after typing that.
func (s *Service) Do(id, action, typed string) error {
	appID, ok := strings.CutPrefix(id, "app:")
	if !ok {
		return fmt.Errorf("do not know how to run %q", id)
	}
	app, found := s.catalog.Index.Find(appID)
	if !found {
		return errors.New("that app is not in the list any more")
	}
	var err error
	switch action {
	case "open":
		err = apps.Launch(app)
	case "admin":
		err = apps.RunAsAdministrator(app)
	case "reveal":
		err = apps.ShowInExplorer(app)
	default:
		return fmt.Errorf("unknown action %q", action)
	}
	if errors.Is(err, apps.ErrGone) {
		s.catalog.Index.Remove(appID) // never a dead row that does nothing
		s.catalog.Refresh()
		return fmt.Errorf("%s is no longer installed", app.Name)
	}
	if err != nil {
		return err
	}
	if action == "open" {
		if err := s.history.Record(id, typed, s.now()); err != nil {
			log.Printf("palette: could not remember the launch: %v", err)
		}
	}
	s.dismiss()
	return nil
}
