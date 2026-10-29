package settingsui

import (
	"strings"

	"omni/internal/config"
)

// NotesHotkeyView is the notes window's hotkey: "" means there is none; Error is why it could not be bound at start.
type NotesHotkeyView struct {
	Hotkey, Error string
}

// NotesHotkey returns the notes window's hotkey.
func (s *Service) NotesHotkey() NotesHotkeyView {
	return NotesHotkeyView{Hotkey: s.o.Store.Get().Notes.Hotkey, Error: s.o.NotesHotkeyError()}
}

// SetNotesHotkey moves the notes hotkey. One that cannot be bound is refused and the working one stays.
func (s *Service) SetNotesHotkey(accelerator string) string {
	accelerator = strings.TrimSpace(accelerator)
	old := s.o.Store.Get().Notes.Hotkey
	if accelerator == old {
		return ""
	}
	// A combination another hotkey has is refused before it is registered, so it is never bound.
	probe := s.o.Store.Get()
	probe.Notes.Hotkey = accelerator
	if err := probe.Validate(); err != nil {
		return reason(err)
	}
	if err := s.o.RebindNotes(accelerator); err != nil {
		return err.Error()
	}
	err := s.o.Store.Update(func(c *config.Config) { c.Notes.Hotkey = accelerator })
	if err != nil {
		_ = s.o.RebindNotes(old) // the page shows the refusal; the key that worked keeps working
	}
	return reason(err)
}
