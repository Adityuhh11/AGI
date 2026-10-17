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
	"strings"

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
	// Browser is the browser web searches open in: a name Windows knows it by ("zen", "firefox",
	// "chrome") or the path of its program. Empty means the Windows default browser.
	Browser string `toml:"browser,omitempty"`
	// SearchURL is the address of a web search with {q} where the text goes. Without it, Firefox-based
	// browsers search with the engine set inside them and others use DuckDuckGo.
	SearchURL string `toml:"search_url,omitempty"`
	// Quicklinks turn a trigger word and some text into a web address: "gh rust lifetimes".
	Quicklinks []Quicklink `toml:"quicklink,omitempty"`
	Sidecar    Sidecar     `toml:"sidecar"`
}

// Quicklink is a web address with a place for what the user types after its trigger word.
type Quicklink struct {
	Trigger string `toml:"trigger"`
	Title   string `toml:"title"`
	// URL holds {q}, where the text typed after the trigger goes, escaped.
	URL string `toml:"url"`
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
	if err := c.validateWeb(); err != nil {
		return err
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

// validateWeb checks the browser settings: a typo in a quicklink should fail loudly, not give a dead row.
func (c Config) validateWeb() error {
	if c.SearchURL != "" {
		if err := checkTemplate(c.SearchURL); err != nil {
			return fmt.Errorf("search_url: %w", err)
		}
	}
	seen := map[string]bool{}
	for _, link := range c.Quicklinks {
		switch {
		case link.Trigger == "" || strings.ContainsAny(link.Trigger, " \t:"):
			return fmt.Errorf("quicklink trigger %q must be one word without a colon", link.Trigger)
		case link.Title == "":
			return fmt.Errorf("quicklink %q needs a title", link.Trigger)
		case seen[strings.ToLower(link.Trigger)]:
			return fmt.Errorf("quicklink trigger %q is used twice", link.Trigger)
		}
		seen[strings.ToLower(link.Trigger)] = true
		if err := checkTemplate(link.URL); err != nil {
			return fmt.Errorf("quicklink %q: %w", link.Trigger, err)
		}
	}
	return nil
}

// checkTemplate says what is wrong with a web address template, if anything.
func checkTemplate(template string) error {
	if !strings.HasPrefix(template, "https://") && !strings.HasPrefix(template, "http://") {
		return fmt.Errorf("%q must start with https:// or http://", template)
	}
	if strings.Count(template, "{q}") != 1 {
		return fmt.Errorf("%q must contain {q} once, where the text goes", template)
	}
	return nil
}
