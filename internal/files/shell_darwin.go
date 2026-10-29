package files

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

// macShell is the real opener on a Mac: `open` does what a double-click in Finder does.
type macShell struct{}

func newShell() opener { return macShell{} }

func (macShell) Default(path string) error { return start(exec.Command("open", path)) }

// Editor opens the file in VS Code at the line when it is installed, and in the default text editor when not (TextEdit unless the
// person chose another), as Notepad is the fallback on Windows.
func (macShell) Editor(path string, line int) error {
	if code := visualStudioCode(); code != "" {
		target := path
		if line > 0 {
			target = fmt.Sprintf("%s:%d", path, line)
		}
		return start(exec.Command(code, "-g", target))
	}
	return start(exec.Command("open", "-t", path))
}

// Reveal shows the file selected in a Finder window.
func (macShell) Reveal(path string) error { return start(exec.Command("open", "-R", path)) }

// visualStudioCode returns the `code` command if it is installed: on PATH (the person ran "Install 'code' command in PATH"), in
// Homebrew's folders, or inside the app itself.
func visualStudioCode() string {
	if found, err := exec.LookPath("code"); err == nil {
		return found
	}
	candidates := []string{
		"/opt/homebrew/bin/code", "/usr/local/bin/code",
		"/Applications/Visual Studio Code.app/Contents/Resources/app/bin/code",
	}
	if home, err := os.UserHomeDir(); err == nil {
		candidates = append(candidates, filepath.Join(home, "Applications", "Visual Studio Code.app", "Contents", "Resources", "app", "bin", "code"))
	}
	for _, candidate := range candidates {
		if info, err := os.Stat(candidate); err == nil && !info.IsDir() {
			return candidate
		}
	}
	return ""
}
