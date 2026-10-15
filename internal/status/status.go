// Package status tells the UI about the launcher itself: what it is configured to do and, as
// later blocks add them, whether the search engine is ready and what is being indexed.
package status

// Info is the launcher's configuration as the UI shows it.
type Info struct {
	Hotkey string
	Roots  []string
}

// Service is bound to the frontend; its exported methods become TypeScript functions.
type Service struct {
	info Info
}

// New returns a service that reports info.
func New(info Info) *Service {
	return &Service{info: info}
}

// Info returns the launcher's configuration.
func (s *Service) Info() Info {
	return s.info
}
