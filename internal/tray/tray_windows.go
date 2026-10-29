package tray

import (
	_ "embed"

	"github.com/wailsapp/wails/v3/pkg/application"
)

//go:embed icon.png
var icon []byte

// setIcon gives the tray the coloured icon.
func setIcon(t *application.SystemTray) *application.SystemTray { return t.SetIcon(icon) }
