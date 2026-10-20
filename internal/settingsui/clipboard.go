package settingsui

import (
	"strings"

	"agi/internal/clipboard"
	"agi/internal/config"
)

// ClipboardView is the Clipboard page: whether the history is on, how much it holds, and the programs it leaves out.
type ClipboardView struct {
	Enabled bool
	Count   int
	// Ignore are the programs the user added; Defaults are the password managers always left out.
	Ignore   []string
	Defaults []string
	File     string
	// Hotkey opens the bar listing what was copied ("" means there is none); HotkeyError is why it could not be bound at start.
	Hotkey, HotkeyError string
}

// Clipboard returns the Clipboard page.
func (s *Service) Clipboard() ClipboardView {
	c := s.o.Store.Get().Clipboard
	return ClipboardView{
		Enabled: c.Enabled, Count: s.o.Clipboard.Len(), Ignore: c.Ignore,
		Defaults: clipboard.DefaultIgnore, File: s.o.Clipboard.Path(),
		Hotkey: c.Hotkey, HotkeyError: s.o.ClipboardHotkeyError(),
	}
}

// SaveClipboard switches the history on or off and sets the programs it leaves out (blank lines are dropped) and
// its hotkey. A hotkey that cannot be bound is refused and the working one stays.
func (s *Service) SaveClipboard(enabled bool, ignore []string, hotkey string) string {
	var programs []string
	for _, name := range ignore {
		if name = strings.TrimSpace(name); name != "" {
			programs = append(programs, name)
		}
	}
	hotkey = strings.TrimSpace(hotkey)
	old := s.o.Store.Get().Clipboard.Hotkey
	if hotkey != old {
		if err := s.o.RebindClipboard(hotkey); err != nil {
			return err.Error()
		}
	}
	err := s.o.Store.Update(func(c *config.Config) {
		c.Clipboard = config.Clipboard{Enabled: enabled, Ignore: programs, Hotkey: hotkey}
	})
	if err != nil && hotkey != old {
		_ = s.o.RebindClipboard(old) // the window shows the refusal; the key that worked keeps working
	}
	return reason(err)
}

// ClearClipboard forgets everything the history holds.
func (s *Service) ClearClipboard() string {
	if err := s.o.Clipboard.Clear(); err != nil {
		return err.Error()
	}
	return ""
}
