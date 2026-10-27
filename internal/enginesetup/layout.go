// Package enginesetup knows where the file-search engine lives on disk and whether it is installed.
// The engine is a Python environment, a llama.cpp server and two model files under one folder, so
// removing file search is deleting that folder; the index it built is kept elsewhere.
package enginesetup

import (
	"fmt"
	"os"
	"path/filepath"
)

// Layout is the engine folder and the paths inside it, and where the index is kept.
type Layout struct {
	Root string
	// Memory is the folder of the index; empty means the engine's own default, %LOCALAPPDATA%\Omni\memory.
	Memory string
}

// Locate returns where the engine and the index live: under dataDir (the engine in dataDir\engine, the index in
// dataDir\memory) when the person chose a folder for them, else %LOCALAPPDATA%\Omni\engine and \memory. The folder in
// OMNI_ENGINE_DIR (for developers who keep an engine somewhere else) wins over both.
func Locate(dataDir string) (Layout, error) {
	if dir := os.Getenv("OMNI_ENGINE_DIR"); dir != "" {
		return Layout{Root: dir}, nil
	}
	if dataDir != "" {
		return Layout{Root: filepath.Join(dataDir, "engine"), Memory: filepath.Join(dataDir, "memory")}, nil
	}
	cache, err := os.UserCacheDir() // %LOCALAPPDATA% on Windows
	if err != nil {
		return Layout{}, fmt.Errorf("locate the engine folder: %w", err)
	}
	return Layout{Root: filepath.Join(cache, "Omni", "engine")}, nil
}

// Index is the folder the engine keeps its index in.
func (l Layout) Index() string {
	if l.Memory != "" {
		return l.Memory
	}
	cache, err := os.UserCacheDir()
	if err != nil {
		return ""
	}
	return filepath.Join(cache, "Omni", "memory")
}

// HasIndex reports whether an index has been built in Index.
func (l Layout) HasIndex() bool {
	if l.Index() == "" {
		return false
	}
	_, err := os.Stat(filepath.Join(l.Index(), "meta.db"))
	return err == nil
}

// App is the engine's source, and the folder it runs in.
func (l Layout) App() string { return filepath.Join(l.Root, "app") }

// Python is the interpreter of the engine's environment.
func (l Layout) Python() string { return filepath.Join(l.Root, "venv", "Scripts", "python.exe") }

// LlamaServer is the llama.cpp server the engine starts to embed with.
func (l Layout) LlamaServer() string { return filepath.Join(l.Root, "llama", "llama-server.exe") }

// UV is the package tool the install uses to build the Python environment.
func (l Layout) UV() string { return filepath.Join(l.Root, "tools", "uv.exe") }

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

// Env is what the engine process needs to find its llama.cpp server and its models, and the index when it is kept
// somewhere other than the default.
func (l Layout) Env() []string {
	env := []string{"OMNI_LLAMA_SERVER=" + l.LlamaServer(), "OMNI_MODEL_DIR=" + l.Models()}
	if l.Memory != "" {
		env = append(env, "OMNI_MEMORY_DIR="+l.Memory)
	}
	return env
}
