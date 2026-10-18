// Package panel owns the search bar window: it opens on a global hotkey, takes the keyboard,
// and gives it back to the app the user came from when it closes.
//
// The window is never hidden, only cloaked (see cloak), because Wails' own Show() and Hide() cost
// about 70 ms and stop the web view from rendering. Cloaked, the page stays painted and
// uncloaking is nearly instant. A cloaked window still owns the keyboard focus, so closing the
// panel hands focus back to the window that had it before.
package panel

import (
	"fmt"
	"log"
	"sync"
	"sync/atomic"
	"time"

	"github.com/wailsapp/wails/v3/pkg/application"
	"github.com/wailsapp/wails/v3/pkg/events"
)

// barHeightDIP is the height of the bare search bar: the 65 DIP pill (60 plus its 2.5 DIP metal
// ring on each side) and the clear margin the page leaves for its shadow (12 above, 40 below).
const barHeightDIP = 117

// offscreen is where the window waits before its first use (relative to the primary work area).
const offscreen = -32000

// shortcuts is the part of Wails' global shortcut manager the panel uses.
type shortcuts interface {
	Register(accelerator string, callback func()) error
	Unregister(accelerator string) error
}

// Panel is the search bar window.
type Panel struct {
	app  *application.App
	win  *application.WebviewWindow
	keys shortcuts

	// OnHotkeyError, if set before the app runs, is called whenever the hotkey's state changes: it turns
	// out to be taken, or it was moved to another combination.
	OnHotkeyError func()

	shown atomic.Bool

	mu           sync.Mutex
	hotkey       string  // the accelerator the panel is asked to listen on
	previous     uintptr // the window that had the keyboard before the panel opened
	heightDIP    int
	hotkeyErrMsg string
}

// New creates the (still invisible) window and arranges to bind hotkey once the app runs.
func New(app *application.App, hotkey string) *Panel {
	// The window takes the keyboard when it is created; remember who had it to give it back.
	p := &Panel{app: app, keys: app.GlobalShortcut, hotkey: hotkey, heightDIP: barHeightDIP, previous: foregroundWindow()}
	p.win = app.Window.NewWithOptions(application.WebviewWindowOptions{
		Name:          "panel",
		Title:         "AGI",
		Width:         widthDIP,
		Height:        barHeightDIP,
		Frameless:     true,
		AlwaysOnTop:   true,
		DisableResize: true,
		// Wails creates a hidden window's native window only when it is first shown, and a
		// shown-then-hidden web view stops rendering. So the window exists from the start, far
		// off screen, and is cloaked as soon as it has a handle.
		InitialPosition: application.WindowXY,
		X:               offscreen,
		Y:               offscreen,
		// The page draws the whole bar, including its rounded outline and its shadow, on a clear
		// window: Windows' own frame would add a square border and shadow around the pill.
		BackgroundType:   application.BackgroundTypeTransparent,
		BackgroundColour: application.NewRGBA(0, 0, 0, 0),
		URL:              "/",
		Windows: application.WindowsWindow{
			HiddenOnTaskbar:                   true,
			DisableFramelessWindowDecorations: true,
		},
	})
	p.win.OnWindowEvent(events.Common.WindowLostFocus, func(*application.WindowEvent) {
		p.hide(false) // the user clicked elsewhere: do not steal the focus back
	})
	app.Event.OnApplicationEvent(events.Common.ApplicationStarted, func(*application.ApplicationEvent) {
		go p.prepare()
	})
	return p
}

// HotkeyError says why the hotkey could not be bound (for example another app already owns it),
// or is empty when it works.
func (p *Panel) HotkeyError() string {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.hotkeyErrMsg
}

// prepare cloaks the window as soon as it exists and binds the hotkey.
func (p *Panel) prepare() {
	var hwnd uintptr
	for deadline := time.Now().Add(5 * time.Second); hwnd == 0 && time.Now().Before(deadline); time.Sleep(10 * time.Millisecond) {
		hwnd = p.hwnd()
	}
	if hwnd == 0 {
		log.Print("panel: the window never got a handle; the hotkey will not work")
		return
	}
	cloak(hwnd, true)
	go p.releaseKeyboardAfterCreation(hwnd)
	p.mu.Lock()
	hotkey := p.hotkey
	p.mu.Unlock()
	if err := p.keys.Register(hotkey, p.Toggle); err != nil {
		p.mu.Lock()
		p.hotkeyErrMsg = fmt.Sprintf("the hotkey %s cannot be used: %v", hotkey, err)
		p.mu.Unlock()
		log.Print("panel: ", p.HotkeyError())
		if p.OnHotkeyError != nil {
			p.OnHotkeyError()
		}
	}
}

// Rebind moves the hotkey to accelerator while the app runs. The new combination is registered
// before the old one is dropped, so one that cannot be used (another program owns it, or it is not a
// valid combination) leaves the working one in place and the error says why.
func (p *Panel) Rebind(accelerator string) error {
	p.mu.Lock()
	old, oldWorks := p.hotkey, p.hotkeyErrMsg == ""
	p.mu.Unlock()
	if accelerator == old && oldWorks {
		return nil
	}
	if accelerator != old {
		if err := p.keys.Register(accelerator, p.Toggle); err != nil {
			return fmt.Errorf("the hotkey %s cannot be used: %w", accelerator, err)
		}
		if oldWorks {
			if err := p.keys.Unregister(old); err != nil {
				log.Printf("panel: the old hotkey %s could not be released: %v", old, err)
			}
		}
	} else if err := p.keys.Register(accelerator, p.Toggle); err != nil { // the same combination, tried again after it was taken
		return fmt.Errorf("the hotkey %s cannot be used: %w", accelerator, err)
	}
	p.mu.Lock()
	p.hotkey, p.hotkeyErrMsg = accelerator, ""
	p.mu.Unlock()
	log.Printf("panel: the hotkey is now %s", accelerator)
	if p.OnHotkeyError != nil {
		p.OnHotkeyError()
	}
	return nil
}

func (p *Panel) hwnd() uintptr { return uintptr(p.win.NativeWindow()) }

// Toggle opens the panel if it is closed and closes it if it is open.
func (p *Panel) Toggle() {
	if p.shown.Load() {
		p.Hide()
		return
	}
	p.Show()
}

// Show opens the panel on the monitor that holds the mouse cursor and takes the keyboard.
func (p *Panel) Show() {
	hwnd := p.hwnd()
	if hwnd == 0 || p.shown.Swap(true) {
		return
	}
	p.mu.Lock()
	if fg := foregroundWindow(); fg != hwnd {
		p.previous = fg
	}
	height := p.heightDIP
	p.mu.Unlock()

	work, dpi := cursorMonitor()
	moveTopmost(hwnd, place(work, dpi, height))
	cloak(hwnd, false)
	p.win.Focus()
}

// Hide closes the panel and gives the keyboard back to the app the user came from.
func (p *Panel) Hide() { p.hide(true) }

// HideForLaunch closes the panel after a file was handed to another program: that program has
// the keyboard now and must keep it.
func (p *Panel) HideForLaunch() { p.hide(false) }

// maxHeightDIP keeps the panel from growing past a comfortable list of results.
const maxHeightDIP = 640

// Resize makes the panel heightDIP tall (device-independent pixels), for the page to fit its
// results. The top edge stays where it is; the height never goes below the bare bar, above
// maxHeightDIP, or below the bottom of the screen.
func (p *Panel) Resize(heightDIP int) {
	heightDIP = max(barHeightDIP, min(heightDIP, maxHeightDIP))
	p.mu.Lock()
	p.heightDIP = heightDIP
	p.mu.Unlock()
	hwnd := p.hwnd()
	if hwnd == 0 || !p.shown.Load() {
		return
	}
	heightPx := int(float64(heightDIP) * float64(windowDPI(hwnd)) / 96)
	work, _ := cursorMonitor()
	if room := work.Y + work.H - windowRect(hwnd).Y; room > 0 {
		heightPx = min(heightPx, room)
	}
	resizeHeight(hwnd, heightPx)
}

func (p *Panel) hide(restoreFocus bool) {
	hwnd := p.hwnd()
	if hwnd == 0 || !p.shown.Swap(false) {
		return
	}
	cloak(hwnd, true)
	p.mu.Lock()
	previous := p.previous
	p.mu.Unlock()
	if restoreFocus && isWindow(previous) {
		setForeground(previous)
	}
}

// Bounds returns the window's position and size in physical pixels.
func (p *Panel) Bounds() Rect { return windowRect(p.hwnd()) }

// HasKeyboard reports whether the panel's window is the foreground window.
func (p *Panel) HasKeyboard() bool { return foregroundWindow() == p.hwnd() }

// Visible reports whether the panel is open.
func (p *Panel) Visible() bool { return p.shown.Load() }

// startupWatch is how long after creation the panel keeps taking the keyboard back from itself.
const startupWatch = 6 * time.Second

// releaseKeyboardAfterCreation gives the keyboard back to the app that had it before the launcher
// started. Windows activates a new window a moment after creating it, and the web view activates
// it again when it finishes initialising, so for the first few seconds every time the closed
// panel finds itself holding the keyboard it returns it.
func (p *Panel) releaseKeyboardAfterCreation(hwnd uintptr) {
	p.mu.Lock()
	previous := p.previous
	p.mu.Unlock()
	if !isWindow(previous) {
		return
	}
	for deadline := time.Now().Add(startupWatch); time.Now().Before(deadline); time.Sleep(40 * time.Millisecond) {
		if !p.shown.Load() && foregroundWindow() == hwnd {
			setForeground(previous) // the invisible window must not keep the keyboard
		}
	}
}
