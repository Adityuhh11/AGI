package tray

import (
	_ "embed"

	"github.com/wailsapp/wails/v3/pkg/application"
)

// templateIcon is black with the ring's transparency. A Mac menu bar icon is a template image: the system colours it for a light or dark
// menu bar and for the pressed state, so it must carry shape and nothing else.
//
//go:embed icon-template.png
var templateIcon []byte

// setIcon gives the tray the template icon.
func setIcon(t *application.SystemTray) *application.SystemTray { return t.SetTemplateIcon(templateIcon) }
