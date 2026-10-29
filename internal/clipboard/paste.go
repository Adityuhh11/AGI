package clipboard

import (
	"errors"
	"fmt"
	"time"
)

// ErrBlocked and ErrNoTarget say why nothing was typed. In both the text is on the clipboard already, and the bar stays
// open to say so. Their wording is each system's (messages_windows.go, messages_darwin.go), since the keys and the reason differ.

const (
	// foregroundWait is how long the program the keyboard goes back to gets to become the foreground window.
	foregroundWait = 500 * time.Millisecond
	// settle is how long that program gets, once it is the foreground window, to be ready for keys.
	settle = 40 * time.Millisecond
)

// Paster types text into the window the bar was opened from. Its parts are system calls, held as fields so
// the order of events can be tested without a screen; NewPaster fills them in.
type Paster struct {
	// SetText puts text on the clipboard.
	SetText func(text string) bool
	// Previous is the window (on a Mac, the application) that had the keyboard before the bar opened, or 0 if there is none any more.
	Previous func() uintptr
	// Release closes the bar and hands the keyboard back to Previous.
	Release func()
	// Foreground is the window that has the keyboard now.
	Foreground func() uintptr
	// Allowed is nil when this program may type into the window, and otherwise the error that says why not (ErrBlocked when the
	// window runs as administrator on Windows, or a password field has all keyboard input to itself on a Mac; on a Mac also
	// that the Accessibility permission is missing). The text is on the clipboard already, and the bar stays open to say so.
	Allowed func(window uintptr) error
	// SendPaste types the paste key (Ctrl+V on Windows, Command+V on a Mac).
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
	}
	if err := p.Allowed(target); err != nil {
		return err
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
