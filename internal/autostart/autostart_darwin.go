package autostart

import (
	"fmt"

	"github.com/wailsapp/wails/v3/pkg/application"
)

// Enabled reports whether signing in starts the launcher. Wails registers a login item with SMAppService when it runs from
// Omni.app (macOS 13 and later), and a LaunchAgent when it does not (a developer's build). A login item the person
// switched off in System Settings counts as off.
func Enabled() bool {
	status, err := application.Get().Autostart.Status()
	return err == nil && status.Enabled
}

// Set turns starting at sign-in on or off.
func Set(on bool) error {
	manager := application.Get().Autostart
	if on {
		if err := manager.Enable(); err != nil {
			return fmt.Errorf("add the login item: %w", err)
		}
		return nil
	}
	if err := manager.Disable(); err != nil {
		return fmt.Errorf("remove the login item: %w", err)
	}
	return nil
}
