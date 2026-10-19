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
	"time"

	"agi/internal/clipboard"
	"agi/internal/config"
	"agi/internal/enginesetup"
	"agi/internal/indexing"
	"agi/internal/settings"
	"agi/internal/update"
)

// Setup is the installer of file search as the service uses it; *enginesetup.Manager satisfies it.
type Setup interface {
	Installed() bool
	Start() error
	Cancel()
	State() enginesetup.State
	Remove() error
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
	// Updates asks whether a newer release exists; OpenURL opens an address in the user's browser.
	Updates Updates
	OpenURL func(address string) error
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
	UpdateFound   string // the version of a newer release the last check saw, or ""
	ConfigPath    string
	LogDir        string
	Version       string
}

// View returns the current settings.
func (s *Service) View() View {
	c := s.o.Store.Get()
	return View{
		Hotkey: c.Hotkey, DefaultHotkey: config.DefaultHotkey, Browser: c.Browser, SearchURL: c.SearchURL, Quicklinks: c.Quicklinks, Snippets: c.Snippets,
		Roots: c.Roots, FileSearch: c.Search.Enabled, Pace: c.Search.Pace,
		Autostart: s.o.Autostart(), CheckUpdates: c.CheckUpdates, UpdateFound: s.updateFound(),
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

// SetAutostart makes the launcher start (or not) when the user signs in.
func (s *Service) SetAutostart(on bool) string {
	if err := s.o.SetAutostart(on); err != nil {
		return err.Error()
	}
	return ""
}

func (s *Service) updateFound() string {
	rel, _ := s.o.Updates.Found()
	return rel.Version
}

// SetCheckUpdates turns the daily update check on or off.
func (s *Service) SetCheckUpdates(on bool) string {
	return reason(s.o.Store.Update(func(c *config.Config) { c.CheckUpdates = on }))
}

// UpdateView is what a check found: the newest release, whether it is newer than this build, or why
// the check failed.
type UpdateView struct {
	Latest    string
	Available bool
	Error     string
}

// CheckUpdate asks GitHub for the newest release now.
func (s *Service) CheckUpdate() UpdateView {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	rel, newer, err := s.o.Updates.Check(ctx)
	if err != nil {
		return UpdateView{Error: err.Error()}
	}
	return UpdateView{Latest: rel.Version, Available: newer}
}

// OpenUpdate opens the page of the newer release the last check found. The address comes from that
// check, never from the window.
func (s *Service) OpenUpdate() string {
	rel, ok := s.o.Updates.Found()
	if !ok {
		return "there is no newer release to open"
	}
	return reason(s.o.OpenURL(rel.URL))
}

// SaveQuicklinks replaces the quicklinks.
func (s *Service) SaveQuicklinks(links []config.Quicklink) string {
	return reason(s.o.Store.Update(func(c *config.Config) { c.Quicklinks = links }))
}

// SaveSnippets replaces the snippets.
func (s *Service) SaveSnippets(snippets []config.Snippet) string {
	return reason(s.o.Store.Update(func(c *config.Config) { c.Snippets = snippets }))
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

// SearchView is the File search page: what is installed, how an install is going, and how the engine is.
type SearchView struct {
	Enabled     bool
	Installed   bool
	Installing  bool
	NeedsUpdate bool
	// Step, Steps and StepName say where an install is; Done and Total are bytes of the download in progress.
	Step, Steps int
	StepName    string
	Done, Total int64
	Message     string // the last line uv printed
	Error       string // why the last install failed
	// EngineState is off, starting, warming, ready, restarting, failed or stopped.
	EngineState, EngineMessage string
	// Hosts and DownloadMB are what the page tells a person before they agree to the download.
	Hosts      []string
	DownloadMB int
	Pace       string
	// IndexState is waiting, indexing, uptodate or failed; IndexText is its sentence; Paused says indexing is stopped.
	IndexState, IndexText string
	Paused                bool
	Folders               []FolderView
}

// FolderView is a folder with where it is, in words.
type FolderView struct {
	Root  string
	State string // waiting, indexing, uptodate or failed
	Text  string
}

// Search returns the File search page; the window asks again every second or two.
func (s *Service) Search() SearchView {
	st := s.o.Setup.State()
	engineState, engineMessage := s.o.Engine()
	cfg := s.o.Store.Get()
	index := s.o.Indexing.Status()
	return SearchView{
		IndexState: string(index.State), IndexText: index.String(), Paused: index.Paused, Folders: s.folders(engineState),
		Enabled: cfg.Search.Enabled, Installed: st.Installed, Installing: st.Installing, NeedsUpdate: st.NeedsUpdate,
		Step: st.Progress.Step, Steps: st.Progress.Steps, StepName: st.Progress.Name,
		Done: st.Progress.Done, Total: st.Progress.Total, Message: st.Progress.Message, Error: st.Error,
		EngineState: engineState, EngineMessage: engineMessage,
		Hosts: enginesetup.Hosts, DownloadMB: int(enginesetup.DownloadBytes() >> 20), Pace: cfg.Search.Pace,
	}
}

// TurnOn switches file search on. If the engine is not installed yet (or is out of date) the download
// starts in the background, and file search is switched on when it finishes.
func (s *Service) TurnOn() string {
	st := s.o.Setup.State()
	if st.Installed && !st.NeedsUpdate {
		return reason(s.o.Store.Update(func(c *config.Config) { c.Search.Enabled = true }))
	}
	return s.install()
}

// Update installs the current engine over an installed one: it is stopped first, because its files are in use.
func (s *Service) Update() string { return s.install() }

func (s *Service) install() string {
	if st := s.o.Setup.State(); st.Installing {
		return "file search is already being installed"
	}
	if msg := reason(s.o.Store.Update(func(c *config.Config) { c.Search.Enabled = false })); msg != "" {
		return msg
	}
	return reason(s.o.Setup.Start())
}

// CancelInstall stops a running install. What was downloaded is kept, so the next one starts where it ended.
func (s *Service) CancelInstall() { s.o.Setup.Cancel() }

// TurnOff stops the engine and leaves its files, so turning file search on again is instant.
func (s *Service) TurnOff() string {
	return reason(s.o.Store.Update(func(c *config.Config) { c.Search.Enabled = false }))
}

// RemoveFiles turns file search off and deletes the engine and the models (about a gigabyte). The index it
// built stays, and works again after the engine is installed again.
func (s *Service) RemoveFiles() string {
	if msg := s.TurnOff(); msg != "" {
		return msg
	}
	return reason(s.o.Setup.Remove())
}

// folders describes each configured folder. While the engine is not running nothing is indexed, and the page
// says what the folder is waiting for.
func (s *Service) folders(engineState string) []FolderView {
	var out []FolderView
	for _, f := range s.o.Indexing.Folders() {
		view := FolderView{Root: f.Root, State: string(f.State)}
		switch {
		case engineState == "off":
			view.State, view.Text = "waiting", "Waits for file search to be turned on"
		case f.State == indexing.Indexing && f.FilesTotal == 0:
			view.Text = "Looking at what is in it"
		case f.State == indexing.Indexing:
			view.Text = fmt.Sprintf("Indexing: %d of %d files", f.FilesDone, f.FilesTotal)
		case f.State == indexing.UpToDate:
			view.Text = fmt.Sprintf("%d files searchable", f.Files)
		case f.State == indexing.Failed:
			view.Text = "Could not be indexed: " + f.Message
		default:
			view.Text = "Waiting"
		}
		out = append(out, view)
	}
	return out
}

// PauseIndexing stops indexing at the next batch. Searching is not affected, and nothing is lost: a file is
// recorded only once its vectors are stored.
func (s *Service) PauseIndexing() { s.o.Indexing.Pause() }

// ResumeIndexing lets indexing go on.
func (s *Service) ResumeIndexing() { s.o.Indexing.Resume() }

// ClipboardView is the Clipboard page: whether the history is on, how much it holds, and the programs it leaves out.
type ClipboardView struct {
	Enabled bool
	Count   int
	// Ignore are the programs the user added; Defaults are the password managers always left out.
	Ignore   []string
	Defaults []string
	File     string
	// Hotkey opens the bar listing what was copied ("" means there is none); HotkeyError is why it could not be bound at start.
	Hotkey, HotkeyError string
}

// Clipboard returns the Clipboard page.
func (s *Service) Clipboard() ClipboardView {
	c := s.o.Store.Get().Clipboard
	return ClipboardView{
		Enabled: c.Enabled, Count: s.o.Clipboard.Len(), Ignore: c.Ignore,
		Defaults: clipboard.DefaultIgnore, File: s.o.Clipboard.Path(),
		Hotkey: c.Hotkey, HotkeyError: s.o.ClipboardHotkeyError(),
	}
}

// SaveClipboard switches the history on or off and sets the programs it leaves out (blank lines are dropped) and
// its hotkey. A hotkey that cannot be bound is refused and the working one stays.
func (s *Service) SaveClipboard(enabled bool, ignore []string, hotkey string) string {
	var programs []string
	for _, name := range ignore {
		if name = strings.TrimSpace(name); name != "" {
			programs = append(programs, name)
		}
	}
	hotkey = strings.TrimSpace(hotkey)
	old := s.o.Store.Get().Clipboard.Hotkey
	if hotkey != old {
		if err := s.o.RebindClipboard(hotkey); err != nil {
			return err.Error()
		}
	}
	err := s.o.Store.Update(func(c *config.Config) {
		c.Clipboard = config.Clipboard{Enabled: enabled, Ignore: programs, Hotkey: hotkey}
	})
	if err != nil && hotkey != old {
		_ = s.o.RebindClipboard(old) // the window shows the refusal; the key that worked keeps working
	}
	return reason(err)
}

// ClearClipboard forgets everything the history holds.
func (s *Service) ClearClipboard() string {
	if err := s.o.Clipboard.Clear(); err != nil {
		return err.Error()
	}
	return ""
}
