// Package palette answers what is typed in the bar with the things that can be answered from memory in
// microseconds (applications, quicklinks and a web search), decides which of them are worth showing, and carries out what
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
	"os"
	"path/filepath"
	"strings"
	"time"
	"unicode/utf8"

	"agi/internal/apps"
	"agi/internal/clipboard"
	"agi/internal/config"
	"agi/internal/rank"
	"agi/internal/state"
	"agi/internal/web"
)

// The modes of the bar: what a search covers. The page cycles them with Tab; a prefix typed in the text
// (`app: code`) chooses one too, so the chip and the text never disagree.
const (
	ModeAll   = "all"
	ModeApps  = "apps"
	ModeFiles = "files"
	ModeWeb   = "web"
	ModeTags  = "tags"
	// ModeClip lists what was copied and ModeSnip the snippets. Tab does not reach them (`clip:`, `snip:` and the
	// built-in rows do).
	ModeClip = "clip"
	ModeSnip = "snip"
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
	// webMinLength is the shortest text that offers a web search: one letter is not a question.
	webMinLength = 2
	// tagRows is how many rows the Tags mode lists.
	tagRows = 20
	// clipRows is how many copied texts the Clipboard mode lists.
	clipRows = 20
	// clipTitleLength is how many characters of a copied text a row shows.
	clipTitleLength = 100
)

// Files is what the bar needs of the files service to open something that was tagged; *files.Service
// satisfies it. It closes the bar itself once the file is handed over.
type Files interface {
	Open(path string, line int) error
	Reveal(path string) error
}

// Clips is the clipboard history as the bar uses it; *clipboard.Manager satisfies it.
type Clips interface {
	Enabled() bool
	Search(words string, limit int) []clipboard.Entry
	// Paste types the entry into the program the bar came from; Copy only puts it on the clipboard. Both close the bar
	// when they have worked, and say why when they have not.
	Paste(id int64) error
	Copy(id int64) error
	Delete(id int64) error
}

// Browser is what the bar needs of the web package; *web.Web satisfies it.
type Browser interface {
	Search(text string) error
	Open(address string) error
	// Quicklink recognises a trigger word followed by text.
	Quicklink(typed string) (config.Quicklink, string, bool)
	// Find returns the quicklink with a trigger.
	Find(trigger string) (config.Quicklink, bool)
}

// Item is one row the bar can show.
type Item struct {
	// ID names the thing: the kind, a colon and the kind's own id.
	ID string
	// Kind is "app", "quicklink", "web", "file" (a tagged file), "tag", "clip", "snippet" or "cmd". The page lists quicklinks
	// first, then apps and files, and the web search last, as the fallback.
	Kind     string
	Title    string
	Subtitle string
	// Icon is a URL the page can load the row's icon from.
	Icon string
	// Actions are the extra things that can be done with it ("reveal", "admin"); opening is always possible.
	Actions []string
	// Fill, if set, is what choosing the row does instead of running it: the bar puts this text in its input (a pinned
	// quicklink's trigger, the prefix of a mode) and the person goes on typing.
	Fill string
}

// Answer is what the bar shows for the text typed.
type Answer struct {
	// Mode is the mode the text is searched in once any prefix is taken off.
	Mode string
	// Text is what is searched for.
	Text  string
	Items []Item
	// Note says why a mode has nothing to list (the clipboard history is off, nothing matches), or is empty.
	Note string
}

// Service answers queries and runs the chosen row.
type Service struct {
	catalog *apps.Catalog
	history *state.Store
	browser Browser
	files   Files
	clips   Clips
	snips   Snippets
	// openSettings shows the Settings window on a page (settingsui.Window.OpenAt).
	openSettings func(section string)
	dismiss      func()
	now     func() time.Time
}

// New returns a service. dismiss closes the bar after something was started, without giving the
// keyboard back (the program started owns it now).
func New(catalog *apps.Catalog, history *state.Store, browser Browser, files Files, clips Clips, snips Snippets, openSettings func(section string), dismiss func()) *Service {
	return &Service{catalog: catalog, history: history, browser: browser, files: files, clips: clips, snips: snips, openSettings: openSettings, dismiss: dismiss, now: time.Now}
}

// Query answers raw (what is in the input) in the mode the page is in.
func (s *Service) Query(raw, mode string) Answer {
	mode, text := parse(raw, mode)
	answer := Answer{Mode: mode, Text: text}
	if mode == ModeTags {
		answer.Items = s.tagItems(text)
		return answer
	}
	if mode == ModeClip {
		answer.Items, answer.Note = s.clipItems(text)
		return answer
	}
	if mode == ModeSnip {
		answer.Items, answer.Note = s.snippetItems(text)
		return answer
	}
	if text == "" || mode == ModeFiles {
		return answer
	}
	if mode == ModeAll {
		answer.Items = append(answer.Items, s.keywordItems(text)...)
	}
	if mode == ModeAll || mode == ModeWeb {
		if link, rest, ok := s.browser.Quicklink(text); ok {
			answer.Items = append(answer.Items, quicklinkItem(link, rest))
		}
	}
	if mode != ModeWeb {
		matches := s.catalog.Index.Query(text, appsModeRows+strongRows, s.history, s.now())
		for _, m := range pick(matches, mode, text) {
			answer.Items = append(answer.Items, s.item(m.App))
		}
	}
	if mode == ModeAll {
		answer.Items = append(answer.Items, commandItems(text)...)
		answer.Items = append(answer.Items, settingsItems(text)...)
	}
	if (mode == ModeAll || mode == ModeWeb) && utf8.RuneCountInString(text) >= webMinLength {
		answer.Items = append(answer.Items, Item{ID: "web:" + text, Kind: "web", Title: "Search the web for “" + text + "”", Subtitle: "Web"})
	}
	return answer
}

func quicklinkItem(link config.Quicklink, text string) Item {
	return Item{ID: "quicklink:" + link.Trigger + ":" + text, Kind: "quicklink", Title: link.Title + ": " + text, Subtitle: "Quicklink"}
}

// parse takes a mode prefix off the text, whatever the chip says: `app:` and `apps:` choose the Apps
// mode, `web:` and a leading `?` the Web mode, a leading `#` the Tags mode.
func parse(raw, mode string) (string, string) {
	if mode == "" {
		mode = ModeAll
	}
	if rest, ok := strings.CutPrefix(strings.TrimSpace(raw), "?"); ok {
		return ModeWeb, strings.TrimSpace(rest)
	}
	if rest, ok := strings.CutPrefix(strings.TrimSpace(raw), "#"); ok {
		return ModeTags, strings.TrimSpace(rest)
	}
	if mode == ModeTags {
		return mode, strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(raw), "#"))
	}
	if mode == ModeClip {
		return mode, strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(raw), "clip:"))
	}
	if mode == ModeSnip {
		return mode, strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(raw), "snip:"))
	}
	if name, rest, found := strings.Cut(raw, ":"); found {
		switch strings.ToLower(strings.TrimSpace(name)) {
		case "app", "apps":
			return ModeApps, strings.TrimSpace(rest)
		case "web":
			return ModeWeb, strings.TrimSpace(rest)
		case "clip", "clipboard":
			return ModeClip, strings.TrimSpace(rest)
		case "snip", "snippet", "snippets":
			return ModeSnip, strings.TrimSpace(rest)
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
	if raw, ok := strings.CutPrefix(id, "clip:"); ok {
		return s.doClip(raw, action)
	}
	if name, ok := strings.CutPrefix(id, "snippet:"); ok {
		return s.doSnippet(name, action)
	}
	if page, ok := settingsPageFor(id); ok {
		return s.doSettings(page, action)
	}
	if path, ok := strings.CutPrefix(id, "file:"); ok {
		switch action {
		case "open":
			return s.files.Open(path, 0)
		case "reveal":
			return s.files.Reveal(path)
		}
		return fmt.Errorf("unknown action %q", action)
	}
	if text, ok := strings.CutPrefix(id, "web:"); ok {
		return s.browse(action, func() error { return s.browser.Search(text) })
	}
	if rest, ok := strings.CutPrefix(id, "quicklink:"); ok {
		trigger, text, _ := strings.Cut(rest, ":")
		link, found := s.browser.Find(trigger)
		if !found {
			return fmt.Errorf("there is no quicklink %q any more", trigger)
		}
		return s.browse(action, func() error { return s.browser.Open(web.Expand(link.URL, text)) })
	}
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

// browse opens something in the browser and closes the bar.
func (s *Service) browse(action string, open func() error) error {
	if action != "open" {
		return fmt.Errorf("a web row can only be opened, not %q", action)
	}
	if err := open(); err != nil {
		return err
	}
	s.dismiss()
	return nil
}

// tagItems answers the Tags mode. Text that starts with the name of a tag lists what carries it, narrowed by
// any words after the name (`work budget`); text that is not a whole tag name, or nothing, lists the tags.
func (s *Service) tagItems(text string) []Item {
	word, rest, _ := strings.Cut(text, " ")
	rest = strings.TrimSpace(rest)
	if tag, ok := state.NormaliseTag(word); ok {
		if ids := s.history.Tagged(tag); len(ids) > 0 {
			return s.taggedItems(ids, rest)
		}
	}
	var items []Item
	for _, tag := range s.history.Tags() {
		if word == "" || rank.Match(word, tag.Name).Score > 0 {
			items = append(items, Item{ID: "tag:" + tag.Name, Kind: "tag", Title: "#" + tag.Name, Subtitle: countOf(tag.Count)})
		}
	}
	return items[:min(len(items), tagRows)]
}

func countOf(n int) string {
	if n == 1 {
		return "1 item"
	}
	return fmt.Sprintf("%d items", n)
}

// taggedItems turns tagged ids into rows, leaving out what is gone and, when filter is not empty, what
// does not match it.
func (s *Service) taggedItems(ids []string, filter string) []Item {
	var items []Item
	for _, id := range ids {
		item, ok := s.itemFor(id)
		if !ok || (filter != "" && rank.Match(filter, item.Title).Score == 0) {
			continue
		}
		items = append(items, item)
	}
	return items[:min(len(items), tagRows)]
}

// itemFor is the row for a thing that can carry a tag or a pin: an app, a quicklink that is still
// configured (as its trigger alone, with no text), or a file that still exists.
func (s *Service) itemFor(id string) (Item, bool) {
	if rest, ok := strings.CutPrefix(id, "quicklink:"); ok {
		trigger, _, _ := strings.Cut(rest, ":")
		link, found := s.browser.Find(trigger)
		if !found {
			return Item{}, false
		}
		return Item{ID: "quicklink:" + link.Trigger + ":", Kind: "quicklink", Title: link.Title, Subtitle: "Quicklink", Fill: link.Trigger + " "}, true
	}
	if c, ok := commandFor(id); ok {
		return c.item(), true
	}
	if p, ok := settingsPageFor(id); ok {
		return p.item(), true
	}
	if name, ok := strings.CutPrefix(id, "snippet:"); ok {
		for _, snippet := range s.snips.List() {
			if snippet.Name == name {
				return snippetItem(snippet), true
			}
		}
		return Item{}, false
	}
	if appID, ok := strings.CutPrefix(id, "app:"); ok {
		app, found := s.catalog.Index.Find(appID)
		return s.item(app), found
	}
	if path, ok := strings.CutPrefix(id, "file:"); ok {
		if _, err := os.Stat(path); err != nil {
			return Item{}, false
		}
		return Item{ID: id, Kind: "file", Title: filepath.Base(path), Subtitle: filepath.Dir(path), Actions: []string{"reveal", "copy-path"}}, true
	}
	return Item{}, false
}

// TagInfo is a tag and how many things carry it.
type TagInfo struct {
	Name  string
	Count int
}

// Tags lists every tag, the most used first.
func (s *Service) Tags() []TagInfo {
	var tags []TagInfo
	for _, tag := range s.history.Tags() {
		tags = append(tags, TagInfo{Name: tag.Name, Count: tag.Count})
	}
	return tags
}

// TagsOf says which tags each of the things carries; things without any are left out.
func (s *Service) TagsOf(ids []string) map[string][]string {
	found := map[string][]string{}
	for _, id := range ids {
		if tags := s.history.TagsOf(id); len(tags) > 0 {
			found[id] = tags
		}
	}
	return found
}

// Tag puts the tag on a thing, or takes it off if it already has it, and returns the tags the thing has now.
// Apps ("app:...") and files ("file:...") can be tagged.
func (s *Service) Tag(id, tag string) ([]string, error) {
	if !strings.HasPrefix(id, "app:") && !strings.HasPrefix(id, "file:") {
		return nil, fmt.Errorf("%q cannot be tagged", id)
	}
	if _, err := s.history.ToggleTag(id, tag); err != nil {
		return nil, err
	}
	return s.history.TagsOf(id), nil
}

// Pinned is what the bar lists when nothing is typed: the pinned things in the order they were pinned,
// without the ones that are gone (an uninstalled app, a deleted file, a removed quicklink), which stay
// pinned in case they come back.
func (s *Service) Pinned() []Item {
	var items []Item
	for _, id := range s.history.Pins() {
		if item, ok := s.itemFor(id); ok {
			items = append(items, item)
		}
	}
	return items
}

// PinIDs lists the ids of everything pinned, for the page to mark pinned rows.
func (s *Service) PinIDs() []string { return s.history.Pins() }

// TogglePin pins a thing, or unpins it if it is pinned, and reports whether it is pinned now. Apps, files and
// quicklinks can be pinned; a quicklink is pinned as its trigger, not with whatever was typed after it.
func (s *Service) TogglePin(id string) (bool, error) {
	switch {
	case strings.HasPrefix(id, "app:"), strings.HasPrefix(id, "file:"), strings.HasPrefix(id, "snippet:"):
	case strings.HasPrefix(id, "quicklink:"):
		trigger, _, _ := strings.Cut(strings.TrimPrefix(id, "quicklink:"), ":")
		id = "quicklink:" + trigger + ":"
	default:
		_, command := commandFor(id)
		_, page := settingsPageFor(id)
		if !command && !page {
			return false, fmt.Errorf("%q cannot be pinned", id)
		}
	}
	return s.history.TogglePin(id)
}
