package apps

import (
	"fmt"
	"path/filepath"
	"strings"
	"sync"
	"unsafe"

	"golang.org/x/sys/windows"
)

var (
	user32             = windows.NewLazySystemDLL("user32.dll")
	procGetWindow      = user32.NewProc("GetWindow")
	procGetWindowLongW = user32.NewProc("GetWindowLongW")
	procIsIconic       = user32.NewProc("IsIconic")
	procShowWindow     = user32.NewProc("ShowWindow")
	procSetForeground  = user32.NewProc("SetForegroundWindow")
)

const (
	gwOwner                        = 4
	gwlExStyle                     = -20
	wsExToolWindow                 = 0x80
	swRestore                      = 9
	processQueryLimitedInformation = 0x1000
)

// probe is what one pass over the windows is looking for. A Windows callback can never be freed, so there
// is one for the whole process and the question goes in through this variable.
var probe struct {
	sync.Mutex
	app   App
	file  string
	found windows.HWND
}

var enumCallback = windows.NewCallback(func(hwnd windows.HWND, _ uintptr) uintptr {
	if !isMainWindow(hwnd) {
		return 1
	}
	var pid uint32
	if _, err := windows.GetWindowThreadProcessId(hwnd, &pid); err != nil {
		return 1
	}
	exe := imagePath(pid)
	if exe == "" || !sameApp(probe.app.Name, probe.file, exe, fileDescription) {
		return 1
	}
	probe.found = hwnd
	return 0
})

// focusRunning brings the app's window to the front if the app is already open, and says whether it did.
// Windows lists windows front to back, so the first match is the one the user used last.
func focusRunning(app App, file string) bool {
	probe.Lock()
	defer probe.Unlock()
	probe.app, probe.file, probe.found = app, file, 0
	_ = windows.EnumWindows(enumCallback, nil) // reports an error when the callback stops it early
	hwnd := probe.found
	if hwnd == 0 {
		return false
	}
	if iconic, _, _ := procIsIconic.Call(uintptr(hwnd)); iconic != 0 {
		_, _, _ = procShowWindow.Call(uintptr(hwnd), swRestore)
	}
	ok, _, _ := procSetForeground.Call(uintptr(hwnd))
	return ok != 0
}

// sameApp tells whether a running program is this Start Menu entry. An entry that names its file is matched
// by that file. Most do not (Zed and Zen are only an id), so the program's name, or the description inside
// its exe ("Google Chrome" for chrome.exe), is compared with the entry's name.
// ponytail: matching by name can miss (Store apps run inside ApplicationFrameHost, a renamed shortcut)
// and then the app simply starts again; reading each Start Menu shortcut's id and target would be exact.
func sameApp(name, file, exe string, describe func(exe string) string) bool {
	if file != "" {
		return strings.EqualFold(file, exe)
	}
	if strings.EqualFold(name, strings.TrimSuffix(filepath.Base(exe), filepath.Ext(exe))) {
		return true
	}
	return strings.EqualFold(name, describe(exe))
}

// isMainWindow is true for a window the user would call "the app": visible, not owned by another window and
// not a tool window (tray popups, hidden helpers).
func isMainWindow(hwnd windows.HWND) bool {
	if !windows.IsWindowVisible(hwnd) {
		return false
	}
	if owner, _, _ := procGetWindow.Call(uintptr(hwnd), gwOwner); owner != 0 {
		return false
	}
	style, _, _ := procGetWindowLongW.Call(uintptr(hwnd), uintptr(gwlExStyle&0xffffffff))
	return style&wsExToolWindow == 0
}

// imagePath is the exe a process runs from, or "" when it cannot be asked (a program running as
// administrator, for one).
func imagePath(pid uint32) string {
	h, err := windows.OpenProcess(processQueryLimitedInformation, false, pid)
	if err != nil {
		return ""
	}
	defer windows.CloseHandle(h)
	buf := make([]uint16, windows.MAX_LONG_PATH)
	n := uint32(len(buf))
	if windows.QueryFullProcessImageName(h, 0, &buf[0], &n) != nil {
		return ""
	}
	return windows.UTF16ToString(buf[:n])
}

// fileDescription is the "File description" of an exe's version info, which is the name Windows shows for it.
func fileDescription(exe string) string {
	size, err := windows.GetFileVersionInfoSize(exe, nil)
	if err != nil || size == 0 {
		return ""
	}
	block := make([]byte, size)
	if windows.GetFileVersionInfo(exe, 0, size, unsafe.Pointer(&block[0])) != nil {
		return ""
	}
	var lang *[2]uint16
	var n uint32
	if windows.VerQueryValue(unsafe.Pointer(&block[0]), `\VarFileInfo\Translation`, unsafe.Pointer(&lang), &n) != nil || n < 4 {
		return ""
	}
	var text *uint16
	key := fmt.Sprintf(`\StringFileInfo\%04x%04x\FileDescription`, lang[0], lang[1])
	if windows.VerQueryValue(unsafe.Pointer(&block[0]), key, unsafe.Pointer(&text), &n) != nil || n == 0 {
		return ""
	}
	return windows.UTF16PtrToString(text)
}
