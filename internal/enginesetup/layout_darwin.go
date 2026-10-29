package enginesetup

import "path/filepath"

// Python is the interpreter of the engine's environment.
func (l Layout) Python() string { return filepath.Join(l.Root, "venv", "bin", "python") }

// LlamaServer is the llama.cpp server the engine starts to embed with.
func (l Layout) LlamaServer() string { return filepath.Join(l.Root, "llama", "llama-server") }

// UV is the package tool the install uses to build the Python environment.
func (l Layout) UV() string { return filepath.Join(l.Root, "tools", "uv") }
