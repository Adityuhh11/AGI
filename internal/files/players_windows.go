package files

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"golang.org/x/sys/windows/registry"
)

// SeekPlayer finds the first known player that is installed.
func (windowsShell) SeekPlayer() (Player, bool) {
	return findPlayer(exec.LookPath, appPath, fileExists, programFolders())
}

// Player starts the player on path at a moment, and lets it go like an editor.
func (windowsShell) Player(p Player, path string, seconds float64) error {
	return start(exec.Command(p.Exe, p.args(path, seconds)...))
}

// findPlayer looks for each known player in turn on PATH, in the registry's App Paths (where installers register a
// program) and in the usual install folders. The lookups are arguments so a test can stand in for the machine.
func findPlayer(lookPath func(string) (string, error), registered func(string) string, exists func(string) bool, folders []string) (Player, bool) {
	for _, known := range knownPlayers {
		for _, exe := range known.exes {
			if found, err := lookPath(exe); err == nil {
				return Player{Name: known.name, Exe: found, kind: known.kind}, true
			}
			if found := registered(exe); found != "" && exists(found) {
				return Player{Name: known.name, Exe: found, kind: known.kind}, true
			}
		}
		for _, folder := range folders {
			for _, rel := range known.paths {
				if candidate := filepath.Join(folder, rel); exists(candidate) {
					return Player{Name: known.name, Exe: candidate, kind: known.kind}, true
				}
			}
		}
	}
	return Player{}, false
}

// programFolders are where a program is installed for everyone or for one user.
func programFolders() []string {
	var folders []string
	for _, env := range []string{"ProgramFiles", "ProgramFiles(x86)", "LOCALAPPDATA"} {
		if dir := os.Getenv(env); dir != "" {
			folders = append(folders, dir)
		}
	}
	return folders
}

// appPath reads the path an installer registered for a program name, or "" if none did.
func appPath(exe string) string {
	for _, root := range []registry.Key{registry.CURRENT_USER, registry.LOCAL_MACHINE} {
		key, err := registry.OpenKey(root, `SOFTWARE\Microsoft\Windows\CurrentVersion\App Paths\`+exe, registry.QUERY_VALUE)
		if err != nil {
			continue
		}
		value, _, err := key.GetStringValue("")
		key.Close()
		if value = strings.Trim(value, `"`); err == nil && value != "" { // some installers quote it
			return value
		}
	}
	return ""
}

func fileExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}
