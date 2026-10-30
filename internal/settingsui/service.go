// Package settingsui is the settings window's side in Go: the service the page calls to read and
// change the launcher's settings, and the window itself. The settings file stays the source of
// truth (internal/settings); this package only turns what a person does in the window into valid
// changes of it.
package settingsui

import (
	"context"
	"fmt"
	"path/filepath"
	"runtime"
	"strings"
	"sync"

	"omni/internal/config"
	"omni/internal/enginesetup"
	"omni/internal/indexing"
	"omni/internal/settings"
	"omni/internal/sidecar"
	"omni/internal/update"
)

// Setup is the installer of file search as the service uses it; *enginesetup.Manager satisfies it.
type Setup interface {
	Installed() bool
	Start() error
	Cancel()
	State() enginesetup.State
	Remove() error
	RemoveIndex() error
}

// Indexing is the indexing manager as the service uses it; *indexing.Manager satisfies it.
type Indexing interface {
	Pause()
	Resume()
	Status() indexing.Status
	Folders() []indexing.FolderStatus
}

// ClipboardHistory is the clipboard history as the service uses it; *clipboard.Manager satisfies it.
type ClipboardHistory interface {
	Len() int
	Path() string
	Clear() error
}

// Installer is the update installer as the service uses it; *update.Installer satisfies it.
type Installer interface {
	Start(asset update.Asset, exePath string, done func(error)) error
	Cancel()
	Progress() update.Progress
}

// Updates is the update checker as the service uses it; *update.Checker satisfies it.
type Updates interface {
	Check(ctx context.Context) (update.Release, bool, error)
	Found() (update.Release, bool)
}

// Options are the parts of the launcher the service needs; main supplies them.
type Options struct {
	Store *settings.Store
	// Indexing reports where each folder is and pauses and resumes indexing.
	Indexing Indexing
	// Place knows where the search engine and the index are, and moves where a first install goes.
	Place Place
	// Surveys counts what the folders hold, for the estimates.
	Surveys Surveys
	// Setup installs, updates and removes the engine. When an install finishes well, whoever built it turns
	// file search on (main does).
	Setup Setup
	// Rebind moves the global hotkey; it fails if the combination cannot be used.
	Rebind func(accelerator string) error
	// PickFolder shows the native folder picker and returns the folder, or "" if it was cancelled.
	PickFolder func() (string, error)
	// Browsers lists the installed browsers by name.
	Browsers func() []string
	// OpenFolder shows a folder in Explorer.
	OpenFolder func(path string) error
	// Autostart says whether the launcher starts when the user signs in; SetAutostart changes that.
	Autostart    func() bool
	SetAutostart func(on bool) error
	// Clipboard is the history of what was copied. RebindClipboard moves its hotkey (empty turns it off) and fails if
	// the combination cannot be used; ClipboardHotkeyError says why it could not be bound at start, if it could not.
	Clipboard            ClipboardHistory
	RebindClipboard      func(accelerator string) error
	ClipboardHotkeyError func() string
	// RebindNotes moves the notes window's hotkey (empty turns it off) and fails if the combination cannot be used;
	// NotesHotkeyError says why it could not be bound at start, if it could not.
	RebindNotes      func(accelerator string) error
	NotesHotkeyError func() string
	// Updates asks whether a newer release exists; OpenURL opens an address in the user's browser.
	Updates Updates
	OpenURL func(address string) error
	// Installer puts a newer release in place of the program at ExePath, and Restart then starts the new one and ends
	// this one (it returns only if it could not).
	Installer Installer
	ExePath   string
	Restart   func() error
	// Open opens (or brings forward) the settings window; OpenSetup the first-run guide.
	Open      func()
	OpenSetup func()
	// Engine reports the file-search engine's state, why it is in it, and where its model runs.
	Engine func() sidecar.Status
	// LogDir is where omni.log is written. Version is the build's version.
	LogDir, Version string
}

// Service is bound to the frontend; its exported methods become TypeScript functions. Methods that
// change something return the reason it was refused, or "" when it worked.
type Service struct {
	o Options

	// movedTo says where the system keeps a user folder that a configured folder is the old place of (OneDrive on Windows); tests replace it.
	movedTo func(root string) string
	// known is where the system keeps a user folder ("Documents", "Downloads"); tests replace it.
	known func(name string) (string, error)
	// blocked says whether the system will not let this program read a folder, and reading it is also what makes a Mac ask the person for
	// permission; tests replace it.
	blocked func(root string) bool

	probeMu sync.Mutex
	probes  map[string]*probe // what isBlocked knows of each folder

	mu         sync.Mutex
	restartErr string // why the launcher could not restart itself after an update, if it could not
}

// New returns the service.
func New(o Options) *Service { return &Service{o: o, movedTo: movedTo, known: knownFolder, blocked: blocked} }

// View is everything the window shows.
type View struct {
	Hotkey string
	// DefaultHotkey is what the Reset button puts back.
	DefaultHotkey string
	Browser       string
	SearchURL     string
	Quicklinks    []config.Quicklink
	Snippets      []config.Snippet
	Roots         []string
	FileSearch    bool
	Pace          string
	Autostart     bool
	CheckUpdates  bool
	// SelectionEffect is the arrow-key lift in the bar; Timer is the pomodoro's lengths.
	SelectionEffect bool
	Theme           string
	Timer           config.Timer
	UpdateFound     string // the version of a newer release the last check saw, or ""
	// UpdateInstallable says that release has a zip that can be checked, so the page can offer to install it.
	UpdateInstallable bool
	ConfigPath        string
	LogDir            string
	Version           string
}

// View returns the current settings.
func (s *Service) View() View {
	c := s.o.Store.Get()
	return View{
		Hotkey: c.Hotkey, DefaultHotkey: config.DefaultHotkey, Browser: c.Browser, SearchURL: c.SearchURL, Quicklinks: c.Quicklinks, Snippets: c.Snippets,
		Roots: c.Roots, FileSearch: c.Search.Enabled, Pace: c.Search.Pace,
		Autostart: s.o.Autostart(), CheckUpdates: c.CheckUpdates, SelectionEffect: c.UI.SelectionEffect, Theme: c.UI.Theme, Timer: c.Timer, UpdateFound: s.updateFound(), UpdateInstallable: s.updateInstallable(),
		ConfigPath: s.o.Store.Path(), LogDir: s.o.LogDir, Version: s.o.Version,
	}
}

// Open opens the settings window (the bar and the tray call it).
func (s *Service) Open() { s.o.Open() }

// OpenSetup opens the first-run guide again (the About page and the tray call it).
func (s *Service) OpenSetup() { s.o.OpenSetup() }

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

// SetAutostart makes the launcher start (or not) when the user signs in.
func (s *Service) SetAutostart(on bool) string {
	if err := s.o.SetAutostart(on); err != nil {
		return err.Error()
	}
	return ""
}

// SetSelectionEffect turns the lift of the selected result (while the arrow keys move) on or off.
func (s *Service) SetSelectionEffect(on bool) string {
	return reason(s.o.Store.Update(func(c *config.Config) { c.UI.SelectionEffect = on }))
}

// Theme is the colours every window wears; each window asks once when it opens and then follows the "ui:theme" event.
func (s *Service) Theme() string { return s.o.Store.Get().UI.Theme }

// SetTheme changes the colours of every window; a name that is not one of config.Themes is refused.
func (s *Service) SetTheme(name string) string {
	return reason(s.o.Store.Update(func(c *config.Config) { c.UI.Theme = name }))
}

// SaveTimer sets the pomodoro's lengths; a value out of range is refused and nothing changes.
func (s *Service) SaveTimer(t config.Timer) string {
	return reason(s.o.Store.Update(func(c *config.Config) { c.Timer = t }))
}

// SaveQuicklinks replaces the quicklinks.
func (s *Service) SaveQuicklinks(links []config.Quicklink) string {
	return reason(s.o.Store.Update(func(c *config.Config) { c.Quicklinks = links }))
}

// SaveSnippets replaces the snippets.
func (s *Service) SaveSnippets(snippets []config.Snippet) string {
	return reason(s.o.Store.Update(func(c *config.Config) { c.Snippets = snippets }))
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
	engine := s.o.Engine()
	var b strings.Builder
	fmt.Fprintf(&b, "Omni %s on %s/%s\n", s.o.Version, runtime.GOOS, runtime.GOARCH)
	fmt.Fprintf(&b, "file search: enabled=%v engine=%s %s device=%s\n", c.Search.Enabled, engine.State, engine.Message, engine.Device)
	fmt.Fprintf(&b, "folders: %d, quicklinks: %d, pace: %s, pdf pages: %s, photos: %s, videos: %s, audio: %s\n",
		len(c.Roots), len(c.Quicklinks), c.Search.Pace, c.Search.PDFPages, c.Search.Photos, c.Search.Videos, c.Search.Audio)
	return b.String()
}

// reason is the sentence the window shows for an error, without the package prefix it carries.
func reason(err error) string {
	if err == nil {
		return ""
	}
	return strings.TrimPrefix(err.Error(), "settings: ")
}
