package settingsui

import "github.com/wailsapp/wails/v3/pkg/application"

// platformWindow adds what only Windows needs to a window's options: a dark title bar for a window that has one, and no frame decorations for the
// ones the page draws itself (the cards and the timer, which also has no taskbar button).
func platformWindow(options *application.WebviewWindowOptions, spec windowSpec) {
	options.Windows = application.WindowsWindow{Theme: application.Dark}
	if spec.card {
		options.Windows = application.WindowsWindow{DisableFramelessWindowDecorations: true}
	}
	if spec.overlay {
		options.Windows = application.WindowsWindow{HiddenOnTaskbar: true, DisableFramelessWindowDecorations: true}
	}
}
