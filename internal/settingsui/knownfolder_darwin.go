package settingsui

import (
	"fmt"
	"os"
	"path/filepath"
)

// guideFolders are the folders the setup guide offers, and what each is called inside the home folder on a Mac (the guide says
// "Videos"; a Mac says Movies).
var guideFolders = map[string]string{
	"Documents": "Documents",
	"Desktop":   "Desktop",
	"Pictures":  "Pictures",
	"Videos":    "Movies",
	"Downloads": "Downloads",
}

// knownFolder is where a user folder is: Documents, Pictures, Videos, Desktop or Downloads.
func knownFolder(name string) (string, error) {
	dir, ok := guideFolders[name]
	if !ok {
		return "", fmt.Errorf("no known folder %q", name)
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, dir), nil
}

// movedTo has nothing to find on a Mac: with Desktop & Documents in iCloud the folders stay where they were, as far as any program
// can see, so a configured folder is never the old place of one.
func movedTo(string) string { return "" }
