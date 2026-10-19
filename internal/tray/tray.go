// Package tray puts the launcher in the notification area, so there is always a way to see what
// it is doing and to quit it: the launcher has no taskbar button and no window while it is closed.
package tray

import (
	_ "embed"

	"github.com/wailsapp/wails/v3/pkg/application"
)

//go:embed icon.png
var icon []byte

// Tray is the notification-area icon and its menu.
type Tray struct {
	icon   *application.SystemTray
	menu   *application.Menu
	status *application.MenuItem
	pause  *application.MenuItem
	update *application.MenuItem
}

// New adds the icon. Clicking it calls open; the menu offers open, settings, rescan, pausing indexing and quit.
func New(app *application.App, open, settings, rescan, togglePause func()) *Tray {
	menu := application.NewMenu()
	status := menu.Add("Starting...")
	status.SetEnabled(false)
	menu.AddSeparator()
	menu.Add("Open search").OnClick(func(*application.Context) { open() })
	menu.Add("Settings").OnClick(func(*application.Context) { settings() })
	update := menu.Add("Update available")
	update.SetHidden(true)
	update.OnClick(func(*application.Context) { settings() })
	menu.Add("Rescan now").OnClick(func(*application.Context) { rescan() })
	pause := menu.Add("Pause indexing")
	pause.OnClick(func(*application.Context) { togglePause() })
	menu.AddSeparator()
	menu.Add("Quit").OnClick(func(*application.Context) { app.Quit() })

	icon := app.SystemTray.New().SetIcon(icon).SetMenu(menu).OnClick(open)
	icon.SetTooltip("AGI")
	return &Tray{icon: icon, menu: menu, status: status, pause: pause, update: update}
}

// SetPaused makes the pause item say what pressing it does. It may be called from any goroutine.
func (t *Tray) SetPaused(paused bool) {
	application.InvokeAsync(func() {
		if paused {
			t.pause.SetLabel("Resume indexing")
		} else {
			t.pause.SetLabel("Pause indexing")
		}
		t.menu.Update()
	})
}

// SetUpdate shows an "Update available" item for version, or hides it when version is empty. It opens
// Settings, where the About page has the link. It may be called from any goroutine.
func (t *Tray) SetUpdate(version string) {
	application.InvokeAsync(func() {
		t.update.SetHidden(version == "")
		t.update.SetLabel("Update available: " + version)
		t.menu.Update()
	})
}

// SetStatus shows line as the menu's first row and as the icon's tooltip. It may be called from
// any goroutine.
func (t *Tray) SetStatus(line string) {
	application.InvokeAsync(func() {
		t.status.SetLabel(line)
		t.menu.Update()
		t.icon.SetTooltip("AGI: " + line)
	})
}
