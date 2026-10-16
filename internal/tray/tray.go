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
}

// New adds the icon. Clicking it calls open; the menu offers open, rescan and quit.
func New(app *application.App, open, rescan func()) *Tray {
	menu := application.NewMenu()
	status := menu.Add("Starting...")
	status.SetEnabled(false)
	menu.AddSeparator()
	menu.Add("Open search").OnClick(func(*application.Context) { open() })
	menu.Add("Rescan now").OnClick(func(*application.Context) { rescan() })
	menu.AddSeparator()
	menu.Add("Quit").OnClick(func(*application.Context) { app.Quit() })

	icon := app.SystemTray.New().SetIcon(icon).SetMenu(menu).OnClick(open)
	icon.SetTooltip("AGI")
	return &Tray{icon: icon, menu: menu, status: status}
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
