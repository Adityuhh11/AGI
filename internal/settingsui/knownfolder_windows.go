package settingsui

import (
	"os"
	"path/filepath"
	"strings"

	"golang.org/x/sys/windows"
)

// userFolders are the folders Windows can move out of the profile, usually into OneDrive when its backup is on.
var userFolders = map[string]*windows.KNOWNFOLDERID{
	"Documents": windows.FOLDERID_Documents,
	"Desktop":   windows.FOLDERID_Desktop,
	"Pictures":  windows.FOLDERID_Pictures,
	"Videos":    windows.FOLDERID_Videos,
	"Music":     windows.FOLDERID_Music,
}

// movedTo is where Windows keeps the user folder that root is the old place of (C:\Users\me\Documents, after OneDrive
// moved Documents into C:\Users\me\OneDrive\Documents); "" when root is not such a folder or it was not moved.
func movedTo(root string) string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return moved(root, home, func(name string) (string, error) {
		return windows.KnownFolderPath(userFolders[name], 0)
	})
}

// moved is movedTo with the Windows lookup passed in: where the user folder that root names now lives, or "".
func moved(root, home string, lookup func(name string) (string, error)) string {
	for name := range userFolders {
		old := filepath.Join(home, name)
		if !strings.EqualFold(filepath.Clean(root), old) {
			continue
		}
		now, err := lookup(name)
		if err != nil || strings.EqualFold(filepath.Clean(now), old) {
			return ""
		}
		return now
	}
	return ""
}
