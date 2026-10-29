package panel

/*
#cgo CFLAGS: -x objective-c -fobjc-arc
#cgo LDFLAGS: -framework Cocoa
#include "native_darwin.h"
*/
import "C"

import (
	"math"
	"os"

	"github.com/wailsapp/wails/v3/pkg/application"
)

// The window calls the panel needs from the system, for a Mac (native_darwin.m; the Windows ones are win32_windows.go). A "window"
// here is the NSWindow's address as a number, and the window that "had the keyboard" is the application that was frontmost, by its
// process id: the panel never activates this program, so that application stays the active one while the panel is up.

// platformWindow makes the panel window a non-activating NSPanel: it takes typing without making this program the active one (so the app
// underneath keeps its place, and a full-screen app's Space is not left), floats above ordinary windows, appears on whichever Space is
// current and over full-screen apps, and leaves Mission Control and ⌘` alone. The page draws its own shadow.
func platformWindow(options *application.WebviewWindowOptions) {
	options.Mac = application.MacWindow{
		WindowClass:      application.MacWindowClassPanel,
		PanelPreferences: application.MacPanelPreferences{FloatingPanel: true, NonActivating: true},
		WindowLevel:      application.MacWindowLevelFloating,
		CollectionBehavior: application.MacWindowCollectionBehaviorCanJoinAllSpaces |
			application.MacWindowCollectionBehaviorFullScreenAuxiliary |
			application.MacWindowCollectionBehaviorIgnoresCycle,
		DisableShadow: true,
		// The page draws the whole bar, its rounded edge and its shadow, on a clear window. A frameless Mac window is still a titled
		// one with a hidden title bar, which keeps the system's corners and an opaque plate behind the page; "square" gives a
		// borderless window with nothing of its own.
		Backdrop:   application.MacBackdropTransparent,
		CornerType: application.MacWindowCornerTypeSquare,
	}
}

// foregroundWindow is the application that has the keyboard, or 0 when that is this program (its own windows are not somewhere to paste).
func foregroundWindow() uintptr {
	pid := int(C.omniFrontmostPID())
	if pid == os.Getpid() {
		return 0
	}
	return uintptr(pid)
}

// setForeground brings the application back to the front. The panel never took that from it, so this only matters when something else did.
func setForeground(app uintptr) { C.omniActivate(C.int(app)) }

// isWindow says whether the application is still running.
func isWindow(app uintptr) bool { return app != 0 && C.omniIsRunning(C.int(app)) != 0 }

// cloak on a Mac is plain ordering: out when on, in when off. (ponytail: if the first frame after showing turns out stale or blank, keep
// the panel ordered in at alpha 0 instead; the decision is in docs/superpowers/specs/2026-10-29-macos-design.md, section 0.)
func cloak(window uintptr, on bool) {
	if on {
		C.omniOrderOut(C.uintptr_t(window))
	} else {
		C.omniOrderFront(C.uintptr_t(window))
	}
}

// moveTopmost puts the window at r (points).
func moveTopmost(window uintptr, r Rect) {
	C.omniSetFrame(C.uintptr_t(window), C.double(r.X), C.double(r.Y), C.double(r.W), C.double(r.H))
}

// cursorMonitor returns the usable area of the screen that holds the mouse, in points, and 96: a point is one device-independent
// pixel, so place() needs no scaling.
func cursorMonitor() (Rect, uint32) {
	var x, y, w, h C.double
	C.omniScreenUnderMouse(&x, &y, &w, &h)
	return rect(x, y, w, h), 96
}

// windowRect is where the window is, in points.
func windowRect(window uintptr) Rect {
	var x, y, w, h C.double
	C.omniWindowFrame(C.uintptr_t(window), &x, &y, &w, &h)
	return rect(x, y, w, h)
}

func windowDPI(uintptr) uint32 { return 96 }

// resizeHeight changes the height in place, keeping the top edge.
func resizeHeight(window uintptr, heightPx int) { C.omniSetHeight(C.uintptr_t(window), C.double(heightPx)) }

func rect(x, y, w, h C.double) Rect {
	return Rect{X: int(math.Round(float64(x))), Y: int(math.Round(float64(y))), W: int(math.Round(float64(w))), H: int(math.Round(float64(h)))}
}
