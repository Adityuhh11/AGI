package apps

import (
	"context"
	"os"
	"path/filepath"
	"strings"
)

// applicationFolders are where a Mac keeps applications: the ones the person installed, the ones in Utilities, Apple's own
// (kept apart from /Applications since macOS 10.15), and the person's own ~/Applications.
func applicationFolders() []string {
	folders := []string{"/Applications", "/Applications/Utilities", "/System/Applications", "/System/Applications/Utilities"}
	if home, err := os.UserHomeDir(); err == nil {
		folders = append(folders, filepath.Join(home, "Applications"))
	}
	return folders
}

// finder is the one app of Apple's that does not live in an applications folder.
const finder = "/System/Library/CoreServices/Finder.app"

// WatchedFolders are the application folders that exist. A program installed or removed changes one of them.
func WatchedFolders() []string {
	var found []string
	for _, folder := range applicationFolders() {
		if info, err := os.Stat(folder); err == nil && info.IsDir() {
			found = append(found, folder)
		}
	}
	return found
}

// list is the application bundles of the folders above. It is a directory read, a few milliseconds, so it needs none of
// the caching tricks Windows' PowerShell call does (the cache file only serves the first keystroke).
func list(context.Context) ([]App, error) {
	return scan(applicationFolders(), finder), nil
}

// scan returns the .app bundles directly in the folders, and in one level of folders inside them (vendors keep their apps in
// a folder of their own: "Adobe Photoshop 2025/Adobe Photoshop 2025.app"), plus the extra bundles that exist. An app is named as Finder names it:
// the bundle's name without ".app". (ponytail: a localized display name from Info.plist can come later; it needs a plist reader.)
func scan(folders []string, extra ...string) []App {
	seen := map[string]bool{}
	var found []App
	add := func(path string) {
		if seen[path] {
			return
		}
		seen[path] = true
		found = append(found, App{ID: path, Name: strings.TrimSuffix(filepath.Base(path), ".app")})
	}
	for _, folder := range folders {
		entries, err := os.ReadDir(folder)
		if err != nil {
			continue // a folder that is not there (no ~/Applications) is not an error
		}
		for _, entry := range entries {
			path := filepath.Join(folder, entry.Name())
			switch {
			case strings.HasSuffix(entry.Name(), ".app"):
				add(path)
			case entry.IsDir() && !strings.HasPrefix(entry.Name(), "."):
				inner, err := os.ReadDir(path)
				if err != nil {
					continue
				}
				for _, app := range inner {
					if strings.HasSuffix(app.Name(), ".app") {
						add(filepath.Join(path, app.Name()))
					}
				}
			}
		}
	}
	for _, path := range extra {
		if _, err := os.Stat(path); err == nil {
			add(path)
		}
	}
	return found
}
