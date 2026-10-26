package settingsui

import (
	"fmt"
	"sync"

	"github.com/wailsapp/wails/v3/pkg/application"
	"github.com/wailsapp/wails/v3/pkg/events"
)

// windowSpec is what tells the settings window, the setup guide, the notes window and the timer apart: all four are the
// same page host, each opened on its own route.
type windowSpec struct {
	name, title, route  string
	width, height       int
	minWidth, minHeight int
	// onTop keeps the window above other windows.
	onTop bool
	// overlay makes a small window with no frame and no taskbar button that sits at the right edge of the screen: the
	// timer. The page draws its own rounded outline on the clear window and says which part drags it.
	overlay bool
}

var (
	settingsSpec = windowSpec{name: "settings", title: "AGI Settings", route: "settings", width: 860, height: 620, minWidth: 680, minHeight: 480}
	setupSpec    = windowSpec{name: "setup", title: "Set up AGI", route: "setup", width: 760, height: 700, minWidth: 640, minHeight: 560}
	notesSpec    = windowSpec{name: "notes", title: "AGI Notes", route: "notes", width: 420, height: 560, minWidth: 300, minHeight: 280, onTop: true}
	timerSpec    = windowSpec{name: "timer", title: "AGI Timer", route: "timer", width: 264, height: 92, onTop: true, overlay: true}
)

// overlayMargin is the gap between the timer and the right edge of the screen, and its top is a fifth of the way down.
const overlayMargin = 16

// Window opens one of the program's windows. It is built the first time it is asked for, so the hotkey path never
// pays for it, and a second request brings the open one forward.
type Window struct {
	app  *application.App
	spec windowSpec

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

// NewTimerWindow returns the Window of the timer overlay.
func NewTimerWindow(app *application.App) *Window { return &Window{app: app, spec: timerSpec} }

// Open shows the window.
func (w *Window) Open() { w.OpenAt("") }

// OpenAt shows the window on the section (or note) with this id, the first one if it is empty. An open window switches
// to it through its address (the page listens for the change), so a page opened from the bar is never the wrong one.
func (w *Window) OpenAt(section string) {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.win != nil {
		w.win.Show()
		w.win.UnMinimise()
		if !w.spec.overlay {
			w.win.Focus()
		}
		if section != "" {
			w.win.ExecJS(fmt.Sprintf("location.hash = %q", "#"+w.spec.route+"/"+section))
		}
		return
	}
	w.count++
	url := "/#" + w.spec.route
	if section != "" {
		url += "/" + section
	}
	options := application.WebviewWindowOptions{
		Name:             fmt.Sprintf("%s-%d", w.spec.name, w.count),
		Title:            w.spec.title,
		Width:            w.spec.width,
		Height:           w.spec.height,
		MinWidth:         w.spec.minWidth,
		MinHeight:        w.spec.minHeight,
		AlwaysOnTop:      w.spec.onTop,
		URL:              url,
		BackgroundColour: application.NewRGBA(0, 0, 0, 255),
		Windows:          application.WindowsWindow{Theme: application.Dark},
	}
	if w.spec.overlay {
		options.Frameless = true
		options.DisableResize = true
		options.BackgroundType = application.BackgroundTypeTransparent
		options.BackgroundColour = application.NewRGBA(0, 0, 0, 0)
		options.Windows = application.WindowsWindow{HiddenOnTaskbar: true, DisableFramelessWindowDecorations: true}
		if screen := w.app.Screen.GetPrimary(); screen != nil {
			options.Screen = screen
			options.InitialPosition = application.WindowXY
			options.X = screen.WorkArea.Width - w.spec.width - overlayMargin
			options.Y = screen.WorkArea.Height / 5
		}
	}
	win := w.app.Window.NewWithOptions(options)
	win.OnWindowEvent(events.Common.WindowClosing, func(*application.WindowEvent) {
		w.mu.Lock()
		w.win = nil
		w.mu.Unlock()
	})
	w.win = win
	win.Show()
	if !w.spec.overlay {
		win.Focus() // the timer must not take the keyboard from whatever is being worked in
	}
}

// Close closes the window if it is open.
func (w *Window) Close() {
	w.mu.Lock()
	win := w.win
	w.mu.Unlock()
	if win != nil {
		win.Close()
	}
}

// SetAlwaysOnTop keeps the window above other windows, or lets them cover it. Without an open window it does nothing.
func (w *Window) SetAlwaysOnTop(on bool) {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.win != nil {
		w.win.SetAlwaysOnTop(on)
	}
}
