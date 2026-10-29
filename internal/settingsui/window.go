package settingsui

import (
	"fmt"
	"sync"

	"github.com/wailsapp/wails/v3/pkg/application"
	"github.com/wailsapp/wails/v3/pkg/events"

	"omni/internal/dock"
)

// windowSpec is what tells the settings window, the setup guide, the notes window, the chat and the timer apart: all are the
// same page host, each opened on its own route.
type windowSpec struct {
	name, title, route  string
	width, height       int
	minWidth, minHeight int
	// onTop keeps the window above other windows.
	onTop bool
	// card makes a window with no frame, title bar or buttons: a clear window whose page draws its own rounded card, edge,
	// close button and the part that drags it. It stays resizable (the page's edges do that).
	card bool
	// overlay makes a small window with no frame and no taskbar button that sits at the right edge of the screen: the
	// timer. The page draws its own rounded outline on the clear window and says which part drags it.
	overlay bool
}

var (
	settingsSpec = windowSpec{name: "settings", title: "Omni Settings", route: "settings", width: 860, height: 620, minWidth: 680, minHeight: 480, card: true}
	setupSpec    = windowSpec{name: "setup", title: "Set up Omni", route: "setup", width: 760, height: 700, minWidth: 640, minHeight: 560}
	// The notes and the chat are as wide as the search bar (720) and shorter than they are wide.
	notesSpec = windowSpec{name: "notes", title: "Omni Notes", route: "notes", width: 720, height: 520, minWidth: 480, minHeight: 340, onTop: true, card: true}
	chatSpec  = windowSpec{name: "chat", title: "Omni AI", route: "chat", width: 720, height: 540, minWidth: 520, minHeight: 380, onTop: true, card: true}
	timerSpec = windowSpec{name: "timer", title: "Omni Timer", route: "timer", width: 264, height: 92, onTop: true, overlay: true}
)

// overlayMargin is the gap between the timer and the right edge of the screen, and its top is a fifth of the way down.
const overlayMargin = 16

// Window opens one of the program's windows. It is built the first time it is asked for, so the hotkey path never
// pays for it, and a second request brings the open one forward. The lock only guards which window is open: calls into Wails
// are made without it, because a window closing runs a handler that takes the lock too.
type Window struct {
	app  *application.App
	spec windowSpec
	// onTop, when set, decides whether the window is above other windows each time it is built; the spec's choice otherwise.
	onTop func() bool

	build sync.Mutex // one window is made at a time, so two requests at once cannot make two

	mu    sync.Mutex
	win   *application.WebviewWindow
	count int // Wails wants every window name to be new
}

// NewWindow returns a Window for app.
func NewWindow(app *application.App) *Window { return &Window{app: app, spec: settingsSpec} }

// NewSetupWindow returns the Window of the first-run guide.
func NewSetupWindow(app *application.App) *Window { return &Window{app: app, spec: setupSpec} }

// NewNotesWindow returns the Window of the notes: small, resizable and above other windows until the person says otherwise.
func NewNotesWindow(app *application.App) *Window { return &Window{app: app, spec: notesSpec} }

// NewChatWindow returns the Window of Omni AI: resizable and above other windows until the person says otherwise.
func NewChatWindow(app *application.App, onTop func() bool) *Window {
	return &Window{app: app, spec: chatSpec, onTop: onTop}
}

// NewTimerWindow returns the Window of the timer overlay.
func NewTimerWindow(app *application.App) *Window { return &Window{app: app, spec: timerSpec} }

func (w *Window) current() *application.WebviewWindow {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.win
}

// Open shows the window.
func (w *Window) Open() { w.OpenAt("") }

// OpenAt shows the window on the section (or note) with this id, the first one if it is empty. An open window switches
// to it through its address (the page listens for the change), so a page opened from the bar is never the wrong one.
func (w *Window) OpenAt(section string) {
	w.build.Lock()
	defer w.build.Unlock()
	if win := w.current(); win != nil {
		win.Show()
		win.UnMinimise()
		if !w.spec.overlay {
			win.Focus()
		}
		if section != "" {
			win.ExecJS(fmt.Sprintf("location.hash = %q", "#"+w.spec.route+"/"+section))
		}
		return
	}
	w.mu.Lock()
	w.count++
	name := fmt.Sprintf("%s-%d", w.spec.name, w.count)
	w.mu.Unlock()
	url := "/#" + w.spec.route
	if section != "" {
		url += "/" + section
	}
	options := application.WebviewWindowOptions{
		Name:             name,
		Title:            w.spec.title,
		Width:            w.spec.width,
		Height:           w.spec.height,
		MinWidth:         w.spec.minWidth,
		MinHeight:        w.spec.minHeight,
		AlwaysOnTop:      w.spec.onTop && (w.onTop == nil || w.onTop()),
		URL:              url,
		BackgroundColour: application.NewRGBA(0, 0, 0, 255),
	}
	if w.spec.card {
		options.Frameless = true
		options.BackgroundType = application.BackgroundTypeTransparent
		options.BackgroundColour = application.NewRGBA(0, 0, 0, 0)
	}
	if w.spec.overlay {
		options.Frameless = true
		options.DisableResize = true
		options.BackgroundType = application.BackgroundTypeTransparent
		options.BackgroundColour = application.NewRGBA(0, 0, 0, 0)
		if screen := w.app.Screen.GetPrimary(); screen != nil {
			options.Screen = screen
			options.InitialPosition = application.WindowXY
			options.X = screen.WorkArea.Width - w.spec.width - overlayMargin
			options.Y = screen.WorkArea.Height / 5
		}
	}
	platformWindow(&options, w.spec)
	win := w.app.Window.NewWithOptions(options)
	if !w.spec.overlay { // the timer is a small overlay, not a window to switch to
		dock.Counter.Opened()
	}
	win.OnWindowEvent(events.Common.WindowClosing, func(*application.WindowEvent) {
		if !w.spec.overlay {
			dock.Counter.Closed()
		}
		w.mu.Lock()
		if w.win == win { // not a newer window that replaced this one
			w.win = nil
		}
		w.mu.Unlock()
	})
	w.mu.Lock()
	w.win = win
	w.mu.Unlock()
	win.Show()
	if !w.spec.overlay {
		win.Focus() // the timer must not take the keyboard from whatever is being worked in
	}
}

// Close closes the window if it is open. It is forgotten at once, so opening again straight after builds a new one instead of
// reusing the one that is closing.
func (w *Window) Close() {
	w.mu.Lock()
	win := w.win
	w.win = nil
	w.mu.Unlock()
	if win != nil {
		win.Close()
	}
}

// Toggle is what a hotkey does: it closes the window when it is the one in use, and opens it otherwise.
func (w *Window) Toggle() {
	if win := w.current(); win != nil && win.IsFocused() {
		w.Close()
		return
	}
	w.Open()
}

// SetAlwaysOnTop keeps the window above other windows, or lets them cover it. Without an open window it does nothing.
func (w *Window) SetAlwaysOnTop(on bool) {
	if win := w.current(); win != nil {
		win.SetAlwaysOnTop(on)
	}
}
