package main

import (
	"os"
	"path/filepath"
	"testing"

	"omni/internal/config"
)

func write(t *testing.T, path, text string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(text), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestMigrateFromAGI(t *testing.T) {
	tests := []struct {
		name         string
		oldSettings  bool
		oldEngine    bool
		newSettings  bool
		wantMoved    bool
		wantDataDir  bool
		wantClipMove bool
	}{
		{name: "settings and engine", oldSettings: true, oldEngine: true, wantMoved: true, wantDataDir: true, wantClipMove: true},
		{name: "settings only", oldSettings: true, wantMoved: true, wantClipMove: true},
		{name: "omni already set up", oldSettings: true, oldEngine: true, newSettings: true},
		{name: "fresh machine"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfgDir, cacheDir := t.TempDir(), t.TempDir()
			if tt.oldSettings {
				write(t, filepath.Join(cfgDir, "AGI", "config.toml"), "[search]\nenabled = true\n")
				write(t, filepath.Join(cacheDir, "AGI", "clipboard.json"), "[]")
			}
			if tt.oldEngine {
				write(t, filepath.Join(cacheDir, "AGI", "engine", "x"), "x")
			}
			if tt.newSettings {
				write(t, filepath.Join(cfgDir, "Omni", "config.toml"), "[search]\nenabled = false\n")
			}
			if err := migrateFromAGI(cfgDir, cacheDir); err != nil {
				t.Fatal(err)
			}
			_, oldLeft := os.Stat(filepath.Join(cfgDir, "AGI", "config.toml"))
			if moved := oldLeft != nil && tt.oldSettings; moved != tt.wantMoved {
				t.Errorf("settings moved = %v, want %v", moved, tt.wantMoved)
			}
			_, clipErr := os.Stat(filepath.Join(cacheDir, "Omni", "clipboard.json"))
			if (clipErr == nil) != tt.wantClipMove {
				t.Errorf("clipboard moved = %v, want %v", clipErr == nil, tt.wantClipMove)
			}
			if tt.wantMoved {
				cfg, err := config.Load(filepath.Join(cfgDir, "Omni", "config.toml"))
				if err != nil {
					t.Fatal(err)
				}
				if got := cfg.Search.DataDir == filepath.Join(cacheDir, "AGI"); got != tt.wantDataDir {
					t.Errorf("data_dir points at the old engine = %v, want %v (%q)", got, tt.wantDataDir, cfg.Search.DataDir)
				}
			}
		})
	}
}
