// Package settings holds the launcher's configuration while it runs. Everything that used to read
// config.toml once at start asks the store instead, so a change made in the settings window takes
// effect at once, and a change that does not validate never reaches the file or the program.
package settings

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"reflect"
	"sync"

	"agi/internal/config"
)

// Store owns the configuration and the file it lives in.
type Store struct {
	path string

	update sync.Mutex // one Update at a time, so subscribers see changes in order
	mu     sync.RWMutex
	cfg    config.Config
	subs   []func(old, updated config.Config)
}

// Open loads the settings file at path (creating it with the defaults if it is missing). A file that cannot be read
// (a typo, or settings from another version of the program) is set aside as config.unreadable.toml and the defaults
// are used: a launcher that does not start at all is worse than one that has lost its settings, and the old file is kept.
func Open(path string) (*Store, error) {
	cfg, err := config.Load(path)
	if err != nil {
		aside := filepath.Join(filepath.Dir(path), "config.unreadable.toml")
		_ = os.Remove(aside)
		if renameErr := os.Rename(path, aside); renameErr != nil {
			return nil, fmt.Errorf("%w (and it could not be set aside: %v)", err, renameErr)
		}
		log.Printf("settings: %v; starting with the defaults, the old file is kept as %s", err, aside)
		if cfg, err = config.Load(path); err != nil {
			return nil, err
		}
	}
	return &Store{path: path, cfg: cfg}, nil
}

// Path is the settings file.
func (s *Store) Path() string { return s.path }

// Get returns a copy of the current settings; changing it changes nothing.
func (s *Store) Get() config.Config {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.cfg.Clone()
}

// Subscribe registers fn to be called, in the order changes were made, with the settings before and
// after each successful Update. fn runs in the goroutine that called Update and must not call
// Update itself.
func (s *Store) Subscribe(fn func(old, updated config.Config)) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.subs = append(s.subs, fn)
}

// Update applies change to a copy of the settings, checks the result, writes it to the file and
// only then makes it current and tells the subscribers. If the result is invalid or cannot be
// written, nothing changes and the error says why. A change that alters nothing writes and
// announces nothing.
func (s *Store) Update(change func(*config.Config)) error {
	s.update.Lock()
	defer s.update.Unlock()

	old := s.Get()
	updated := old.Clone()
	change(&updated)
	if err := updated.Validate(); err != nil {
		return fmt.Errorf("settings: %w", err)
	}
	if sameSettings(old, updated) {
		return nil
	}
	if err := updated.Save(s.path); err != nil {
		return err
	}
	s.mu.Lock()
	s.cfg = updated.Clone()
	subs := append([]func(old, updated config.Config){}, s.subs...)
	s.mu.Unlock()
	for _, fn := range subs {
		fn(old.Clone(), updated.Clone())
	}
	return nil
}

// sameSettings compares two configurations by value (they hold slices, so == is not available).
func sameSettings(a, b config.Config) bool {
	return reflect.DeepEqual(a, b)
}
