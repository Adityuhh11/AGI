package panel

// The window calls the panel needs from the system, for a Mac. They do nothing yet: the real ones (a non-activating NSPanel, the screen under
// the mouse, the frontmost application) are milestone M1, in Objective-C. Until then the panel is created and the hotkey binds, but the window is
// never put on screen, which is all milestone M0 (the program builds and starts) asks of it.
//
// Everything here has the Windows meaning in win32_windows.go; a window handle is 0 because no window is tracked.

func foregroundWindow() uintptr { return 0 }

func setForeground(uintptr) {}

func isWindow(uintptr) bool { return false }

func cloak(uintptr, bool) {}

func moveTopmost(uintptr, Rect) {}

func cursorMonitor() (Rect, uint32) { return Rect{}, 96 }

func windowRect(uintptr) Rect { return Rect{} }

func windowDPI(uintptr) uint32 { return 96 }

func resizeHeight(uintptr, int) {}
