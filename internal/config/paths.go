package config

import (
	"fmt"
	"path/filepath"
)

// DataDir is the folder for everything Omni makes besides the settings file: the engine, the index, the chat model, the
// clipboard history, the icon and thumbnail caches. Each system says where its base is (dataBase, in paths_windows.go and
// paths_darwin.go); the folder is always called Omni.
func DataDir() (string, error) {
	base, err := dataBase()
	if err != nil {
		return "", fmt.Errorf("locate the data directory: %w", err)
	}
	return filepath.Join(base, "Omni"), nil
}

// LogDir is the folder of omni.log. Each system keeps its logs where its own tools look (logDir).
func LogDir() (string, error) {
	dir, err := logDir()
	if err != nil {
		return "", fmt.Errorf("locate the log directory: %w", err)
	}
	return dir, nil
}
