package notes

// Service is the store as the notes window uses it; its exported methods become TypeScript functions.
type Service struct {
	store    *Store
	setOnTop func(bool)
}

// NewService returns the service; setOnTop keeps the notes window above other windows, or lets it sink (it may be nil).
func NewService(store *Store, setOnTop func(bool)) *Service {
	return &Service{store: store, setOnTop: setOnTop}
}

// List returns the notes that contain every word of query, newest change first; an empty query returns all of them.
func (s *Service) List(query string) ([]Note, error) { return s.store.Search(query, 0) }

// Get returns a note with its text.
func (s *Service) Get(id string) (Note, error) { return s.store.Get(id) }

// Save writes a note, or makes a new one when id is empty, and returns it as saved.
func (s *Service) Save(id, text string) (Note, error) { return s.store.Save(id, text) }

// Delete removes a note.
func (s *Service) Delete(id string) error { return s.store.Delete(id) }

// SetOnTop keeps the notes window above other windows (on) or lets other windows cover it (off).
func (s *Service) SetOnTop(on bool) {
	if s.setOnTop != nil {
		s.setOnTop(on)
	}
}
