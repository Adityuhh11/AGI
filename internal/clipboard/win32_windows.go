package clipboard

import (
	"errors"
	"fmt"
	"path/filepath"
	"runtime"
	"strings"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"
)

// The Win32 calls the clipboard needs, kept in one place.

var (
	user32   = windows.NewLazySystemDLL("user32.dll")
	kernel32 = windows.NewLazySystemDLL("kernel32.dll")

	procOpenClipboard              = user32.NewProc("OpenClipboard")
	procCloseClipboard             = user32.NewProc("CloseClipboard")
	procGetClipboardData           = user32.NewProc("GetClipboardData")
	procGetClipboardSequenceNumber = user32.NewProc("GetClipboardSequenceNumber")
	procGetClipboardOwner          = user32.NewProc("GetClipboardOwner")
	procIsClipboardFormatAvailable = user32.NewProc("IsClipboardFormatAvailable")
	procRegisterClipboardFormatW   = user32.NewProc("RegisterClipboardFormatW")
	procGetForegroundWindow        = user32.NewProc("GetForegroundWindow")
	procSendInput                  = user32.NewProc("SendInput")
	procGlobalLock                 = kernel32.NewProc("GlobalLock")
	procGlobalUnlock               = kernel32.NewProc("GlobalUnlock")
	procGlobalSize                 = kernel32.NewProc("GlobalSize")
)

const (
	cfUnicodeText = 13
	// openAttempts and openPause: another program that has just copied may still hold the clipboard open.
	openAttempts = 10
	openPause    = 10 * time.Millisecond

	vkControl = 0x11
	vkV       = 0x56
)

// Formats that programs which handle secrets put on the clipboard next to the text, asking history tools
// not to keep it. The first is a marker; for the other two a value of 0 means "do not".
const (
	formatExclude = "ExcludeClipboardContentFromMonitorProcessing"
	formatHistory = "CanIncludeInClipboardHistory"
	formatCloud   = "CanUploadToCloudClipboard"
)

func registerFormat(name string) uint32 {
	p, _ := windows.UTF16PtrFromString(name)
	id, _, _ := procRegisterClipboardFormatW.Call(uintptr(unsafe.Pointer(p)))
	return uint32(id)
}

func formatAvailable(format uint32) bool {
	ok, _, _ := procIsClipboardFormatAvailable.Call(uintptr(format))
	return ok != 0
}

func openClipboard() error {
	for range openAttempts {
		if ok, _, _ := procOpenClipboard.Call(0); ok != 0 {
			return nil
		}
		time.Sleep(openPause)
	}
	return errors.New("the clipboard is held open by another program")
}

func closeClipboard() { _, _, _ = procCloseClipboard.Call() }

// globalBytes copies the memory block a clipboard format lives in, or reports that it is not there. The
// clipboard must be open.
func globalBytes(format uint32, limit uintptr) (data []byte, present bool, err error) {
	h, _, _ := procGetClipboardData.Call(uintptr(format))
	if h == 0 {
		return nil, false, nil
	}
	size, _, _ := procGlobalSize.Call(h)
	if size > limit {
		return nil, true, ErrTooLong
	}
	locked, _, _ := procGlobalLock.Call(h)
	if locked == 0 {
		return nil, true, errors.New("could not read the clipboard's memory")
	}
	defer procGlobalUnlock.Call(h)
	// locked is the address of memory Windows owns, valid until the unlock above.
	p := *(*unsafe.Pointer)(unsafe.Pointer(&locked))
	return append([]byte(nil), unsafe.Slice((*byte)(p), size)...), true, nil
}

// markedPrivate reports whether the program that copied asked for the content to be kept out of any history.
// The clipboard must be open.
func markedPrivate() bool {
	if exclude := registerFormat(formatExclude); formatAvailable(exclude) {
		return true
	}
	for _, name := range []string{formatHistory, formatCloud} {
		format := registerFormat(name)
		if !formatAvailable(format) {
			continue
		}
		if data, _, err := globalBytes(format, 16); err == nil && len(data) >= 4 && data[0]|data[1]|data[2]|data[3] == 0 {
			return true
		}
	}
	return false
}

// NoteOwnCopy has nothing to do on Windows: a copy names the program that made it (the clipboard's owner window), so this program's own copies
// are recognised by name. A Mac does not say who copied (source_darwin.go).
func NoteOwnCopy() {}

// NewSource is the real clipboard.
func NewSource() Source { return WindowsSource{} }

// WindowsSource is the real clipboard.
type WindowsSource struct{}

// Sequence changes whenever anything is copied.
func (WindowsSource) Sequence() uint32 {
	n, _, _ := procGetClipboardSequenceNumber.Call()
	return uint32(n)
}

// Read returns the text on the clipboard, who put it there and whether they asked for it to be kept private.
// Something that is not text comes back as a Clip with no text.
func (WindowsSource) Read() (Clip, error) {
	runtime.LockOSThread() // the clipboard is opened and closed by the same thread
	defer runtime.UnlockOSThread()
	clip := Clip{Owner: clipboardOwnerName()}
	if err := openClipboard(); err != nil {
		return clip, err
	}
	defer closeClipboard()
	clip.Private = markedPrivate()
	if !formatAvailable(cfUnicodeText) {
		return clip, nil
	}
	data, present, err := globalBytes(cfUnicodeText, 4*(MaxText+1)) // a rune is at most two UTF-16 units
	if err != nil || !present {
		return clip, err
	}
	units := unsafe.Slice((*uint16)(unsafe.Pointer(unsafe.SliceData(data))), len(data)/2)
	clip.Text = windows.UTF16ToString(units)
	return clip, nil
}

func clipboardOwnerName() string {
	hwnd, _, _ := procGetClipboardOwner.Call()
	if hwnd == 0 {
		return ""
	}
	return windowProgram(hwnd)
}

// windowProgram is the name of the program that owns a window, without its folder or ".exe" ("chrome").
func windowProgram(hwnd uintptr) string {
	process, err := openWindowProcess(hwnd, windows.PROCESS_QUERY_LIMITED_INFORMATION)
	if err != nil {
		return ""
	}
	defer windows.CloseHandle(process)
	buf := make([]uint16, windows.MAX_PATH)
	size := uint32(len(buf))
	if err := windows.QueryFullProcessImageName(process, 0, &buf[0], &size); err != nil {
		return ""
	}
	return strings.TrimSuffix(filepath.Base(windows.UTF16ToString(buf[:size])), ".exe")
}

func openWindowProcess(hwnd uintptr, access uint32) (windows.Handle, error) {
	var pid uint32
	if _, err := windows.GetWindowThreadProcessId(windows.HWND(hwnd), &pid); err != nil || pid == 0 {
		return 0, fmt.Errorf("find the program of a window: %v", err)
	}
	return windows.OpenProcess(access, false, pid)
}

func foregroundWindow() uintptr {
	h, _, _ := procGetForegroundWindow.Call()
	return h
}

// blockedByElevation reports whether Windows would refuse this program's keys to the window: its program runs
// elevated (or cannot even be asked) and this one does not.
func blockedByElevation(hwnd uintptr) bool {
	if windows.GetCurrentProcessToken().IsElevated() {
		return false
	}
	process, err := openWindowProcess(hwnd, windows.PROCESS_QUERY_LIMITED_INFORMATION)
	if err != nil {
		return true // a program that cannot be opened to be asked is protected or elevated
	}
	defer windows.CloseHandle(process)
	var token windows.Token
	if err := windows.OpenProcessToken(process, windows.TOKEN_QUERY, &token); err != nil {
		return true
	}
	defer token.Close()
	return token.IsElevated()
}

// keyInput is Windows' INPUT structure for a keyboard event, laid out for 64-bit Windows (40 bytes).
type keyInput struct {
	Type  uint32
	_     uint32
	VK    uint16
	Scan  uint16
	Flags uint32
	Time  uint32
	_     uint32
	Extra uintptr
	_     [8]byte
}

const (
	inputKeyboard  = 1
	keyEventKeyUp  = 0x0002
	keyInputLength = unsafe.Sizeof(keyInput{})
)

// sendCtrlV types Ctrl+V into whatever has the keyboard.
func sendCtrlV() error {
	key := func(vk uint16, flags uint32) keyInput { return keyInput{Type: inputKeyboard, VK: vk, Flags: flags} }
	events := []keyInput{key(vkControl, 0), key(vkV, 0), key(vkV, keyEventKeyUp), key(vkControl, keyEventKeyUp)}
	sent, _, err := procSendInput.Call(uintptr(len(events)), uintptr(unsafe.Pointer(&events[0])), keyInputLength)
	if int(sent) != len(events) {
		return fmt.Errorf("type Ctrl+V: %d of %d key events were accepted: %v", sent, len(events), err)
	}
	return nil
}

// NewPaster returns the Paster that types into the real windows. setText writes the clipboard (the app's own
// clipboard does), previous is the window the bar was opened from, release closes the bar and returns the keyboard to it.
func NewPaster(setText func(string) bool, previous func() uintptr, release func()) Paster {
	return Paster{
		SetText:    setText,
		Previous:   previous,
		Release:    release,
		Foreground: foregroundWindow,
		Allowed: func(window uintptr) error {
			if blockedByElevation(window) {
				return ErrBlocked
			}
			return nil
		},
		SendPaste:  sendCtrlV,
		Sleep:      time.Sleep,
	}
}
