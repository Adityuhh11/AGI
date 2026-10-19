package clipboard

import (
	"context"
	"errors"
	"log"
	"strings"
	"sync"
	"time"
	"unicode/utf8"
)

// MaxText is the longest text kept, in characters. Longer text is skipped, never cut: half of a copied
// document is worse than none.
const MaxText = 100_000

// ErrTooLong is what a Source answers when what is on the clipboard is longer than MaxText.
var ErrTooLong = errors.New("the text is too long to keep")

// DefaultIgnore are programs whose copies are never kept, beside the ones the user adds: password managers.
// (Most of them also mark what they copy as private, which is honoured separately.)
var DefaultIgnore = []string{"1Password", "Bitwarden", "KeePass", "KeePassXC", "Dashlane", "Enpass", "NordPass", "RoboForm"}

// Clip is what is on the clipboard: the text, the program that put it there and whether that program asked
// for it to be kept out of any history.
type Clip struct {
	Text    string
	Owner   string
	Private bool
}

// Source is the clipboard as the watcher sees it. WindowsSource is the real one.
type Source interface {
	// Sequence changes whenever anything is copied.
	Sequence() uint32
	// Read returns what is on the clipboard now; something that is not text has no Text.
	Read() (Clip, error)
}

// Watcher keeps the history in step with the clipboard.
type Watcher struct {
	src     Source
	store   *Store
	enabled func() bool
	ignore  func() []string
	self    string
	now     func() time.Time

	mu     sync.Mutex
	last   uint32 // the sequence number already dealt with
	failed uint32 // the sequence number whose failure was already logged
}

// NewWatcher returns a Watcher that starts from what is on the clipboard now: it is not part of the history.
// enabled says whether history is switched on, ignore lists the programs to leave out beside DefaultIgnore,
// and self is the name of this program, whose own copies (a paste, "copy path") are not history either.
func NewWatcher(src Source, store *Store, enabled func() bool, ignore func() []string, self string) *Watcher {
	return &Watcher{src: src, store: store, enabled: enabled, ignore: ignore, self: self, now: time.Now, last: src.Sequence()}
}

// Poll looks at the clipboard once and keeps what is new. It is cheap when nothing changed: one counter is read.
func (w *Watcher) Poll() {
	w.mu.Lock()
	defer w.mu.Unlock()
	seq := w.src.Sequence()
	if seq == w.last {
		return
	}
	if !w.enabled() {
		w.last = seq // what is copied while history is off is not kept, and is not picked up when it is switched on
		return
	}
	clip, err := w.src.Read()
	switch {
	case errors.Is(err, ErrTooLong):
		w.last = seq
		log.Printf("clipboard: a text of more than %d characters was not kept", MaxText)
		return
	case err != nil:
		if w.failed != seq {
			w.failed = seq
			log.Printf("clipboard: could not read what was copied (it is tried again): %v", err)
		}
		return // w.last stays, so the next poll tries again
	}
	w.last = seq
	if w.skip(clip) {
		return
	}
	if err := w.store.Add(clip.Text, clip.Owner, w.now()); err != nil {
		log.Printf("clipboard: could not keep what was copied: %v", err)
	}
}

// skip says whether a copy stays out of the history: private, from a program on the list, from this
// program, not text, or nothing but spaces.
func (w *Watcher) skip(clip Clip) bool {
	if clip.Private || strings.TrimSpace(clip.Text) == "" || utf8.RuneCountInString(clip.Text) > MaxText {
		return true
	}
	if clip.Owner == "" {
		return false
	}
	if strings.EqualFold(clip.Owner, w.self) {
		return true
	}
	for _, name := range append(append([]string(nil), DefaultIgnore...), w.ignore()...) {
		if strings.EqualFold(strings.TrimSuffix(strings.TrimSpace(name), ".exe"), clip.Owner) {
			return true
		}
	}
	return false
}

// Run polls until ctx ends, every interval.
func (w *Watcher) Run(ctx context.Context, interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			w.Poll()
		}
	}
}
