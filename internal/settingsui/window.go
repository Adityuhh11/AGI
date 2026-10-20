package settingsui

import (
	"fmt"
	"sync"

	"github.com/wailsapp/wails/v3/pkg/application"
	"github.com/wailsapp/wails/v3/pkg/events"
)

// Window opens the settings window. It is built the first time it is asked for, so the hotkey path
// never pays for it, and a second request brings the open one forward.
type Window struct {
	app *application.App

	mu    sync.Mutex
	win   *application.WebviewWindow
	count int // Wails wants every window name to be new
}

// NewWindow returns a Window for app.
func NewWindow(app *application.App) *Window { return &Window{app: app} }

// Open shows the settings window.
func (w *Window) Open() { w.OpenAt("") }

// OpenAt shows the settings window; a new window starts on the section with this id ("about"), the first one if it is empty.
func (w *Window) OpenAt(section string) {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.win != nil {
		w.win.Show()
		w.win.UnMinimise()
		w.win.Focus()
		return
	}
	w.count++
	url := "/#settings"
	if section != "" {
		url += "/" + section
	}
	win := w.app.Window.NewWithOptions(application.WebviewWindowOptions{
		Name:             fmt.Sprintf("settings-%d", w.count),
		Title:            "AGI Settings",
		Width:            860,
		Height:           620,
		MinWidth:         680,
		MinHeight:        480,
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
