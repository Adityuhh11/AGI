// Package chat is Omni AI's conversations: what is said, what the model is shown of it, and the service the chat window
// calls. The model itself runs in internal/llm; this package talks to it over its OpenAI-style HTTP API.
package chat

import (
	"encoding/json"
	"errors"
	"fmt"
	"omni/internal/atomicfile"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"
	"unicode/utf8"
)

// Message is one thing said. The json names are the file's and the page's.
type Message struct {
	Role string `json:"role"` // "user" or "assistant"
	Text string `json:"text"`
	// Thinking is the reasoning that came before an answer, when thinking was on; it is shown, never sent back to the model.
	Thinking   string `json:"thinking,omitempty"`
	ThinkingMS int64  `json:"thinking_ms,omitempty"`
	// Tokens is what the message takes of the model's context (its text and the template around it).
	Tokens int `json:"tokens"`
	// Stopped marks an answer the person cut short.
	Stopped bool `json:"stopped,omitempty"`
	// Sources are the web pages the answer drew on, when web search was on. What the model read is not kept, only where.
	Sources []Source `json:"sources,omitempty"`
}

// Conversation is a chat as it is kept in a file.
type Conversation struct {
	ID       string    `json:"id"`
	Title    string    `json:"title"`
	Created  int64     `json:"created"` // milliseconds since 1970, what the page's Date takes
	Modified int64     `json:"modified"`
	Messages []Message `json:"messages"`
}

// Summary is a conversation as the history lists it.
type Summary struct {
	ID       string `json:"id"`
	Title    string `json:"title"`
	Modified int64  `json:"modified"`
	Messages int    `json:"messages"`
}

const (
	extension   = ".json"
	titleLength = 60
	untitled    = "New chat"
)

// idPattern is every id the store makes (the time it was made, with a -2, -3 ... if two share a second). Anything else is
// refused, so an id from the page can never name a file outside the folder.
var idPattern = regexp.MustCompile(`^\d{8}-\d{6}(-\d+)?$`)

// Store is a folder of conversations, one JSON file each, in a place the person can open: like notes, they are the person's
// own files.
type Store struct {
	dir string
	now func() time.Time
	mu  sync.Mutex // makes ids unique and writes and deletes one at a time; reading takes no lock (a save is an atomic rename)
}

// Open returns the store of the folder dir, creating it if it is missing.
func Open(dir string) (*Store, error) {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, fmt.Errorf("create the chats folder: %w", err)
	}
	return &Store{dir: dir, now: time.Now}, nil
}

// Dir is the folder the conversations are in.
func (s *Store) Dir() string { return s.dir }

func (s *Store) path(id string) (string, error) {
	if !idPattern.MatchString(id) {
		return "", fmt.Errorf("%q is not a conversation", id)
	}
	return filepath.Join(s.dir, id+extension), nil
}

// Get returns a conversation with its messages.
func (s *Store) Get(id string) (Conversation, error) {
	p, err := s.path(id)
	if err != nil {
		return Conversation{}, err
	}
	data, err := os.ReadFile(p)
	if err != nil {
		return Conversation{}, fmt.Errorf("read the conversation: %w", err)
	}
	var c Conversation
	if err := json.Unmarshal(data, &c); err != nil {
		return Conversation{}, fmt.Errorf("read %s: %w", filepath.Base(p), err)
	}
	c.ID = id
	return c, nil
}

// Save writes a conversation. With an empty ID it gets a new one, and with an empty title the first words of the first
// question. The file is written beside its name and renamed over it, so a crash never leaves half a conversation.
func (s *Store) Save(c *Conversation) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	now := s.now().UnixMilli()
	if c.ID == "" {
		c.ID = s.newID()
		c.Created = now
	}
	c.Modified = now
	if c.Title == "" {
		c.Title = TitleOf(c.Messages)
	}
	p, err := s.path(c.ID)
	if err != nil {
		return err
	}
	body, err := json.Marshal(c)
	if err != nil {
		return fmt.Errorf("encode the conversation: %w", err)
	}
	if err := atomicfile.Write(p, body); err != nil {
		return fmt.Errorf("save the conversation: %w", err)
	}
	return nil
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

// TitleOf is the first line of the first question, cut to fit a list.
func TitleOf(messages []Message) string {
	for _, m := range messages {
		if m.Role != "user" {
			continue
		}
		line, _, _ := strings.Cut(strings.TrimSpace(m.Text), "\n")
		line = strings.TrimSpace(line)
		if line == "" {
			break
		}
		if utf8.RuneCountInString(line) > titleLength {
			line = string([]rune(line)[:titleLength-1]) + "…"
		}
		return line
	}
	return untitled
}

// Search returns the conversations whose title or text contain every word of words, the most recently changed first; no words
// lists all of them. limit 0 is no limit.
func (s *Store) Search(words string, limit int) ([]Summary, error) {
	entries, err := os.ReadDir(s.dir)
	if err != nil {
		return nil, fmt.Errorf("read the chats folder: %w", err)
	}
	terms := strings.Fields(strings.ToLower(words))
	var out []Summary
	for _, e := range entries {
		id, ok := strings.CutSuffix(e.Name(), extension)
		if !ok || e.IsDir() || !idPattern.MatchString(id) {
			continue
		}
		c, err := s.Get(id)
		if err != nil {
			continue // a file that vanished or cannot be read is not a conversation to list
		}
		if matches(c, terms) {
			out = append(out, Summary{ID: id, Title: c.Title, Modified: c.Modified, Messages: len(c.Messages)})
		}
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].Modified > out[j].Modified })
	if limit > 0 && len(out) > limit {
		out = out[:limit]
	}
	return out, nil
}

func matches(c Conversation, terms []string) bool {
	if len(terms) == 0 {
		return true
	}
	var all strings.Builder
	all.WriteString(strings.ToLower(c.Title))
	for _, m := range c.Messages {
		all.WriteByte('\n')
		all.WriteString(strings.ToLower(m.Text))
	}
	text := all.String()
	for _, t := range terms {
		if !strings.Contains(text, t) {
			return false
		}
	}
	return true
}

// Delete removes a conversation. One that is already gone is not an error.
func (s *Store) Delete(id string) error {
	p, err := s.path(id)
	if err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := os.Remove(p); err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("delete the conversation: %w", err)
	}
	return nil
}

// Clear removes every conversation.
func (s *Store) Clear() error { return s.removeWhere(func(time.Time) bool { return true }) }

// Expire removes the conversations not changed for days days; 0 removes nothing.
func (s *Store) Expire(days int) error {
	if days <= 0 {
		return nil
	}
	cutoff := s.now().AddDate(0, 0, -days)
	return s.removeWhere(func(modified time.Time) bool { return modified.Before(cutoff) })
}

func (s *Store) removeWhere(doomed func(modified time.Time) bool) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	entries, err := os.ReadDir(s.dir)
	if err != nil {
		return fmt.Errorf("read the chats folder: %w", err)
	}
	var failed error
	for _, e := range entries {
		id, ok := strings.CutSuffix(e.Name(), extension)
		if !ok || e.IsDir() || !idPattern.MatchString(id) {
			continue
		}
		info, err := e.Info()
		if err != nil || !doomed(info.ModTime()) {
			continue
		}
		if err := os.Remove(filepath.Join(s.dir, e.Name())); err != nil && !errors.Is(err, os.ErrNotExist) {
			failed = fmt.Errorf("delete %s: %w", e.Name(), err)
		}
	}
	return failed
}
