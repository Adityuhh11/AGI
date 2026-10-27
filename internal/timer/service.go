package timer

// Service is the timer as its window uses it; its exported methods become TypeScript functions. The window asks for the
// state a few times a second and shows it; everything else is a button.
type Service struct{ t *Timer }

// NewService returns the service of t.
func NewService(t *Timer) *Service { return &Service{t: t} }

// State is the timer now.
func (s *Service) State() State { return s.t.State() }

// Pause stops the clock of the running period.
func (s *Service) Pause() { s.t.Pause() }

// Resume continues a paused period.
func (s *Service) Resume() { s.t.Resume() }

// Stop ends the timer and closes its window.
func (s *Service) Stop() { s.t.Stop() }

// Skip starts the next period of a pomodoro; on a plain timer it ends it.
func (s *Service) Skip() { s.t.Skip() }
