package config

import (
	"os"
	"path/filepath"
)

// DefaultHotkey is rare enough not to collide with common apps (Alt+Space, Win+Space and
// Ctrl+Alt+Space were all taken on the development machine).
const DefaultHotkey = "Alt+Shift+Space"

// Default is the configuration of a fresh install: the sample folder as the only root and the
// sidecar of the repository this program was built in, when that can be found.
func Default() Config {
	var start []string
	if exe, err := os.Executable(); err == nil {
		start = append(start, filepath.Dir(exe))
	}
	if cwd, err := os.Getwd(); err == nil {
		start = append(start, cwd)
	}
	return defaultsFor(findRepoRoot(start...))
}

func defaultsFor(repoRoot string) Config {
	cfg := Config{Hotkey: DefaultHotkey}
	if repoRoot != "" {
		cfg.Roots = []string{filepath.Join(repoRoot, "Sample Data")}
		cfg.Sidecar.Dir = filepath.Join(repoRoot, "sidecar")
	}
	return cfg
}

// findRepoRoot walks up from each start folder until it finds one that holds sidecar\pyproject.toml.
func findRepoRoot(starts ...string) string {
	for _, start := range starts {
		for dir := start; ; dir = filepath.Dir(dir) {
			if _, err := os.Stat(filepath.Join(dir, "sidecar", "pyproject.toml")); err == nil {
				return dir
			}
			if filepath.Dir(dir) == dir {
				break
			}
		}
	}
	return ""
}
