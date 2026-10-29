package clipboard

/*
#include "pasteboard_darwin.h"
*/
import "C"

import (
	"errors"
	"sync"
	"time"
)

// ErrNoAccessibility says why nothing was typed when the person has not given this program the Accessibility permission, which macOS asks
// for before any program may type into another. The text is on the clipboard already.
var ErrNoAccessibility = errors.New("Copied. To paste for you, Omni needs Accessibility: allow it in System Settings > Privacy & Security > Accessibility, then paste again. Press ⌘V for now.")

// askedOnce keeps macOS' own dialog (it offers to open System Settings) to the first time: asking at every paste would be nagging.
var askedOnce sync.Once

// NewPaster returns the Paster for a Mac: it types ⌘V into the application the bar came from. That application never stopped being the active one
// (the bar does not activate this program), so Foreground is it at once and the wait is short. Typing into another program needs the Accessibility
// permission, and does not work while a password field has the keyboard to itself.
func NewPaster(setText func(string) bool, previous func() uintptr, release func()) Paster {
	return Paster{
		SetText: func(text string) bool { return OwnCopy(func() bool { return setText(text) }) },
		Previous:   previous,
		Release:    release,
		Foreground: func() uintptr { return uintptr(C.omniClipFrontmostPID()) },
		Allowed:    allowed,
		SendPaste: func() error {
			if C.omniSendPaste() != 0 {
				return errors.New("the keys could not be sent")
			}
			return nil
		},
		Sleep: time.Sleep,
	}
}

// allowed says whether this program may type into the application: not while a password field has the keyboard, and only with the Accessibility permission.
func allowed(uintptr) error {
	if C.omniSecureInput() != 0 {
		return ErrBlocked
	}
	if C.omniAccessibilityTrusted(0) == 0 {
		askedOnce.Do(func() { C.omniAccessibilityTrusted(1) })
		return ErrNoAccessibility
	}
	return nil
}
