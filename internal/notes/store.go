// Package notes keeps quick markdown notes as plain files, one per note, in a folder of the user's own: they can be
// read, copied and backed up without this program. Nothing here is indexed by the search engine or leaves the machine.
package notes

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"
	"unicode/utf8"
)

// Note is a note as the windows list it, and with its Text when it is fetched alone.
type Note struct {
	// ID is the file's name without ".md": the time it was made, 20261026-231500 (a -2, -3 ... if two share a second).
	ID string
	// Title is the first line of the note without its markdown marks, or "Untitled note".
	Title string
	// Preview is the start of what follows the title, on one line.
	Preview string
	// Modified is when the note was last saved, in milliseconds since 1970 (what the page's Date takes).
	Modified int64
	// Text is the whole note; List and Search leave it empty.
	Text string
}

const (
	untitled      = "Untitled note"
	titleLength   = 80
	previewLength = 110
	extension     = ".md"
)

// idPattern is every id the store makes. Anything else is refused, so an id from the page can never name a file outside
// the folder.
var idPattern = regexp.MustCompile(`^\d{8}-\d{6}(-\d+)?$`)

// Store is a folder of notes.
type Store struct {
	dir string
	now func() time.Time
	mu  sync.Mutex
}

// Open returns the store of the folder dir, creating it if it is missing.
func Open(dir string) (*Store, error) {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, fmt.Errorf("create the notes folder: %w", err)
	}
	return &Store{dir: dir, now: time.Now}, nil
}

// Dir is the folder the notes are in.
func (s *Store) Dir() string { return s.dir }

func (s *Store) path(id string) (string, error) {
	if !idPattern.MatchString(id) {
		return "", fmt.Errorf("%q is not a note", id)
	}
	return filepath.Join(s.dir, id+extension), nil
}

// List returns every note, the most recently changed first.
func (s *Store) List() ([]Note, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	entries, err := os.ReadDir(s.dir)
	if err != nil {
		return nil, fmt.Errorf("read the notes folder: %w", err)
	}
	var out []Note
	for _, e := range entries {
		id, ok := strings.CutSuffix(e.Name(), extension)
		if !ok || e.IsDir() || !idPattern.MatchString(id) {
			continue
		}
		n, err := s.read(id)
		if err != nil {
			continue // a file that vanished or cannot be read is not a note to list
		}
		n.Text = ""
		out = append(out, n)
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].Modified > out[j].Modified })
	return out, nil
}

// Get returns a note with its text.
func (s *Store) Get(id string) (Note, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.read(id)
}

func (s *Store) read(id string) (Note, error) {
	p, err := s.path(id)
	if err != nil {
		return Note{}, err
	}
	data, err := os.ReadFile(p)
	if err != nil {
		return Note{}, fmt.Errorf("read the note: %w", err)
	}
	info, err := os.Stat(p)
	if err != nil {
		return Note{}, fmt.Errorf("read the note: %w", err)
	}
	text := string(data)
	title, preview := summarise(text)
	return Note{ID: id, Title: title, Preview: preview, Modified: info.ModTime().UnixMilli(), Text: text}, nil
}

// Save writes a note. With an empty id it makes a new one (and makes nothing at all for a text that is only
// whitespace). The file is written beside its name and renamed over it, so a crash never leaves half a note.
func (s *Store) Save(id, text string) (Note, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if id == "" {
		if strings.TrimSpace(text) == "" {
			return Note{}, nil
		}
		id = s.newID()
	}
	p, err := s.path(id)
	if err != nil {
		return Note{}, err
	}
	temp := p + ".tmp"
	if err := os.WriteFile(temp, []byte(text), 0o644); err != nil {
		return Note{}, fmt.Errorf("write the note: %w", err)
	}
	if err := os.Rename(temp, p); err != nil {
		_ = os.Remove(temp)
		return Note{}, fmt.Errorf("save the note: %w", err)
	}
	return s.read(id)
}

// newID is the current time as an id that no file has yet.
func (s *Store) newID() string {
	base := s.now().Format("20060102-150405")
	id := base
	for n := 2; ; n++ {
		if _, err := os.Stat(filepath.Join(s.dir, id+extension)); errors.Is(err, os.ErrNotExist) {
			return id
		}
		id = fmt.Sprintf("%s-%d", base, n)
	}
}

// Delete removes a note. A note that is already gone is not an error.
func (s *Store) Delete(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	p, err := s.path(id)
	if err != nil {
		return err
	}
	if err := os.Remove(p); err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("delete the note: %w", err)
	}
	return nil
}

// Search returns the notes that contain every word typed (case does not matter), newest first, at most limit of them
// (all of them if limit is 0). Nothing typed returns every note.
func (s *Store) Search(words string, limit int) ([]Note, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	entries, err := os.ReadDir(s.dir)
	if err != nil {
		return nil, fmt.Errorf("read the notes folder: %w", err)
	}
	want := strings.Fields(strings.ToLower(words))
	var out []Note
	for _, e := range entries {
		id, ok := strings.CutSuffix(e.Name(), extension)
		if !ok || e.IsDir() || !idPattern.MatchString(id) {
			continue
		}
		n, err := s.read(id)
		if err != nil {
			continue
		}
		lower := strings.ToLower(n.Text)
		match := true
		for _, w := range want {
			match = match && strings.Contains(lower, w)
		}
		if match {
			n.Text = ""
			out = append(out, n)
		}
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].Modified > out[j].Modified })
	if limit > 0 && len(out) > limit {
		out = out[:limit]
	}
	return out, nil
}

// summarise is a note's title and the start of its body, each on one line.
func summarise(text string) (title, preview string) {
	var body []string
	for _, line := range strings.Split(text, "\n") {
		line = strings.TrimSpace(strings.TrimRight(line, "\r"))
		if line == "" {
			continue
		}
		if title == "" {
			title = clean(line)
			if title == "" {
				title = untitled
			}
			continue
		}
		body = append(body, clean(line))
	}
	if title == "" {
		title = untitled
	}
	return cut(title, titleLength), cut(strings.Join(body, " "), previewLength)
}

// clean takes the markdown marks off the front of a line: "## Title", "- [ ] task", "> quote".
func clean(line string) string {
	line = strings.TrimLeft(line, "#>*-+ \t")
	line = strings.TrimPrefix(line, "[ ] ")
	line = strings.TrimPrefix(line, "[x] ")
	return strings.TrimSpace(line)
}

func cut(text string, n int) string {
	if utf8.RuneCountInString(text) <= n {
		return text
	}
	return strings.TrimSpace(string([]rune(text)[:n])) + "…"
}
