// Package state remembers what the launcher has learned about its user: what was launched and when, and
// what was chosen after typing what. It is one small JSON file, written whole and replaced atomically;
// a few thousand entries at most, so a database would be more machinery than the job.
package state

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

// maxUses is how many launch times are kept per thing: frecency only needs the recent ones.
const maxUses = 20

// maxTyped is the longest typed prefix that is learned; longer text is a search, not a shortcut.
const maxTyped = 32

// file is the content of the state file.
type file struct {
	Version int                       `json:"version"`
	Uses    map[string][]int64        `json:"uses"`    // id -> unix milliseconds, oldest first
	Learned map[string]map[string]int `json:"learned"` // typed text -> id -> times chosen
}

// Store is the state, in memory and on disk. It is safe for concurrent use.
type Store struct {
	path string

	mu   sync.Mutex
	data file
}

// Open reads the state file at path, or starts empty if there is none. A file that cannot be understood
// is moved aside (so nothing is lost for good) and the store starts empty.
func Open(path string) (*Store, error) {
	s := &Store{path: path, data: empty()}
	raw, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return s, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read state: %w", err)
	}
	if err := json.Unmarshal(raw, &s.data); err != nil {
		aside := fmt.Sprintf("%s.corrupt-%d", path, time.Now().Unix())
		log.Printf("state: %s is damaged (%v); moved to %s and starting empty", filepath.Base(path), err, filepath.Base(aside))
		if err := os.Rename(path, aside); err != nil {
			return nil, fmt.Errorf("set aside the damaged state: %w", err)
		}
		s.data = empty()
	}
	if s.data.Uses == nil {
		s.data.Uses = map[string][]int64{}
	}
	if s.data.Learned == nil {
		s.data.Learned = map[string]map[string]int{}
	}
	return s, nil
}

func empty() file {
	return file{Version: 1, Uses: map[string][]int64{}, Learned: map[string]map[string]int{}}
}

// Uses are the times the thing was launched, oldest first.
func (s *Store) Uses(id string) []time.Time {
	s.mu.Lock()
	defer s.mu.Unlock()
	stamps := s.data.Uses[id]
	uses := make([]time.Time, len(stamps))
	for i, ms := range stamps {
		uses[i] = time.UnixMilli(ms)
	}
	return uses
}

// Learned is how many times the thing was chosen after typing exactly this (ignoring case and edges).
func (s *Store) Learned(typed, id string) int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.data.Learned[normalise(typed)][id]
}

// Record notes that the thing was launched at at, after typing typed (which may be empty), and saves.
func (s *Store) Record(id, typed string, at time.Time) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	uses := append(s.data.Uses[id], at.UnixMilli())
	if len(uses) > maxUses {
		uses = uses[len(uses)-maxUses:]
	}
	s.data.Uses[id] = uses
	if typed = normalise(typed); typed != "" && len(typed) <= maxTyped {
		if s.data.Learned[typed] == nil {
			s.data.Learned[typed] = map[string]int{}
		}
		s.data.Learned[typed][id]++
	}
	return s.save()
}

func normalise(typed string) string { return strings.ToLower(strings.TrimSpace(typed)) }

// save writes the whole file to a temporary one and moves it into place, so a crash never leaves half a file.
// The caller holds the lock.
func (s *Store) save() error {
	raw, err := json.Marshal(s.data)
	if err != nil {
		return fmt.Errorf("encode state: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(s.path), 0o755); err != nil {
		return fmt.Errorf("create state folder: %w", err)
	}
	temp := s.path + ".tmp"
	if err := os.WriteFile(temp, raw, 0o644); err != nil {
		return fmt.Errorf("write state: %w", err)
	}
	if err := os.Rename(temp, s.path); err != nil {
		return fmt.Errorf("replace state: %w", err)
	}
	return nil
}
