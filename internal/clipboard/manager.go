package clipboard

import (
	"context"
	"fmt"
	"log"
	"time"
)

// pollEvery is how often the clipboard's change counter is looked at.
const pollEvery = 250 * time.Millisecond

// Manager is the clipboard history as the rest of the launcher uses it: switched on and off in the settings,
// searched from the bar, pasted from.
type Manager struct {
	store   *Store
	watcher *Watcher
	paster  Paster
	enabled func() bool
	now     func() time.Time
}

// NewManager joins a history, the watcher that fills it and the paster that types from it. enabled says whether
// the history is switched on.
func NewManager(store *Store, watcher *Watcher, paster Paster, enabled func() bool) *Manager {
	return &Manager{store: store, watcher: watcher, paster: paster, enabled: enabled, now: time.Now}
}

// Enabled says whether the history is switched on.
func (m *Manager) Enabled() bool { return m.enabled() }

// Run keeps the history up to date until ctx ends.
func (m *Manager) Run(ctx context.Context) { m.watcher.Run(ctx, pollEvery) }

// Poll looks at the clipboard now, for when the bar opens (a copy a moment ago may not have been seen yet).
func (m *Manager) Poll() { m.watcher.Poll() }

// Search lists up to limit entries, newest first, that contain every word.
func (m *Manager) Search(words string, limit int) []Entry { return m.store.Search(words, limit) }

// Paste types an entry into the program the bar was opened from, and moves it to the top of the history.
func (m *Manager) Paste(id int64) error {
	entry, err := m.entry(id)
	if err != nil {
		return err
	}
	m.touch(id)
	return m.paster.Paste(entry.Text)
}

// Copy puts an entry on the clipboard and gives the keyboard back, without typing.
func (m *Manager) Copy(id int64) error {
	entry, err := m.entry(id)
	if err != nil {
		return err
	}
	m.touch(id)
	return m.paster.Copy(entry.Text)
}

// Delete forgets one entry.
func (m *Manager) Delete(id int64) error { return m.store.Delete(id) }

// Clear forgets everything.
func (m *Manager) Clear() error { return m.store.Clear() }

// Len is how many entries are kept.
func (m *Manager) Len() int { return m.store.Len() }

// Path is the file the history is kept in.
func (m *Manager) Path() string { return m.store.Path() }

func (m *Manager) entry(id int64) (Entry, error) {
	entry, ok := m.store.Get(id)
	if !ok {
		return Entry{}, fmt.Errorf("that entry is not in the history any more")
	}
	return entry, nil
}

func (m *Manager) touch(id int64) {
	if err := m.store.Touch(id, m.now()); err != nil {
		log.Printf("clipboard: could not move the entry to the top: %v", err)
	}
}
