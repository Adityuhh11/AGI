package llm

import (
	"fmt"
	"os"
	"path"
	"path/filepath"
)

// Layout is the folder the model and its server live in.
type Layout struct{ Root string }

// Locate returns where Omni AI is kept: dataDir\chat when the person chose a folder for the downloads, else
// %LOCALAPPDATA%\Omni\chat. The folder in OMNI_CHAT_DIR (for developers) wins over both.
func Locate(dataDir string) (Layout, error) {
	if dir := os.Getenv("OMNI_CHAT_DIR"); dir != "" {
		return Layout{Root: dir}, nil
	}
	if dataDir != "" {
		return Layout{Root: filepath.Join(dataDir, "chat")}, nil
	}
	cache, err := os.UserCacheDir() // %LOCALAPPDATA% on Windows
	if err != nil {
		return Layout{}, fmt.Errorf("locate the chat folder: %w", err)
	}
	return Layout{Root: filepath.Join(cache, "Omni", "chat")}, nil
}

// Dir is the folder llama.cpp is unpacked into.
func (l Layout) Dir() string { return filepath.Join(l.Root, "llama") }

// Server is the llama.cpp server program.
func (l Layout) Server() string { return filepath.Join(l.Dir(), "llama-server.exe") }

// ModelFile is the model.
func (l Layout) ModelFile() string { return filepath.Join(l.Root, "models", path.Base(Model.URL)) }

// Manifest records what is installed; it is written last, so its presence means a finished install.
func (l Layout) Manifest() string { return filepath.Join(l.Root, "chat.json") }

// Installed reports whether an install finished and its files are still there.
func (l Layout) Installed() bool {
	for _, p := range []string{l.Manifest(), l.Server(), l.ModelFile()} {
		if _, err := os.Stat(p); err != nil {
			return false
		}
	}
	return true
}
