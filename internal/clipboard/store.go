package clipboard

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// The limits of the history. A thousand copies of a few lines each are about half a megabyte; the byte
// limit is for the day someone copies a lot of large documents.
const (
	MaxEntries = 500
	MaxAge     = 30 * 24 * time.Hour
	MaxBytes   = 20 << 20
)

// Entry is one thing that was copied.
type Entry struct {
	// ID names the entry for as long as it exists; it follows the time it was first copied.
	ID int64 `json:"id"`
	// Text is what was copied.
	Text string `json:"text"`
	// Source is the program it was copied from ("Code", "chrome"), or empty if that was not known.
	Source string `json:"source,omitempty"`
	// At is when it was last copied.
	At time.Time `json:"at"`
}

type file struct {
	Version int     `json:"version"`
	Entries []Entry `json:"entries"` // oldest first
}

// Store is the history, in memory and on disk. It is safe for concurrent use. The whole file is rewritten on
// every change, which takes 1.5 to 12 ms for histories from half a megabyte to 14 (docs/eval and the spec).
type Store struct {
	path string

	mu      sync.Mutex
	entries []Entry  // oldest first
	lower   []string // the same texts in lower case, so a search does not lower them again
}

// OpenStore reads the history at path, or starts empty if there is none. A file that cannot be understood
// is moved aside, and the history starts empty.
func OpenStore(path string) (*Store, error) {
	s := &Store{path: path}
	raw, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return s, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read the clipboard history: %w", err)
	}
	var f file
	if err := json.Unmarshal(raw, &f); err != nil {
		aside := fmt.Sprintf("%s.corrupt-%d", path, time.Now().Unix())
		log.Printf("clipboard: %s is damaged (%v); moved to %s and starting empty", filepath.Base(path), err, filepath.Base(aside))
		if err := os.Rename(path, aside); err != nil {
			return nil, fmt.Errorf("set aside the damaged clipboard history: %w", err)
		}
		return s, nil
	}
	s.entries = f.Entries
	s.trim(time.Now())
	s.reindex()
	return s, nil
}

func (s *Store) reindex() {
	s.lower = make([]string, len(s.entries))
	for i, e := range s.entries {
		s.lower[i] = strings.ToLower(e.Text)
	}
}

// trim drops the oldest entries that are too old, or beyond the count or size limits. The caller holds the lock.
func (s *Store) trim(now time.Time) {
	total := 0
	for _, e := range s.entries {
		total += len(e.Text)
	}
	drop := 0
	for drop < len(s.entries) {
		old := now.Sub(s.entries[drop].At) > MaxAge
		if !old && len(s.entries)-drop <= MaxEntries && total <= MaxBytes {
			break
		}
		total -= len(s.entries[drop].Text)
		drop++
	}
	if drop > 0 {
		s.entries = append([]Entry(nil), s.entries[drop:]...)
	}
}

// Add keeps text, copied from source at at. The same text kept already moves to the newest place instead of
// being added again. Text that is only spaces is not kept.
func (s *Store) Add(text, source string, at time.Time) error {
	if strings.TrimSpace(text) == "" {
		return nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	for i, e := range s.entries {
		if e.Text == text {
			s.entries = append(s.entries[:i:i], s.entries[i+1:]...)
			e.At = at
			if source != "" {
				e.Source = source
			}
			s.entries = append(s.entries, e)
			s.reindex()
			return s.save()
		}
	}
	id := at.UnixMilli()
	if n := len(s.entries); n > 0 && s.entries[n-1].ID >= id {
		id = s.entries[n-1].ID + 1
	}
	s.entries = append(s.entries, Entry{ID: id, Text: text, Source: source, At: at})
	s.trim(at)
	s.reindex()
	return s.save()
}

// Touch moves an entry to the newest place, as if it had just been copied.
func (s *Store) Touch(id int64, at time.Time) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for i, e := range s.entries {
		if e.ID == id {
			s.entries = append(s.entries[:i:i], s.entries[i+1:]...)
			e.At = at
			s.entries = append(s.entries, e)
			s.reindex()
			return s.save()
		}
	}
	return nil
}

// Search lists up to limit entries, newest first, that contain every word of query (in any case, anywhere).
// An empty query lists the newest.
func (s *Store) Search(query string, limit int) []Entry {
	words := strings.Fields(strings.ToLower(query))
	s.mu.Lock()
	defer s.mu.Unlock()
	var found []Entry
scan:
	for i := len(s.entries) - 1; i >= 0 && len(found) < limit; i-- {
		for _, word := range words {
			if !strings.Contains(s.lower[i], word) {
				continue scan
			}
		}
		found = append(found, s.entries[i])
	}
	return found
}

// Get returns the entry with this id.
func (s *Store) Get(id int64) (Entry, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, e := range s.entries {
		if e.ID == id {
			return e, true
		}
	}
	return Entry{}, false
}

// Delete forgets one entry.
func (s *Store) Delete(id int64) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for i, e := range s.entries {
		if e.ID == id {
			s.entries = append(s.entries[:i:i], s.entries[i+1:]...)
			s.reindex()
			return s.save()
		}
	}
	return nil
}

// Clear forgets everything.
func (s *Store) Clear() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.entries, s.lower = nil, nil
	return s.save()
}

// Len is how many entries are kept.
func (s *Store) Len() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.entries)
}

// Path is the file the history is kept in.
func (s *Store) Path() string { return s.path }

// save writes the whole file to a temporary one and moves it into place, so a crash never leaves half a file.
// The caller holds the lock.
func (s *Store) save() error {
	raw, err := json.Marshal(file{Version: 1, Entries: s.entries})
	if err != nil {
		return fmt.Errorf("encode the clipboard history: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(s.path), 0o755); err != nil {
		return fmt.Errorf("create the clipboard history folder: %w", err)
	}
	temp := s.path + ".tmp"
	if err := os.WriteFile(temp, raw, 0o600); err != nil {
		return fmt.Errorf("write the clipboard history: %w", err)
	}
	if err := os.Rename(temp, s.path); err != nil {
		return fmt.Errorf("replace the clipboard history: %w", err)
	}
	return nil
}
