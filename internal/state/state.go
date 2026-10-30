// Package state remembers what the launcher has learned about its user: what was launched and when, and
// what was chosen after typing what, and the tags put on things. It is one small JSON file, written
// whole and replaced atomically; a few thousand entries at most, so a database would be more machinery
// than the job.
package state

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"omni/internal/atomicfile"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strings"
	"sync"
	"time"
	"unicode"
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
	Tags    map[string][]string       `json:"tags"`    // tag -> ids of what carries it, in the order tagged
	Pins    []string                  `json:"pins"`    // ids shown when the bar opens, in the order pinned
	Waived  map[string]int64          `json:"waived"`  // offer id -> unix milliseconds of "not now"
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
	if s.data.Tags == nil {
		s.data.Tags = map[string][]string{}
	}
	return s, nil
}

func empty() file {
	return file{Version: 1, Uses: map[string][]int64{}, Learned: map[string]map[string]int{}, Tags: map[string][]string{}}
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
	if err := atomicfile.Write(s.path, raw); err != nil {
		return fmt.Errorf("write state: %w", err)
	}
	return nil
}

// MaxPins is how many things can be pinned: the bar lists them all when it opens, so the list has to fit.
const MaxPins = 8

// Pins lists what is pinned, in the order it was pinned.
func (s *Store) Pins() []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return slices.Clone(s.data.Pins)
}

// SeedPins pins ids if nothing has ever been pinned (the file has no pins yet), and reports whether it did. It
// is offered once: someone who pinned things and then unpinned them all is not given the seed back.
func (s *Store) SeedPins(ids ...string) (bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.data.Pins != nil {
		return false, nil
	}
	s.data.Pins = append([]string{}, ids...)
	return true, s.save()
}

// TogglePin pins the thing, or unpins it if it is pinned already, and saves. It reports whether the
// thing is pinned now. Pinning more than MaxPins things is refused.
func (s *Store) TogglePin(id string) (bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if i := slices.Index(s.data.Pins, id); i >= 0 {
		s.data.Pins = slices.Delete(s.data.Pins, i, i+1)
		return false, s.save()
	}
	if len(s.data.Pins) >= MaxPins {
		return false, fmt.Errorf("%d things are pinned, which is the most; unpin one first", MaxPins)
	}
	s.data.Pins = append(s.data.Pins, id)
	return true, s.save()
}

// maxTag is the longest tag name.
const maxTag = 32

// NormaliseTag turns what was typed into a tag name: no leading #, lower case, spaces and runs of
// punctuation become single dashes. It reports false when nothing usable is left.
func NormaliseTag(typed string) (string, bool) {
	var out []rune
	dash := false
	for _, r := range strings.ToLower(strings.TrimPrefix(strings.TrimSpace(typed), "#")) {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			if dash && len(out) > 0 {
				out = append(out, '-')
			}
			out, dash = append(out, r), false
		} else {
			dash = true
		}
	}
	if len(out) == 0 || len(out) > maxTag {
		return "", false
	}
	return string(out), true
}

// ToggleTag puts the tag on the thing, or takes it off if it is already there, and saves. It reports
// whether the thing carries the tag now.
func (s *Store) ToggleTag(id, tag string) (bool, error) {
	tag, ok := NormaliseTag(tag)
	if !ok {
		return false, fmt.Errorf("%q is not a usable tag", tag)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	ids := s.data.Tags[tag]
	for i, have := range ids {
		if have == id {
			ids = append(ids[:i:i], ids[i+1:]...)
			if len(ids) == 0 {
				delete(s.data.Tags, tag)
			} else {
				s.data.Tags[tag] = ids
			}
			return false, s.save()
		}
	}
	s.data.Tags[tag] = append(ids, id)
	return true, s.save()
}

// Tagged lists what carries the tag, in the order it was tagged.
func (s *Store) Tagged(tag string) []string {
	tag, _ = NormaliseTag(tag)
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]string(nil), s.data.Tags[tag]...)
}

// TagsOf lists the tags a thing carries, alphabetically.
func (s *Store) TagsOf(id string) []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	var tags []string
	for tag, ids := range s.data.Tags {
		if slices.Contains(ids, id) {
			tags = append(tags, tag)
		}
	}
	sort.Strings(tags)
	return tags
}

// TagCount is a tag and how many things carry it.
type TagCount struct {
	Name  string
	Count int
}

// Tags lists every tag, the most used first.
func (s *Store) Tags() []TagCount {
	s.mu.Lock()
	defer s.mu.Unlock()
	tags := make([]TagCount, 0, len(s.data.Tags))
	for name, ids := range s.data.Tags {
		tags = append(tags, TagCount{Name: name, Count: len(ids)})
	}
	sort.Slice(tags, func(a, b int) bool {
		if tags[a].Count != tags[b].Count {
			return tags[a].Count > tags[b].Count
		}
		return tags[a].Name < tags[b].Name
	})
	return tags
}

// Waived is when the person said "not now" to an offer, or the zero time if they never did.
func (s *Store) Waived(id string) time.Time {
	s.mu.Lock()
	defer s.mu.Unlock()
	if ms, ok := s.data.Waived[id]; ok {
		return time.UnixMilli(ms)
	}
	return time.Time{}
}

// Waive records "not now" to an offer, and saves.
func (s *Store) Waive(id string, at time.Time) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.data.Waived == nil {
		s.data.Waived = map[string]int64{}
	}
	s.data.Waived[id] = at.UnixMilli()
	return s.save()
}
