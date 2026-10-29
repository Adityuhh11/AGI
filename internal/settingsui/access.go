package settingsui

import "time"

const (
	// probeFresh is how long what was found out about a folder is believed; the page asks every second or two.
	probeFresh = 5 * time.Second
	// probeWait is the longest a page waits for a folder to answer. A drive that is asleep, a share that is slow, or a Mac waiting for the person to answer its
	// question about the folder would otherwise hold the page; the answer is kept when it comes, and the page shows the last one meanwhile.
	probeWait = 400 * time.Millisecond
)

// probe is what is known about one folder.
type probe struct {
	at      time.Time
	blocked bool
	running bool
}

// isBlocked is blocked(root) for a page that asks often: it reads the folder at most once in probeFresh and never waits longer than probeWait for it.
func (s *Service) isBlocked(root string) bool {
	s.probeMu.Lock()
	if s.probes == nil {
		s.probes = map[string]*probe{}
	}
	p := s.probes[root]
	if p == nil {
		p = &probe{}
		s.probes[root] = p
	}
	if p.running || (!p.at.IsZero() && time.Since(p.at) < probeFresh) {
		defer s.probeMu.Unlock()
		return p.blocked
	}
	p.running = true
	s.probeMu.Unlock()

	done := make(chan struct{})
	go func() {
		blocked := s.blocked(root)
		s.probeMu.Lock()
		p.blocked, p.at, p.running = blocked, time.Now(), false
		s.probeMu.Unlock()
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(probeWait):
	}
	s.probeMu.Lock()
	defer s.probeMu.Unlock()
	return p.blocked
}
