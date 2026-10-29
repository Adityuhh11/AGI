// Package hotkey holds one extra global hotkey that can be moved while the program runs.
package hotkey

import (
	"fmt"
	"log"
	"sync"
)

// Keys registers global hotkeys; Wails' GlobalShortcut satisfies it.
type Keys interface {
	Register(accelerator string, callback func()) error
	Unregister(accelerator string) error
}

// Slot is one hotkey (the chat's) that does one thing. Like the bar's own, a new combination is registered before the old one
// is dropped, so one that cannot be used leaves the working one in place.
type Slot struct {
	keys Keys

	mu      sync.Mutex
	current string
	errMsg  string
}

// NewSlot returns a Slot with no hotkey.
func NewSlot(keys Keys) *Slot { return &Slot{keys: keys} }

// Rebind moves the hotkey to accelerator, where it runs fn; an empty accelerator turns it off.
func (s *Slot) Rebind(accelerator string, fn func()) error {
	s.mu.Lock()
	old := s.current
	s.mu.Unlock()
	if accelerator == old {
		return nil
	}
	if accelerator != "" {
		if err := s.keys.Register(accelerator, fn); err != nil {
			return fmt.Errorf("the hotkey %s cannot be used: %w", accelerator, err)
		}
	}
	if old != "" {
		if err := s.keys.Unregister(old); err != nil {
			log.Printf("hotkey: the old hotkey %s could not be released: %v", old, err)
		}
	}
	s.mu.Lock()
	s.current, s.errMsg = accelerator, ""
	s.mu.Unlock()
	return nil
}

// Error says why the hotkey could not be bound when the program started, or is empty when it works or is off.
func (s *Slot) Error() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.errMsg
}

// SetError remembers why the hotkey could not be bound at start, for the settings to show.
func (s *Slot) SetError(msg string) {
	s.mu.Lock()
	s.errMsg = msg
	s.mu.Unlock()
}
