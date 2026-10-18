// Package status tells the UI about the launcher itself: what it is configured to do and, as
// later blocks add them, whether the search engine is ready and what is being indexed.
package status

import (
	"agi/internal/config"
	"agi/internal/indexing"
	"agi/internal/sidecar"
)

// Info is the launcher's configuration and health as the UI shows it.
type Info struct {
	Hotkey string
	Roots  []string
	// FileSearch says whether file search is switched on.
	FileSearch bool
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
	info := Info{Hotkey: cfg.Hotkey, Roots: cfg.Roots, FileSearch: cfg.Search.Enabled}
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
	return info
}
