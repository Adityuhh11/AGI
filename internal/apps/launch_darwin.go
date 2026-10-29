package apps

import (
	"errors"
	"fmt"
	"log"
	"os"
	"os/exec"
)

// CanRunAsAdministrator says that this system has such an action for an app, which the bar then offers. macOS has none:
// an app asks for administrator rights itself when it needs them.
const CanRunAsAdministrator = false

// Launch brings the app to the front if it is already open, and starts it otherwise: `open` does both. It checks first that the
// bundle is still there, and does not wait for the app.
func Launch(app App) error {
	if _, err := os.Stat(app.ID); err != nil {
		return fmt.Errorf("%s: %w", app.Name, ErrGone)
	}
	return start(app, "open", app.ID)
}

// Path is the bundle of the app; every app on a Mac starts from one.
func Path(app App) (string, bool) { return app.ID, true }

// RunAsAdministrator is not something macOS does; the bar does not offer it (CanRunAsAdministrator).
func RunAsAdministrator(App) error {
	return errors.New("macOS has no run as administrator for apps")
}

// RevealInFileManager shows the app's bundle selected in a Finder window.
func RevealInFileManager(app App) error {
	if _, err := os.Stat(app.ID); err != nil {
		return fmt.Errorf("%s: %w", app.Name, ErrGone)
	}
	return start(app, "open", "-R", app.ID)
}

// withoutDead drops the entries whose bundle is gone (an app dragged to the Trash since the list was made).
func withoutDead(apps []App) []App {
	alive := apps[:0:0]
	for _, app := range apps {
		if _, err := os.Stat(app.ID); err == nil {
			alive = append(alive, app)
		}
	}
	return alive
}

// start runs a command and lets it go; the bar must close at once.
func start(app App, name string, args ...string) error {
	cmd := exec.Command(name, args...)
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("start %s: %w", app.Name, err)
	}
	go func() {
		if err := cmd.Wait(); err != nil {
			log.Printf("apps: %s %v: %v", name, args, err)
		}
	}()
	return nil
}
