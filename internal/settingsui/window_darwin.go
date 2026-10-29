package settingsui

import "github.com/wailsapp/wails/v3/pkg/application"

// platformWindow adds what only a Mac needs to a window's options: the windows the page draws itself (the cards and the timer) are on a clear
// window, which a Mac only gives when asked, or there is an opaque plate behind the page's rounded edge.
func platformWindow(options *application.WebviewWindowOptions, spec windowSpec) {
	if spec.card || spec.overlay {
		options.Mac = application.MacWindow{Backdrop: application.MacBackdropTransparent}
	}
}
