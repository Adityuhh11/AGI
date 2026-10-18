// Package settingsui is the settings window's side in Go: the service the page calls to read and
// change the launcher's settings, and the window itself. The settings file stays the source of
// truth (internal/settings); this package only turns what a person does in the window into valid
// changes of it.
package settingsui

import (
	"fmt"
	"path/filepath"
	"runtime"
	"strings"

	"agi/internal/config"
	"agi/internal/settings"
)

// Options are the parts of the launcher the service needs; main supplies them.
type Options struct {
	Store *settings.Store
	// Rebind moves the global hotkey; it fails if the combination cannot be used.
	Rebind func(accelerator string) error
	// PickFolder shows the native folder picker and returns the folder, or "" if it was cancelled.
	PickFolder func() (string, error)
	// Browsers lists the installed browsers by name.
	Browsers func() []string
	// OpenFolder shows a folder in Explorer.
	OpenFolder func(path string) error
	// Open opens (or brings forward) the settings window.
	Open func()
	// Engine reports the file-search engine's state and message, for the diagnostics.
	Engine func() (state, message string)
	// LogDir is where agi.log is written. Version is the build's version.
	LogDir, Version string
}

// Service is bound to the frontend; its exported methods become TypeScript functions. Methods that
// change something return the reason it was refused, or "" when it worked.
type Service struct{ o Options }

// New returns the service.
func New(o Options) *Service { return &Service{o: o} }

// View is everything the window shows.
type View struct {
	Hotkey     string
	Browser    string
	SearchURL  string
	Quicklinks []config.Quicklink
	Roots      []string
	FileSearch bool
	Pace       string
	ConfigPath string
	LogDir     string
	Version    string
}

// View returns the current settings.
func (s *Service) View() View {
	c := s.o.Store.Get()
	return View{
		Hotkey: c.Hotkey, Browser: c.Browser, SearchURL: c.SearchURL, Quicklinks: c.Quicklinks,
		Roots: c.Roots, FileSearch: c.Search.Enabled, Pace: c.Search.Pace,
		ConfigPath: s.o.Store.Path(), LogDir: s.o.LogDir, Version: s.o.Version,
	}
}

// Open opens the settings window (the bar and the tray call it).
func (s *Service) Open() { s.o.Open() }

// Browsers lists the installed browsers by name.
func (s *Service) Browsers() []string { return s.o.Browsers() }

// SaveGeneral changes the hotkey, the browser and the web search address. A hotkey that cannot be
// bound is refused and the working one stays.
func (s *Service) SaveGeneral(hotkey, browser, searchURL string) string {
	old := s.o.Store.Get().Hotkey
	hotkey = strings.TrimSpace(hotkey)
	if hotkey != old {
		if err := s.o.Rebind(hotkey); err != nil {
			return err.Error()
		}
	}
	err := s.o.Store.Update(func(c *config.Config) {
		c.Hotkey, c.Browser, c.SearchURL = hotkey, strings.TrimSpace(browser), strings.TrimSpace(searchURL)
	})
	if err != nil {
		if hotkey != old {
			_ = s.o.Rebind(old) // the window shows the refusal; the key that worked keeps working
		}
		return reason(err)
	}
	return ""
}

// SaveQuicklinks replaces the quicklinks.
func (s *Service) SaveQuicklinks(links []config.Quicklink) string {
	return reason(s.o.Store.Update(func(c *config.Config) { c.Quicklinks = links }))
}

// AddFolder asks for a folder and adds it to the indexed ones. Cancelling the picker changes nothing.
func (s *Service) AddFolder() string {
	folder, err := s.o.PickFolder()
	if err != nil {
		return fmt.Sprintf("the folder picker failed: %v", err)
	}
	if folder == "" {
		return ""
	}
	folder = filepath.Clean(folder)
	return reason(s.o.Store.Update(func(c *config.Config) { c.Roots = append(c.Roots, folder) }))
}

// RemoveFolder stops indexing a folder and drops what it held from the index. Exclusions inside it go too.
func (s *Service) RemoveFolder(folder string) string {
	return reason(s.o.Store.Update(func(c *config.Config) {
		id := config.SourceID(folder)
		kept := c.Roots[:0:0]
		for _, root := range c.Roots {
			if config.SourceID(root) != id {
				kept = append(kept, root)
			}
		}
		c.Roots = kept
		var exclude []string
		for _, skipped := range c.Exclude {
			if len(config.ExcludedUnder(folder, []string{skipped})) == 0 {
				exclude = append(exclude, skipped)
			}
		}
		c.Exclude = exclude
	}))
}

// SetPace sets how hard indexing works: fast, balanced or gentle.
func (s *Service) SetPace(pace string) string {
	return reason(s.o.Store.Update(func(c *config.Config) { c.Search.Pace = pace }))
}

// OpenFolder shows the settings folder ("config") or the log folder ("logs") in Explorer.
func (s *Service) OpenFolder(which string) string {
	var path string
	switch which {
	case "config":
		path = filepath.Dir(s.o.Store.Path())
	case "logs":
		path = s.o.LogDir
	default:
		return "unknown folder " + which
	}
	return reason(s.o.OpenFolder(path))
}

// Diagnostics is the text "Copy diagnostics" puts on the clipboard: what a bug report needs and
// nothing about the user's files or settings beyond what is listed.
func (s *Service) Diagnostics() string {
	c := s.o.Store.Get()
	engineState, engineMessage := s.o.Engine()
	var b strings.Builder
	fmt.Fprintf(&b, "AGI %s on %s/%s\n", s.o.Version, runtime.GOOS, runtime.GOARCH)
	fmt.Fprintf(&b, "file search: enabled=%v engine=%s %s\n", c.Search.Enabled, engineState, engineMessage)
	fmt.Fprintf(&b, "folders: %d, quicklinks: %d, pace: %s\n", len(c.Roots), len(c.Quicklinks), c.Search.Pace)
	return b.String()
}

// reason is the sentence the window shows for an error, without the package prefix it carries.
func reason(err error) string {
	if err == nil {
		return ""
	}
	return strings.TrimPrefix(err.Error(), "settings: ")
}
