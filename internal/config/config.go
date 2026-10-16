// Package config reads the launcher's settings file, %APPDATA%\AGI\config.toml.
//
// A missing file is created with the defaults, a file that sets only some keys keeps the
// defaults for the rest, and a key that does not exist is an error (a typo in a hotkey or a
// folder list should not be silently ignored).
package config

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"

	"github.com/pelletier/go-toml/v2"
)

// Config is the content of config.toml.
type Config struct {
	// Hotkey opens the search bar from anywhere, e.g. "Alt+Shift+Space".
	Hotkey string `toml:"hotkey"`
	// Roots are the folders that get indexed; every file under them is searchable.
	Roots []string `toml:"roots"`
	// Exclude are folders inside a root to leave out, as absolute paths. What they held is dropped
	// from the index at the next sync.
	Exclude []string `toml:"exclude,omitempty"`
	Sidecar Sidecar  `toml:"sidecar"`
}

// Sidecar says where the Python memory engine lives.
type Sidecar struct {
	// Dir is the folder `uv run python -m agi_memory` runs in (the repository's sidecar folder).
	Dir string `toml:"dir"`
}

// Path is where the settings file lives for the current user.
func Path() (string, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", fmt.Errorf("locate the user config directory: %w", err)
	}
	return filepath.Join(dir, "AGI", "config.toml"), nil
}

// Load reads the settings file at path. If it does not exist it is created with the defaults.
func Load(path string) (Config, error) {
	cfg := Default()
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		if err := cfg.Save(path); err != nil {
			return Config{}, err
		}
		return cfg, nil
	}
	if err != nil {
		return Config{}, fmt.Errorf("read config: %w", err)
	}
	decoder := toml.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&cfg); err != nil {
		return Config{}, fmt.Errorf("parse %s: %w", path, err)
	}
	if err := cfg.Validate(); err != nil {
		return Config{}, fmt.Errorf("check %s: %w", path, err)
	}
	return cfg, nil
}

// Save writes the settings to path, creating the folder if needed.
func (c Config) Save(path string) error {
	body, err := toml.Marshal(c)
	if err != nil {
		return fmt.Errorf("encode config: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("create config folder: %w", err)
	}
	if err := os.WriteFile(path, body, 0o644); err != nil {
		return fmt.Errorf("write config: %w", err)
	}
	return nil
}

// Validate reports the first thing wrong with the settings.
func (c Config) Validate() error {
	if c.Hotkey == "" {
		return fmt.Errorf("hotkey must not be empty")
	}
	seen := map[string]string{}
	for _, root := range c.Roots {
		if !filepath.IsAbs(root) {
			return fmt.Errorf("root %q must be an absolute path", root)
		}
		id := SourceID(root)
		if other, dup := seen[id]; dup {
			return fmt.Errorf("root %q repeats %q", root, other)
		}
		seen[id] = root
	}
	for _, folder := range c.Exclude {
		if !filepath.IsAbs(folder) {
			return fmt.Errorf("excluded folder %q must be an absolute path", folder)
		}
		inside := false
		for _, root := range c.Roots {
			inside = inside || len(ExcludedUnder(root, []string{folder})) > 0
		}
		if !inside {
			return fmt.Errorf("excluded folder %q is not inside any root", folder)
		}
	}
	return nil
}
