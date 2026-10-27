package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"omni/internal/config"
)

// removeDataFlag is what the uninstaller runs the program with when the person also wants Omni's data gone:
// "omni.exe --remove-data" deletes what the program keeps for this user and ends. It runs before the log is opened, since
// the log is one of the things it deletes.
const removeDataFlag = "--remove-data"

// runRemoveData finds where this user's data is and deletes it. A missing folder is not an error.
func runRemoveData() error {
	cache, err := os.UserCacheDir() // %LOCALAPPDATA%
	if err != nil {
		return fmt.Errorf("locate the cache directory: %w", err)
	}
	settingsFile, err := config.Path()
	if err != nil {
		return err
	}
	var dataDir string
	if _, statErr := os.Stat(settingsFile); statErr == nil { // Load would make the file if it were missing
		if cfg, err := config.Load(settingsFile); err == nil {
			dataDir = cfg.Search.DataDir
		}
	}
	return removeData(filepath.Join(cache, "Omni"), filepath.Dir(settingsFile), dataDir)
}

// removeData deletes the program's own folders: appData (the engine, the index, caches, clipboard history, the log) and
// settings (config.toml, the notes and what the bar has learned), the engine and the index inside a folder the person chose for
// them. (The start-at-sign-in entry is the installer's to remove, whatever the person answers.) The folder the person chose is never deleted whole, only the two it holds:
// a typo in config.toml must not be able to point this at a drive.
func removeData(appData, settings, dataDir string) error {
	var errs []error
	for _, dir := range []string{appData, settings} {
		if !strings.EqualFold(filepath.Base(dir), "Omni") {
			errs = append(errs, fmt.Errorf("%s is not an Omni folder, not deleting it", dir))
			continue
		}
		errs = append(errs, os.RemoveAll(dir))
	}
	if dataDir != "" {
		if !strings.EqualFold(filepath.Base(dataDir), "Omni") {
			errs = append(errs, fmt.Errorf("%s is not an Omni folder, not deleting what is in it", dataDir))
		} else {
			errs = append(errs, os.RemoveAll(filepath.Join(dataDir, "engine")), os.RemoveAll(filepath.Join(dataDir, "memory")))
			_ = os.Remove(dataDir) // only if nothing else is in it
		}
	}
	return errors.Join(errs...)
}
