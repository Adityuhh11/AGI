package panel

import (
	"unsafe"

	"golang.org/x/sys/windows"
)

// The few Win32 calls the panel needs, kept in one place. All coordinates are physical pixels
// (Wails makes the process per-monitor DPI aware).

var (
	user32 = windows.NewLazySystemDLL("user32.dll")
	dwmapi = windows.NewLazySystemDLL("dwmapi.dll")
	shcore = windows.NewLazySystemDLL("shcore.dll")

	procGetForegroundWindow   = user32.NewProc("GetForegroundWindow")
	procSetForegroundWindow   = user32.NewProc("SetForegroundWindow")
	procIsWindow              = user32.NewProc("IsWindow")
	procGetDpiForWindow       = user32.NewProc("GetDpiForWindow")
	procGetWindowRect         = user32.NewProc("GetWindowRect")
	procGetCursorPos          = user32.NewProc("GetCursorPos")
	procMonitorFromPoint      = user32.NewProc("MonitorFromPoint")
	procGetMonitorInfoW       = user32.NewProc("GetMonitorInfoW")
	procSetWindowPos          = user32.NewProc("SetWindowPos")
	procDwmSetWindowAttribute = dwmapi.NewProc("DwmSetWindowAttribute")
	procGetDpiForMonitor      = shcore.NewProc("GetDpiForMonitor")
)

const (
	monitorDefaultToNearest = 2
	swpNoActivate           = 0x0010
	swpNoMove               = 0x0002
	swpNoZOrder             = 0x0004
	dwmwaCloak              = 13
)

type point struct{ X, Y int32 }

type winRect struct{ Left, Top, Right, Bottom int32 }

type monitorInfo struct {
	Size    uint32
	Monitor winRect
	Work    winRect
	Flags   uint32
}

func foregroundWindow() uintptr {
	h, _, _ := procGetForegroundWindow.Call()
	return h
}

func setForeground(hwnd uintptr) { _, _, _ = procSetForegroundWindow.Call(hwnd) }

func isWindow(hwnd uintptr) bool {
	ok, _, _ := procIsWindow.Call(hwnd)
	return hwnd != 0 && ok != 0
}

// cloak hides a window from view without hiding it from Windows: its web view keeps rendering,
// so uncloaking shows an already painted frame (measured 41 ms hotkey to focused input against
// 72 ms for Show()/Hide()).
func cloak(hwnd uintptr, on bool) {
	var value uint32
	if on {
		value = 1
	}
	_, _, _ = procDwmSetWindowAttribute.Call(hwnd, dwmwaCloak, uintptr(unsafe.Pointer(&value)), 4)
}

// moveTopmost puts the window at r without activating it.
func moveTopmost(hwnd uintptr, r Rect) {
	hwndTopmost := ^uintptr(0) // HWND_TOPMOST is (HWND)-1
	_, _, _ = procSetWindowPos.Call(hwnd, hwndTopmost,
		uintptr(int32(r.X)), uintptr(int32(r.Y)), uintptr(int32(r.W)), uintptr(int32(r.H)), swpNoActivate)
}

// cursorMonitor returns the usable area (taskbar excluded) and the dpi of the monitor that holds
// the mouse cursor, which is where the user is looking.
func cursorMonitor() (Rect, uint32) {
	var pt point
	_, _, _ = procGetCursorPos.Call(uintptr(unsafe.Pointer(&pt)))
	// A POINT is passed by value: on 64-bit Windows its two 32-bit halves travel in one register.
	packed := uintptr(uint32(pt.X)) | uintptr(uint32(pt.Y))<<32
	monitor, _, _ := procMonitorFromPoint.Call(packed, monitorDefaultToNearest)
	info := monitorInfo{Size: uint32(unsafe.Sizeof(monitorInfo{}))}
	_, _, _ = procGetMonitorInfoW.Call(monitor, uintptr(unsafe.Pointer(&info)))
	var dpiX, dpiY uint32
	_, _, _ = procGetDpiForMonitor.Call(monitor, 0, uintptr(unsafe.Pointer(&dpiX)), uintptr(unsafe.Pointer(&dpiY)))
	work := Rect{
		X: int(info.Work.Left), Y: int(info.Work.Top),
		W: int(info.Work.Right - info.Work.Left), H: int(info.Work.Bottom - info.Work.Top),
	}
	return work, dpiX
}

// windowRect returns where the window is, in physical pixels.
func windowRect(hwnd uintptr) Rect {
	var r winRect
	_, _, _ = procGetWindowRect.Call(hwnd, uintptr(unsafe.Pointer(&r)))
	return Rect{X: int(r.Left), Y: int(r.Top), W: int(r.Right - r.Left), H: int(r.Bottom - r.Top)}
}

// windowDPI returns the dpi of the monitor the window is on (96 if unknown).
func windowDPI(hwnd uintptr) uint32 {
	dpi, _, _ := procGetDpiForWindow.Call(hwnd)
	if dpi == 0 {
		return 96
	}
	return uint32(dpi)
}

// resizeHeight changes the window's height in place, keeping its top-left corner.
func resizeHeight(hwnd uintptr, heightPx int) {
	width := windowRect(hwnd).W
	_, _, _ = procSetWindowPos.Call(hwnd, 0, 0, 0, uintptr(int32(width)), uintptr(int32(heightPx)),
		swpNoMove|swpNoZOrder|swpNoActivate)
}
