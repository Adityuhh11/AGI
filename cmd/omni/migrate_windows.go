package main

import (
	"fmt"
	"os"
	"path/filepath"

	"omni/internal/config"
)

// migrateFromAGI carries a person's settings over from the folders the program kept under its old name, AGI. It runs
// only while Omni has no settings file of its own, so it never overwrites anything. The engine is not moved: its Python
// environment records absolute paths and would not start from another folder, so the settings point at where it already
// is (data_dir) instead.
func migrateFromAGI(configDir, cacheDir string) error {
	oldSettings := filepath.Join(configDir, "AGI", "config.toml")
	newSettings := filepath.Join(configDir, "Omni", "config.toml")
	if _, err := os.Stat(newSettings); err == nil {
		return nil
	}
	if _, err := os.Stat(oldSettings); err != nil {
		return nil // nothing from before, or nothing readable: a fresh start
	}
	if err := os.MkdirAll(filepath.Dir(newSettings), 0o755); err != nil {
		return fmt.Errorf("make the settings folder: %w", err)
	}
	if err := os.Rename(oldSettings, newSettings); err != nil {
		return fmt.Errorf("move the settings: %w", err)
	}
	oldCache, newCache := filepath.Join(cacheDir, "AGI"), filepath.Join(cacheDir, "Omni")
	if err := os.MkdirAll(newCache, 0o755); err != nil {
		return fmt.Errorf("make the data folder: %w", err)
	}
	if _, err := os.Stat(filepath.Join(newCache, "clipboard.json")); err != nil {
		_ = os.Rename(filepath.Join(oldCache, "clipboard.json"), filepath.Join(newCache, "clipboard.json")) // best effort: a history, not settings
	}
	if _, err := os.Stat(filepath.Join(oldCache, "engine")); err != nil {
		return nil
	}
	cfg, err := config.Load(newSettings)
	if err != nil {
		return fmt.Errorf("read the moved settings: %w", err)
	}
	if cfg.Search.DataDir != "" {
		return nil // the person chose a folder; the engine is already there
	}
	cfg.Search.DataDir = oldCache
	if err := cfg.Save(newSettings); err != nil {
		return fmt.Errorf("point the settings at the engine: %w", err)
	}
	return nil
}

// migrateEarlierVersions is what main runs at start: Windows builds up to 0.9.0 were called AGI (docs/RELEASING.md).
func migrateEarlierVersions() error {
	cfgDir, err := os.UserConfigDir()
	if err != nil {
		return nil
	}
	cacheDir, err := os.UserCacheDir()
	if err != nil {
		return nil
	}
	return migrateFromAGI(cfgDir, cacheDir)
}
