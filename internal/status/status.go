// Package status tells the UI about the launcher itself: what it is configured to do and, as
// later blocks add them, whether the search engine is ready and what is being indexed.
package status

import "agi/internal/sidecar"

// Info is the launcher's configuration and health as the UI shows it.
type Info struct {
	Hotkey string
	Roots  []string
	// HotkeyError is why the hotkey does not work (another app owns it), or empty.
	HotkeyError string
	// Engine is the search engine's state: starting, warming, ready, restarting, failed or stopped.
	Engine string
	// EngineMessage explains a restart or a failure.
	EngineMessage string
}

// Service is bound to the frontend; its exported methods become TypeScript functions.
type Service struct {
	info        Info
	hotkeyError func() string
	engine      func() sidecar.Status
}

// New returns a service that reports info; hotkeyError is asked for the current hotkey problem
// every time the UI asks (the hotkey is only bound once the app runs).
func New(info Info, hotkeyError func() string, engine func() sidecar.Status) *Service {
	return &Service{info: info, hotkeyError: hotkeyError, engine: engine}
}

// Info returns the launcher's configuration and health.
func (s *Service) Info() Info {
	info := s.info
	if s.hotkeyError != nil {
		info.HotkeyError = s.hotkeyError()
	}
	if s.engine != nil {
		st := s.engine()
		info.Engine, info.EngineMessage = string(st.State), st.Message
	}
	return info
}
