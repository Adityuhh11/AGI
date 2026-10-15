package panel

// Service is what the page can ask of the panel; its exported methods become TypeScript functions.
type Service struct{ p *Panel }

// NewService exposes p to the frontend.
func NewService(p *Panel) *Service { return &Service{p: p} }

// Hide closes the panel (the Escape key) and returns the keyboard to the previous app.
func (s *Service) Hide() { s.p.Hide() }

// Resize fits the panel to its content: heightDIP is the page's content height in CSS pixels.
func (s *Service) Resize(heightDIP int) { s.p.Resize(heightDIP) }
