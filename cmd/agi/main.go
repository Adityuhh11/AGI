// Command agi is the launcher: a global hotkey opens a search bar over the memory engine's index.
package main

import (
	"context"
	"io"
	"io/fs"
	"log"
	"os"
	"path/filepath"
	"time"

	"github.com/wailsapp/wails/v3/pkg/application"

	"agi"
	"agi/internal/config"
	"agi/internal/files"
	"agi/internal/panel"
	"agi/internal/search"
	"agi/internal/sidecar"
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

// setUpLog sends the log to the file agi.log under %LOCALAPPDATA%\AGI\logs as well as to the
// console, if there is one: the release build has no console window, so the file is the only record.
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

	// A second launch (the shortcut clicked again) does not start a second launcher or engine: it
	// exits right inside application.New and the running one shows the search bar instead.
	var bar *panel.Panel
	app := application.New(application.Options{
		Name:   "AGI",
		Assets: application.AssetOptions{Handler: application.AssetFileServerFS(assets)},
		SingleInstance: &application.SingleInstanceOptions{
			UniqueID:               "com.adityuhh11.agi.launcher",
			OnSecondInstanceLaunch: func(application.SecondInstanceData) { bar.Show() },
		},
	})

	// The engine dies with the launcher, even if the launcher is killed from the task manager.
	if err := sidecar.KillWithLauncher(); err != nil {
		log.Printf("the search engine will not be tied to the launcher's lifetime: %v", err)
	}
	engine, err := sidecar.New(sidecar.Options{Dir: cfg.Sidecar.Dir, Log: log.Writer()})
	if err != nil {
		return err
	}
	engineCtx, stopEngine := context.WithCancel(context.Background())
	engineStopped := make(chan struct{})
	go func() {
		engine.Run(engineCtx)
		close(engineStopped)
	}()

	roots := map[string]string{}
	for _, root := range cfg.Roots {
		roots[config.SourceID(root)] = root
	}

	bar = panel.New(app, cfg.Hotkey)
	for _, service := range []application.Service{
		application.NewService(panel.NewService(bar)),
		application.NewService(status.New(status.Info{Hotkey: cfg.Hotkey, Roots: cfg.Roots}, bar.HotkeyError, engine.Status)),
		application.NewService(search.New(engine.Client(), roots, engine.Status)),
		application.NewService(files.New(cfg.Roots, bar.HideForLaunch)),
	} {
		app.RegisterService(service)
	}

	runErr := app.Run()

	stopEngine()
	select {
	case <-engineStopped:
	case <-time.After(10 * time.Second):
		log.Print("the search engine did not stop in time")
	}
	return runErr
}
