package palette

import (
	"fmt"
	"strings"
	"unicode/utf8"

	"omni/internal/calc"
	"omni/internal/rank"
)

// Extras are the parts of the bar that reach outside it: each is optional, and the bar works without them (the tests do).
type Extras struct {
	// Copy puts text on the clipboard (a calculator answer).
	Copy func(text string) error
	// RunSystem carries out a system action by id (internal/system.Run).
	RunSystem func(id string) error
	// Notes is the notes folder and OpenNote shows the notes window on a note ("new" for an empty one).
	Notes    NoteBook
	OpenNote func(id string)
	// Timer is the countdown and the pomodoro.
	Timer TimerControl
}

// With sets the extras of s and returns it. It is a function, not a method: the exported methods of a service are bound to
// the page, and a method that returns the service itself collides with the service's own name there.
func With(s *Service, x Extras) *Service {
	s.x = x
	return s
}

// systemMinLength is the shortest text that offers the system rows: `sl` is the start of too many things.
const systemMinLength = 3

// systemCommand is a row that does something to the computer.
type systemCommand struct{ id, title string }

var systemCommands = []systemCommand{
	{"sys:lock", "Lock the computer"},
	{"sys:sleep", "Sleep"},
	{"sys:taskmgr", "Task Manager"},
	{"sys:recycle", "Empty the Recycle Bin"},
	{"sys:winsettings", "Windows Settings"},
}

func (c systemCommand) item() Item {
	return Item{ID: c.id, Kind: "system", Title: c.title, Subtitle: "System"}
}

func systemCommandFor(id string) (systemCommand, bool) {
	for _, c := range systemCommands {
		if c.id == id {
			return c, true
		}
	}
	return systemCommand{}, false
}

// systemItems are the system rows the text starts or abbreviates ("lock", "task", "bin").
func systemItems(text string) []Item {
	if utf8.RuneCountInString(text) < systemMinLength {
		return nil
	}
	var items []Item
	for _, c := range systemCommands {
		if rank.Match(text, c.title).Strong {
			items = append(items, c.item())
		}
	}
	return items
}

// calcItems is the calculator's answer to the text, as a row, or nothing when the text is not a calculation.
func calcItems(text string) []Item {
	r, ok := calc.Answer(text)
	if !ok {
		return nil
	}
	return []Item{{ID: "calc:" + r.Copy, Kind: "calc", Title: r.Title, Subtitle: "Enter copies the answer"}}
}

// doCalc copies the answer of a calculator row and closes the bar.
func (s *Service) doCalc(answer, action string) error {
	if action != "open" {
		return fmt.Errorf("a calculator row can only be opened, not %q", action)
	}
	if s.x.Copy == nil {
		return fmt.Errorf("copying is not available")
	}
	if err := s.x.Copy(strings.TrimSpace(answer)); err != nil {
		return fmt.Errorf("copy the answer: %w", err)
	}
	s.dismiss()
	return nil
}

// doSystem runs a system row and closes the bar.
func (s *Service) doSystem(c systemCommand, action string) error {
	if action != "open" {
		return fmt.Errorf("a system row can only be opened, not %q", action)
	}
	if s.x.RunSystem == nil {
		return fmt.Errorf("system actions are not available")
	}
	s.dismiss() // the bar goes first: locking or sleeping with the bar still up would bring it back when the computer wakes
	if err := s.x.RunSystem(c.id); err != nil {
		return err
	}
	return nil
}
