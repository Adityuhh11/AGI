package config

import (
	"os"
	"path/filepath"
)

// dataBase is ~/Library/Application Support, beside config.toml. Not ~/Library/Caches: cleaning tools empty that, and a gigabyte
// of models and an index that took hours to build are not a cache.
func dataBase() (string, error) { return os.UserConfigDir() }

// logDir is ~/Library/Logs/Omni, where Console.app looks.
func logDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, "Library", "Logs", "Omni"), nil
}
