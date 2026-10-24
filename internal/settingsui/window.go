package settingsui

import (
	"fmt"
	"sync"

	"github.com/wailsapp/wails/v3/pkg/application"
	"github.com/wailsapp/wails/v3/pkg/events"
)

// windowSpec is what tells the settings window from the setup guide: both are the same page host.
type windowSpec struct {
	name, title, route  string
	width, height       int
	minWidth, minHeight int
}

var (
	settingsSpec = windowSpec{"settings", "AGI Settings", "settings", 860, 620, 680, 480}
	setupSpec    = windowSpec{"setup", "Set up AGI", "setup", 760, 700, 640, 560}
)

// Window opens the settings window or the setup guide. It is built the first time it is asked for, so the hotkey
// path never pays for it, and a second request brings the open one forward.
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

// Open shows the settings window.
func (w *Window) Open() { w.OpenAt("") }

// OpenAt shows the settings window on the section with this id ("about"), the first one if it is empty. An open window
// switches to it through its address (the page listens for the change), so a page opened from the bar is never the wrong one.
func (w *Window) OpenAt(section string) {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.win != nil {
		w.win.Show()
		w.win.UnMinimise()
		w.win.Focus()
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
	win := w.app.Window.NewWithOptions(application.WebviewWindowOptions{
		Name:             fmt.Sprintf("%s-%d", w.spec.name, w.count),
		Title:            w.spec.title,
		Width:            w.spec.width,
		Height:           w.spec.height,
		MinWidth:         w.spec.minWidth,
		MinHeight:        w.spec.minHeight,
		URL:              url,
		BackgroundColour: application.NewRGBA(0, 0, 0, 255),
		Windows:          application.WindowsWindow{Theme: application.Dark},
	})
	win.OnWindowEvent(events.Common.WindowClosing, func(*application.WindowEvent) {
		w.mu.Lock()
		w.win = nil
		w.mu.Unlock()
	})
	w.win = win
	win.Show()
	win.Focus()
}
