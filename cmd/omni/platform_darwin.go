package main

import (
	"os"
	"os/exec"

	"github.com/wailsapp/wails/v3/pkg/application"

	"omni/internal/sidecar"
)

// platformOptions adds what only a Mac needs to the application's options.
func platformOptions(options *application.Options) {
	options.Mac = application.MacOptions{
		// No Dock icon and no menu bar while only the bar is open; the windows that need them ask for them (milestone M1).
		ActivationPolicy: application.ActivationPolicyAccessory,
		// Closing Settings must not end the launcher: it lives in the menu bar.
		ApplicationShouldTerminateAfterLastWindowClosed: false,
	}
}

// runHelper runs the program as the reaper when it was started as one (internal/sidecar/reaper_darwin.go), and says so.
func runHelper() bool {
	if len(os.Args) == 2 && os.Args[1] == sidecar.ReaperFlag {
		sidecar.RunReaper(os.Stdin)
		return true
	}
	return false
}

// openFolder shows a folder in Finder.
func openFolder(path string) error { return exec.Command("open", path).Start() }
