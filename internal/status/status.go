// Package status tells the UI about the launcher itself: what it is configured to do and, as
// later blocks add them, whether the search engine is ready and what is being indexed.
package status

import (
	"log"
	"sync/atomic"

	"omni/internal/config"
	"omni/internal/indexing"
	"omni/internal/sidecar"
)

// Info is the launcher's configuration and health as the UI shows it.
type Info struct {
	Hotkey string
	Roots  []string
	// FileSearch says whether file search is switched on.
	FileSearch bool
	// FilesInBar says files are among the results of the default search; the Files mode is there whenever FileSearch is.
	FilesInBar bool
	// SelectionEffect and GroupResults are the bar's two feel settings (config.UI, config.Search).
	SelectionEffect, GroupResults bool
	// HotkeyError is why the hotkey does not work (another app owns it), or empty.
	HotkeyError string
	// Engine is the search engine's state: off, starting, warming, ready, restarting, failed or stopped.
	Engine string
	// EngineMessage explains a restart or a failure.
	EngineMessage string
	// IndexState is the indexer's state (waiting, indexing, uptodate or failed) and IndexText the
	// sentence for it ("Indexing Downloads: 40 / 96 files").
	IndexState string
	IndexText  string
}

// Service is bound to the frontend; its exported methods become TypeScript functions.
type Service struct {
	settings    func() config.Config
	hotkeyError func() string
	engine      func() sidecar.Status
	index       func() indexing.Status
}

// New returns a service that reports the current settings; hotkeyError is asked for the current
// hotkey problem every time the UI asks (the hotkey is only bound once the app runs).
func New(settings func() config.Config, hotkeyError func() string, engine func() sidecar.Status, index func() indexing.Status) *Service {
	return &Service{settings: settings, hotkeyError: hotkeyError, engine: engine, index: index}
}

// Info returns the launcher's configuration and health.
func (s *Service) Info() Info {
	cfg := s.settings()
	info := Info{Hotkey: cfg.Hotkey, Roots: cfg.Roots, FileSearch: cfg.Search.Enabled, FilesInBar: cfg.Search.Enabled && cfg.Search.InLauncher,
		SelectionEffect: cfg.UI.SelectionEffect, GroupResults: cfg.Search.GroupResults}
	if s.hotkeyError != nil {
		info.HotkeyError = s.hotkeyError()
	}
	if s.engine != nil {
		st := s.engine()
		info.Engine, info.EngineMessage = string(st.State), st.Message
	}
	if s.index != nil {
		st := s.index()
		info.IndexState, info.IndexText = string(st.State), st.String()
	}
	if info.Engine == string(sidecar.StateOff) {
		// Nothing is waiting for an engine that is switched off; the page shows no index status.
		info.IndexState, info.IndexText = "off", ""
	}
	return info
}

// maxPageError is the most of a page error the log keeps: a stack can be long, and the log is one file for everything.
const maxPageError = 2000

// PageError writes a script error of a window to the log. The message the person sees says the details are in the log; without this
// they were only in a console nobody can open.
func (s *Service) PageError(message string) {
	if n := pageErrors.Add(1); n > maxPageErrors {
		if n == maxPageErrors+1 {
			log.Printf("page error: more than %d in this run; the rest are not written", maxPageErrors)
		}
		return
	}
	if runes := []rune(message); len(runes) > maxPageError {
		message = string(runes[:maxPageError]) + "..."
	}
	log.Printf("page error: %s", message)
}

// maxPageErrors is how many a run writes: a page that throws in a loop must not fill the log.
const maxPageErrors = 50

var pageErrors atomic.Int64
