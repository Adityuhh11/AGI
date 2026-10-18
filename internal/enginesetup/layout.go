// Package enginesetup knows where the file-search engine lives on disk and whether it is installed.
// The engine is a Python environment, a llama.cpp server and two model files under one folder, so
// removing file search is deleting that folder; the index it built is kept elsewhere.
package enginesetup

import (
	"fmt"
	"os"
	"path/filepath"
)

// Layout is the engine folder and the paths inside it.
type Layout struct{ Root string }

// Locate returns the engine folder: %LOCALAPPDATA%\AGI\engine, or the folder in AGI_ENGINE_DIR
// (for developers who keep an engine somewhere else).
func Locate() (Layout, error) {
	if dir := os.Getenv("AGI_ENGINE_DIR"); dir != "" {
		return Layout{Root: dir}, nil
	}
	cache, err := os.UserCacheDir() // %LOCALAPPDATA% on Windows
	if err != nil {
		return Layout{}, fmt.Errorf("locate the engine folder: %w", err)
	}
	return Layout{Root: filepath.Join(cache, "AGI", "engine")}, nil
}

// App is the engine's source, and the folder it runs in.
func (l Layout) App() string { return filepath.Join(l.Root, "app") }

// Python is the interpreter of the engine's environment.
func (l Layout) Python() string { return filepath.Join(l.Root, "venv", "Scripts", "python.exe") }

// LlamaServer is the llama.cpp server the engine starts to embed with.
func (l Layout) LlamaServer() string { return filepath.Join(l.Root, "llama", "llama-server.exe") }

// Models is the folder with the two model files.
func (l Layout) Models() string { return filepath.Join(l.Root, "models") }

// Manifest records what is installed; it is written last, so its presence means a finished install.
func (l Layout) Manifest() string { return filepath.Join(l.Root, "engine.json") }

// Installed reports whether an install finished and its interpreter is still there.
func (l Layout) Installed() bool {
	for _, path := range []string{l.Manifest(), l.Python()} {
		if _, err := os.Stat(path); err != nil {
			return false
		}
	}
	return true
}

// Env is what the engine process needs to find its llama.cpp server and its models.
func (l Layout) Env() []string {
	return []string{"AGI_LLAMA_SERVER=" + l.LlamaServer(), "AGI_MODEL_DIR=" + l.Models()}
}
