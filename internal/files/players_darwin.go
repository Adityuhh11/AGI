package files

import (
	"os"
	"os/exec"
	"path/filepath"
)

// knownPlayer is a player the launcher knows how to ask for a moment on a Mac, and where its program is, best first. Their command
// lines are the ones the players document (players.go); IINA's `iina-cli` passes `--mpv-start` to mpv.
type knownPlayer struct {
	name  string
	kind  playerKind
	exes  []string // program names looked for on PATH (Homebrew puts mpv there)
	paths []string // inside an application folder: "VLC.app/Contents/MacOS/VLC"
}

var knownPlayers = []knownPlayer{
	{"VLC", vlc, []string{"vlc"}, []string{"VLC.app/Contents/MacOS/VLC"}},
	{"IINA", iina, nil, []string{"IINA.app/Contents/MacOS/iina-cli"}},
	{"mpv", mpv, []string{"mpv"}, []string{"mpv.app/Contents/MacOS/mpv"}},
}

// SeekPlayer finds the first known player that is installed.
func (macShell) SeekPlayer() (Player, bool) {
	folders := []string{"/Applications"}
	if home, err := os.UserHomeDir(); err == nil {
		folders = append(folders, filepath.Join(home, "Applications"))
	}
	return findPlayer(exec.LookPath, fileExists, append(folders, "/opt/homebrew/bin", "/usr/local/bin"))
}

// Player starts the player on path at a moment, and lets it go like an editor.
func (macShell) Player(p Player, path string, seconds float64) error {
	return start(exec.Command(p.Exe, p.args(path, seconds)...))
}

// findPlayer looks for each known player in turn on PATH and in the application folders. The lookups are arguments so a test
// can stand in for the machine.
func findPlayer(lookPath func(string) (string, error), exists func(string) bool, folders []string) (Player, bool) {
	for _, known := range knownPlayers {
		for _, exe := range known.exes {
			if found, err := lookPath(exe); err == nil {
				return Player{Name: known.name, Exe: found, kind: known.kind}, true
			}
		}
		for _, folder := range folders {
			for _, rel := range known.paths {
				if candidate := filepath.Join(folder, rel); exists(candidate) {
					return Player{Name: known.name, Exe: candidate, kind: known.kind}, true
				}
			}
			for _, exe := range known.exes { // Homebrew's folders hold the program itself
				if candidate := filepath.Join(folder, exe); exists(candidate) {
					return Player{Name: known.name, Exe: candidate, kind: known.kind}, true
				}
			}
		}
	}
	return Player{}, false
}

func fileExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}
