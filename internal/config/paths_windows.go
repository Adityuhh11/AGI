package config

import (
	"os"
	"path/filepath"
)

// dataBase is %LOCALAPPDATA%: the engine and the models are large and belong to this machine, not to the roaming profile.
func dataBase() (string, error) { return os.UserCacheDir() }

// logDir is %LOCALAPPDATA%\Omni\logs.
func logDir() (string, error) {
	dir, err := DataDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "logs"), nil
}
