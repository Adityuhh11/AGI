package clipboard

import (
	"errors"
	"fmt"
	"time"
)

// ErrElevated and ErrNoTarget say why nothing was typed. In both the text is on the clipboard already, and
// the bar stays open to say so.
var (
	ErrElevated = errors.New("Copied. That window runs as administrator, which Windows does not let this program type into: press Ctrl+V yourself.")
	ErrNoTarget = errors.New("Copied. There is no window to paste into: press Ctrl+V where you want it.")
)

const (
	// foregroundWait is how long the program the keyboard goes back to gets to become the foreground window.
	foregroundWait = 500 * time.Millisecond
	// settle is how long that program gets, once it is the foreground window, to be ready for keys.
	settle = 40 * time.Millisecond
)

// Paster types text into the window the bar was opened from. Its parts are Windows calls, held as fields so
// the order of events can be tested without a screen; NewPaster fills them in.
type Paster struct {
	// SetText puts text on the clipboard.
	SetText func(text string) bool
	// Previous is the window that had the keyboard before the bar opened, or 0 if there is none any more.
	Previous func() uintptr
	// Release closes the bar and hands the keyboard back to Previous.
	Release func()
	// Foreground is the window that has the keyboard now.
	Foreground func() uintptr
	// Elevated says that Windows will not let this program type into the window.
	Elevated func(window uintptr) bool
	// SendPaste types Ctrl+V.
	SendPaste func() error
	Sleep     func(time.Duration)
}

// Paste puts text on the clipboard, closes the bar and types Ctrl+V into the program the bar came from. If
// that program cannot be typed into the bar stays open and the error says so.
func (p Paster) Paste(text string) error {
	target := p.Previous()
	if err := p.setText(text); err != nil {
		return err
	}
	switch {
	case target == 0:
		return ErrNoTarget
	case p.Elevated(target):
		return ErrElevated
	}
	p.Release()
	for waited := time.Duration(0); p.Foreground() != target; waited += 5 * time.Millisecond {
		if waited >= foregroundWait {
			return fmt.Errorf("the program did not take the keyboard back, so nothing was pasted (the text is on the clipboard)")
		}
		p.Sleep(5 * time.Millisecond)
	}
	p.Sleep(settle)
	return p.SendPaste()
}

// Copy puts text on the clipboard and gives the keyboard back, without typing anything.
func (p Paster) Copy(text string) error {
	if err := p.setText(text); err != nil {
		return err
	}
	p.Release()
	return nil
}

func (p Paster) setText(text string) error {
	if !p.SetText(text) {
		return errors.New("the clipboard could not be written to")
	}
	return nil
}
