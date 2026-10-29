package clipboard

import (
	"errors"
	"time"
)

// NewPaster returns the Paster for a Mac. Typing ⌘V into the app the bar came from (CGEvent, which needs the Accessibility permission)
// is milestone M2. Until then the bar knows of no app to paste into (Previous is 0), so a paste copies and says to press ⌘V, which is
// what it does on a Mac without that permission.
func NewPaster(setText func(string) bool, previous func() uintptr, release func()) Paster {
	return Paster{
		SetText:    setText,
		Previous:   previous,
		Release:    release,
		Foreground: func() uintptr { return 0 },
		Blocked:    func(uintptr) bool { return false },
		SendPaste: func() error {
			return errors.New("pasting into other apps is not available on macOS yet: press ⌘V yourself")
		},
		Sleep: time.Sleep,
	}
}
