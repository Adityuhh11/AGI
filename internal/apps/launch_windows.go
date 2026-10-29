package apps

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"golang.org/x/sys/windows"
)

// CanRunAsAdministrator says that this system has such an action for an app, which the bar then offers.
const CanRunAsAdministrator = true

// knownFolder says where a Windows known folder is, given its id in braces; tests replace it.
var knownFolder = func(id string) (string, error) {
	guid, err := windows.GUIDFromString(id)
	if err != nil {
		return "", err
	}
	return windows.KnownFolderPath((*windows.KNOWNFOLDERID)(&guid), 0)
}

// target is how to start an app: what to hand to the shell, and the file it comes from when it comes from one.
type target struct {
	run  string
	file string
}

// resolve works out how to start the entry with this id. Windows names Start Menu entries four ways:
//   - a launcher link (`steam://rungameid/1`): handed to the shell as it is;
//   - a path under a known folder (`{Program Files id}\7-Zip\7zFM.exe`): its file is that folder plus the
//     rest, so it can be checked, and the shell starts it through the Start Menu, as Windows does;
//   - an absolute path: started as it is;
//   - an AppUserModelID (Store apps and most desktop programs): started through the Start Menu.
func resolve(id string) target {
	switch {
	case strings.Contains(id, "://"):
		return target{run: id}
	case strings.HasPrefix(id, "{") && strings.Contains(id, `\`):
		guid, rest, _ := strings.Cut(id, `\`)
		if folder, err := knownFolder(guid); err == nil {
			return target{run: `shell:AppsFolder\` + id, file: filepath.Join(folder, rest)}
		}
		return target{run: `shell:AppsFolder\` + id}
	case strings.Contains(id, `\`):
		return target{run: id, file: id}
	}
	return target{run: `shell:AppsFolder\` + id}
}

// Launch brings the app to the front if it is already open, and starts it otherwise. It checks first that
// the file behind it still exists (an uninstalled program would otherwise raise a Windows error dialog), then
// hands it to the shell without waiting: the shell call can block for seconds on a classic program (2.1 s for
// 7-Zip), and the bar must close at once.
func Launch(app App) error {
	t := resolve(app.ID)
	if t.file != "" {
		if _, err := os.Stat(t.file); err != nil {
			return fmt.Errorf("%s: %w", app.Name, ErrGone)
		}
	}
	if focusRunning(app, t.file) {
		return nil
	}
	go func() {
		if err := shellExecute("open", t.run); err != nil {
			log.Printf("apps: could not start %s: %v", app.Name, err)
		}
	}()
	return nil
}

// withoutDead drops the entries whose file is gone: the Start Menu keeps shortcuts to programs that were
// uninstalled (two on the machine this was written on). Entries that do not name a file are kept.
func withoutDead(apps []App) []App {
	alive := apps[:0:0]
	for _, app := range apps {
		if file := resolve(app.ID).file; file != "" {
			if _, err := os.Stat(file); err != nil {
				continue
			}
		}
		alive = append(alive, app)
	}
	return alive
}

// Path is the file an app starts from, when it starts from one (many Store apps and plain-id desktop
// programs do not say).
func Path(app App) (string, bool) {
	file := resolve(app.ID).file
	return file, file != ""
}

// RunAsAdministrator starts the app elevated; Windows asks the user to confirm.
func RunAsAdministrator(app App) error {
	file, ok := Path(app)
	if !ok {
		return fmt.Errorf("%s does not say which file it starts, so it cannot be elevated from here", app.Name)
	}
	if _, err := os.Stat(file); err != nil {
		return fmt.Errorf("%s: %w", app.Name, ErrGone)
	}
	go func() {
		if err := shellExecute("runas", file); err != nil {
			log.Printf("apps: could not start %s as administrator: %v", app.Name, err)
		}
	}()
	return nil
}

// RevealInFileManager opens the folder of the app's file in Explorer with the file selected.
func RevealInFileManager(app App) error {
	file, ok := Path(app)
	if !ok {
		return fmt.Errorf("%s does not say which file it starts", app.Name)
	}
	cmd := exec.Command("explorer.exe", "/select,"+file)
	if err := cmd.Start(); err != nil {
		return err
	}
	go func() { _ = cmd.Wait() }()
	return nil
}

func shellExecute(verb, target string) error {
	v, err := windows.UTF16PtrFromString(verb)
	if err != nil {
		return err
	}
	t, err := windows.UTF16PtrFromString(target)
	if err != nil {
		return err
	}
	return windows.ShellExecute(0, v, t, nil, nil, windows.SW_SHOWNORMAL)
}
