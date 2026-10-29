// Package testpath is for tests only. Many tests were written with Windows paths (`C:\Users\me\Docs`); P keeps them readable and makes
// them paths of the system the test runs on: unchanged on Windows, and on a Mac the same path under a folder named for the drive
// (`/C/Users/me/Docs`), so two drives stay two different places and the separators are the system's own.
package testpath

import (
	"runtime"
	"strings"
)

// P is path as a path of this system.
func P(path string) string {
	if runtime.GOOS == "windows" {
		return path
	}
	drive := ""
	if len(path) >= 2 && path[1] == ':' {
		drive, path = "/"+path[:1], path[2:]
	}
	return drive + strings.ReplaceAll(path, `\`, "/")
}
