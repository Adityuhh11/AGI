// Package dock decides whether the launcher shows in the Dock and the app switcher. It matters on a Mac only.
//
// On a Mac the launcher is an accessory app: no Dock icon, no ⌘Tab entry, no menu bar, which is right while only the search bar is
// open. The windows that are ordinary windows (Settings, the guide, Notes, the chat) need those: a window of an accessory app cannot be reached with ⌘Tab,
// and its program has no menu bar, so ⌘C and ⌘V would not even work in its text fields. So the program becomes a regular app while at least one such window
// is open, and goes back when the last one closes. On Windows there is nothing to decide (dock_windows.go does nothing).
package dock

import "sync"

// Windows counts the ordinary windows that are open and says when that stops or starts being zero.
type Windows struct {
	mu   sync.Mutex
	open int
	set  func(regular bool)
}

// Counter is the one the program uses; set is the system's (dock_windows.go, dock_darwin.go).
var Counter = &Windows{set: setRegular}

// Opened tells that a window opened.
func (w *Windows) Opened() {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.open++
	if w.open == 1 {
		w.set(true)
	}
}

// Closed tells that a window closed. A close without an open (a window closed twice) is ignored.
func (w *Windows) Closed() {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.open == 0 {
		return
	}
	w.open--
	if w.open == 0 {
		w.set(false)
	}
}
