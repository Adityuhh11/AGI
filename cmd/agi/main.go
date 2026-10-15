// Command agi is the launcher: a global hotkey opens a search bar over the memory engine's index.
package main

import (
	"io"
	"io/fs"
	"log"
	"os"
	"path/filepath"

	"github.com/wailsapp/wails/v3/pkg/application"

	"agi"
	"agi/internal/config"
	"agi/internal/panel"
	"agi/internal/status"
)

func main() {
	closeLog := setUpLog()
	defer closeLog()
	if err := run(); err != nil {
		log.Print(err)
		closeLog()
		os.Exit(1)
	}
}

// setUpLog sends the log to %LOCALAPPDATA%\AGI\logsgi.log as well as to the console, if there
// is one: the release build has no console window, so the file is the only record.
func setUpLog() (closeLog func()) {
	dir, err := os.UserCacheDir() // %LOCALAPPDATA% on Windows
	if err != nil {
		return func() {}
	}
	logDir := filepath.Join(dir, "AGI", "logs")
	if err := os.MkdirAll(logDir, 0o755); err != nil {
		return func() {}
	}
	file, err := os.OpenFile(filepath.Join(logDir, "agi.log"), os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return func() {}
	}
	log.SetOutput(io.MultiWriter(file, bestEffort{os.Stderr}))
	log.Printf("agi starting (pid %d)", os.Getpid())
	return func() { _ = file.Close() }
}

// bestEffort ignores write errors: without a console (the release build) stderr is not a valid
// handle, and io.MultiWriter would otherwise stop before reaching the log file.
type bestEffort struct{ w io.Writer }

func (b bestEffort) Write(p []byte) (int, error) {
	_, _ = b.w.Write(p)
	return len(p), nil
}

func run() error {
	path, err := config.Path()
	if err != nil {
		return err
	}
	cfg, err := config.Load(path)
	if err != nil {
		return err
	}
	assets, err := fs.Sub(agi.Frontend, "frontend/dist")
	if err != nil {
		return err
	}

	app := application.New(application.Options{
		Name:   "AGI",
		Assets: application.AssetOptions{Handler: application.AssetFileServerFS(assets)},
	})
	bar := panel.New(app, cfg.Hotkey)
	app.RegisterService(application.NewService(panel.NewService(bar)))
	app.RegisterService(application.NewService(
		status.New(status.Info{Hotkey: cfg.Hotkey, Roots: cfg.Roots}, bar.HotkeyError),
	))
	return app.Run()
}
