package main

import (
	"os/exec"

	"github.com/wailsapp/wails/v3/pkg/application"

	"omni/internal/settingsui"
)

// platformOptions adds what only Windows needs to the application's options.
func platformOptions(options *application.Options) {
	// The page never sees Alt+Space (Windows takes it as the system-menu key), so the settings'
	// hotkey field hears about it from here. Emitting from a goroutine keeps the window procedure free.
	options.Windows = application.WindowsOptions{WndProcInterceptor: settingsui.KeyMenuFilter(func(accelerator string) {
		go application.Get().Event.Emit("hotkey:alt-space", accelerator)
	})}
}

// runHelper runs the program as one of its own helpers when it was started as one, and says so. Windows has none: its helpers are
// kernel objects (the Job Object that ends the engine with the launcher).
func runHelper() bool { return false }

// openFolder shows a folder in Explorer.
func openFolder(path string) error { return exec.Command("explorer.exe", path).Start() }

// platformSetup does what only Windows needs once the application exists. Nothing: its windows carry their own menus, and the program has no application menu.
func platformSetup(*application.App) {}
