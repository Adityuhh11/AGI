package settingsui

import (
	"fmt"
	"path/filepath"

	"agi/internal/config"
	"agi/internal/enginesetup"
	"agi/internal/indexing"
)

// The File search page: installing the engine, the folders it indexes, and how hard it works.

// SearchView is the File search page: what is installed, how an install is going, and how the engine is.
type SearchView struct {
	Enabled      bool
	InLauncher   bool // files are among the results of the default search
	GroupResults bool // file results are shown under a heading per kind
	Installed    bool
	Installing   bool
	NeedsUpdate  bool
	// Step, Steps and StepName say where an install is; Done and Total are bytes of the download in progress.
	Step, Steps int
	StepName    string
	Done, Total int64
	Message     string // the last line uv printed
	Error       string // why the last install failed
	// EngineState is off, starting, warming, ready, restarting, failed or stopped.
	EngineState, EngineMessage string
	// Device is where the model runs once the engine is ready: "vulkan", "cuda" or "cpu" (no usable graphics card).
	Device string
	// PDFPages, Photos and Videos are the indexing choices (config.PDFPageModes, PhotoModes, VideoModes).
	PDFPages, Photos, Videos string
	// Hosts and DownloadMB are what the page tells a person before they agree to the download.
	Hosts      []string
	DownloadMB int
	// Data says where the download and the index are saved and whether that can still be changed.
	Data DataFolder
	Pace string
	// HoldIndexing says the folders wait for StartIndexing; Indexed is how many files the index holds, which decides
	// whether a change of choice is a draft the page keeps until it is saved.
	HoldIndexing bool
	Indexed      int
	// IndexDone and IndexTotal are the files of the folder being indexed (the total is 0 while it is scanned).
	IndexDone, IndexTotal int
	// IndexState is waiting, indexing, uptodate or failed; IndexText is its sentence; Paused says indexing is stopped.
	IndexState, IndexText string
	Paused                bool
	Folders               []FolderView
	// Exclude are the folders inside the searched ones that are left out.
	Exclude []string
	// Surveys is what each folder holds and Estimate what indexing them will take on this computer.
	Surveys  []FolderSurvey
	Estimate Estimates
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
	engine := s.o.Engine()
	cfg := s.o.Store.Get()
	index := s.o.Indexing.Status()
	onCPU := engine.Device == "cpu"
	return SearchView{
		Surveys: s.surveys(cfg, onCPU), Estimate: s.estimates(cfg, onCPU),
		IndexState: string(index.State), IndexText: index.String(), Paused: index.Paused, Folders: s.folders(string(engine.State), cfg.Search.HoldIndexing),
		Enabled: cfg.Search.Enabled, InLauncher: cfg.Search.InLauncher, GroupResults: cfg.Search.GroupResults, Installed: st.Installed, Installing: st.Installing, NeedsUpdate: st.NeedsUpdate,
		Step: st.Progress.Step, Steps: st.Progress.Steps, StepName: st.Progress.Name,
		Done: st.Progress.Done, Total: st.Progress.Total, Message: st.Progress.Message, Error: st.Error,
		EngineState: string(engine.State), EngineMessage: engine.Message, Device: engine.Device,
		Hosts: enginesetup.Hosts, DownloadMB: int(enginesetup.DownloadBytes() >> 20), Pace: cfg.Search.Pace,
		PDFPages: cfg.Search.PDFPages, Photos: cfg.Search.Photos, Videos: cfg.Search.Videos,
		Exclude: cfg.Exclude, Data: s.dataFolder(cfg), HoldIndexing: cfg.Search.HoldIndexing, Indexed: s.indexed(), IndexDone: index.FilesDone, IndexTotal: index.FilesTotal,
	}
}

// indexed counts the files the index holds, folder by folder.
func (s *Service) indexed() int {
	n := 0
	for _, f := range s.o.Indexing.Folders() {
		n += f.Files
	}
	return n
}

// StartIndexing lets the folders be indexed after a first download, with the choices the person made meanwhile.
func (s *Service) StartIndexing() string {
	return reason(s.o.Store.Update(func(c *config.Config) { c.Search.HoldIndexing = false }))
}

// SetIndexing sets what PDF pages, photos and videos are searched by. The next sync processes again only the kinds of
// file whose choice changed, and reuses everything already embedded.
func (s *Service) SetIndexing(pdfPages, photos, videos string) string {
	return reason(s.o.Store.Update(func(c *config.Config) {
		c.Search.PDFPages, c.Search.Photos, c.Search.Videos = pdfPages, photos, videos
	}))
}

// TurnOn switches file search on. If the engine is not installed yet (or is out of date) the download
// starts in the background, and file search is switched on when it finishes. Switching it on before any folder is
// chosen, or downloading it for the first time, holds indexing until StartIndexing, so the choices come first.
func (s *Service) TurnOn() string {
	st := s.o.Setup.State()
	chooseFirst := len(s.o.Store.Get().Roots) == 0
	if st.Installed && !st.NeedsUpdate {
		return reason(s.o.Store.Update(func(c *config.Config) {
			c.Search.Enabled = true
			c.Search.HoldIndexing = c.Search.HoldIndexing || chooseFirst
		}))
	}
	return s.install(!st.Installed || chooseFirst)
}

// Update installs the current engine over an installed one: it is stopped first, because its files are in use.
func (s *Service) Update() string { return s.install(false) }

// install downloads the engine. A first download (first) holds indexing, so the choices are made before any picture is
// embedded; an update of an installed engine does not, because the person has already chosen.
func (s *Service) install(first bool) string {
	if st := s.o.Setup.State(); st.Installing {
		return "file search is already being installed"
	}
	if msg := reason(s.o.Store.Update(func(c *config.Config) {
		c.Search.Enabled = false
		c.Search.HoldIndexing = c.Search.HoldIndexing || first
	})); msg != "" {
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

// RemoveEverything takes file search off the computer as if it had never been set up: the engine, the models, the index,
// the folders and what was chosen for them. Nothing of it is kept, so it can be set up again from nothing, for instance
// after switching to another version of AGI.
func (s *Service) RemoveEverything() string {
	if s.o.Setup.State().Installing {
		return "file search is being installed; cancel that first"
	}
	// The engine stops when the setting turns it off, before its files are deleted.
	if msg := reason(s.o.Store.Update(func(c *config.Config) {
		defaults := config.Default().Search
		defaults.DataDir = c.Search.DataDir // where it goes is the person's choice, not something to forget
		c.Search = defaults
		c.Roots, c.Exclude = nil, nil
	})); msg != "" {
		return msg
	}
	if err := s.o.Setup.Remove(); err != nil {
		return reason(err)
	}
	return reason(s.o.Setup.RemoveIndex())
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

// SetInLauncher puts files among the results of the default search, or keeps that to apps, the web and the rest
// (files are then searched in the Files mode).
func (s *Service) SetInLauncher(on bool) string {
	return reason(s.o.Store.Update(func(c *config.Config) { c.Search.InLauncher = on }))
}

// SetGroupResults shows file results under a heading per kind of file with the best match first, or as one list.
func (s *Service) SetGroupResults(on bool) string {
	return reason(s.o.Store.Update(func(c *config.Config) { c.Search.GroupResults = on }))
}

// SetPace sets how hard indexing works: fast, balanced or gentle.
func (s *Service) SetPace(pace string) string {
	return reason(s.o.Store.Update(func(c *config.Config) { c.Search.Pace = pace }))
}

// folders describes each configured folder. While the engine is not running nothing is indexed, and the page
// says what the folder is waiting for.
func (s *Service) folders(engineState string, hold bool) []FolderView {
	var out []FolderView
	for _, f := range s.o.Indexing.Folders() {
		view := FolderView{Root: f.Root, State: string(f.State)}
		switch {
		case engineState == "off":
			view.State, view.Text = "waiting", "Waits for file search to be turned on"
		case hold:
			view.State, view.Text = "waiting", "Waits for you to start indexing"
		case f.State == indexing.Indexing && f.FilesTotal == 0:
			view.Text = "Looking at what is in it"
		case f.State == indexing.Indexing:
			view.Text = fmt.Sprintf("Indexing: %d of %d files", f.FilesDone, f.FilesTotal) + indexing.LeftText(f.Remaining)
		case f.State == indexing.UpToDate && f.Files == 0:
			view.Text = "Nothing here AGI can read"
			if now := s.movedTo(f.Root); now != "" {
				view.Text += fmt.Sprintf(". Windows keeps this folder in %s now: add that one instead", now)
			}
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
