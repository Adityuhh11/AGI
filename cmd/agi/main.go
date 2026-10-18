// Command agi is the launcher: a global hotkey opens a search bar over the memory engine's index.
package main

import (
	"context"
	"fmt"
	"io"
	"io/fs"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"sync"
	"time"

	"github.com/wailsapp/wails/v3/pkg/application"
	"github.com/wailsapp/wails/v3/pkg/events"

	"agi"
	"agi/internal/apps"
	"agi/internal/config"
	"agi/internal/enginesetup"
	"agi/internal/files"
	"agi/internal/indexing"
	"agi/internal/palette"
	"agi/internal/panel"
	"agi/internal/search"
	"agi/internal/settings"
	"agi/internal/sidecar"
	"agi/internal/state"
	"agi/internal/status"
	"agi/internal/tray"
	"agi/internal/watcher"
	"agi/internal/web"
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
	store, err := settings.Open(path)
	if err != nil {
		return err
	}
	cfg := store.Get()
	assets, err := fs.Sub(agi.Frontend, "frontend/dist")
	if err != nil {
		return err
	}

	// A second launch (the shortcut clicked again) does not start a second launcher or engine: it
	// exits right inside application.New and the running one shows the search bar instead.
	cacheDir, err := os.UserCacheDir() // %LOCALAPPDATA% on Windows
	if err != nil {
		return fmt.Errorf("locate the cache directory: %w", err)
	}
	var bar *panel.Panel
	iconDir := filepath.Join(cacheDir, "AGI", "icons")
	catalog := apps.NewCatalog(filepath.Join(cacheDir, "AGI", "apps.json"), iconDir)
	// Everything below that needs the indexed folders asks the store when it needs them, so folders
	// added in the settings count at once.
	roots := func() []string { return store.Get().Roots }
	sourceRoots := func() map[string]string {
		byID := map[string]string{}
		for _, root := range roots() {
			byID[config.SourceID(root)] = root
		}
		return byID
	}
	thumbnails := files.Thumbnails(roots, filepath.Join(cacheDir, "AGI", "thumbs"))
	icons := apps.Icons(catalog.Index, iconDir)
	app := application.New(application.Options{
		Name: "AGI",
		Assets: application.AssetOptions{
			Handler: application.AssetFileServerFS(assets),
			// /thumb serves picture thumbnails, /icon the icons of apps; anything else is the page.
			Middleware: func(next http.Handler) http.Handler {
				return thumbnails(icons(next))
			},
		},
		SingleInstance: &application.SingleInstanceOptions{
			UniqueID:               "com.adityuhh11.agi.launcher",
			OnSecondInstanceLaunch: func(application.SecondInstanceData) { bar.Show() },
		},
	})

	// The engine dies with the launcher, even if the launcher is killed from the task manager.
	if err := sidecar.KillWithLauncher(); err != nil {
		log.Printf("the search engine will not be tied to the launcher's lifetime: %v", err)
	}
	// The tray line is rebuilt whenever the engine, the indexing or the hotkey changes. These
	// variables are all assigned before the goroutines that call refresh start.
	var (
		icon    *tray.Tray
		indexer *indexing.Manager
		engine  *sidecar.Supervisor
	)
	refresh := func() {
		icon.SetStatus(statusLine(bar.HotkeyError(), engine.Status(), indexer.Status()))
	}
	layout, err := enginesetup.Locate()
	if err != nil {
		return err
	}
	engine, err = sidecar.New(sidecar.Options{
		Dir: layout.App(), Python: layout.Python(), Env: layout.Env(),
		Log: log.Writer(), OnChange: func(sidecar.Status) { refresh() },
	})
	if err != nil {
		return err
	}
	indexer = indexing.New(engine.Client(), cfg.Roots, func() bool { return engine.Status().State == sidecar.StateReady },
		func(indexing.Status) { refresh() })
	indexer.SetExclude(cfg.Exclude)
	browser := web.New(cfg.Browser, cfg.SearchURL, cfg.Quicklinks)

	// What the bar has learned about its user (what was launched, and after typing what) lives beside the
	// settings.
	history, err := state.Open(filepath.Join(filepath.Dir(path), "state.json"))
	if err != nil {
		return err
	}

	bar = panel.New(app, cfg.Hotkey)
	opener := files.New(roots, bar.HideForLaunch) // opens files and shows them in Explorer
	bar.OnHotkeyError = refresh
	icon = tray.New(app, bar.Show, indexer.Rescan)
	for _, service := range []application.Service{
		application.NewService(panel.NewService(bar)),
		application.NewService(status.New(store.Get, bar.HotkeyError, engine.Status, indexer.Status)),
		application.NewService(search.New(engine.Client(), sourceRoots, engine.Status)),
		application.NewService(opener),
		application.NewService(palette.New(catalog, history, browser, opener, bar.HideForLaunch)),
	} {
		app.RegisterService(service)
	}

	// The engine and the indexer report to the tray, which only exists once the app is running.
	ctx, stop := context.WithCancel(context.Background())
	engineSwitch := sidecar.NewSwitch(ctx, engine)
	engineWanted := func(c config.Config) bool { return c.Search.Enabled && layout.Installed() }

	// The watcher follows the folders: it is started again whenever they change.
	var watchMu sync.Mutex
	var stopWatching context.CancelFunc
	watchFolders := func(folders []string) {
		watchMu.Lock()
		defer watchMu.Unlock()
		if stopWatching != nil {
			stopWatching()
		}
		var watchCtx context.Context
		watchCtx, stopWatching = context.WithCancel(ctx)
		// A file that appears, moves or goes is in the index a few seconds after the folder goes
		// quiet (and within 30 s even if it never does).
		go watcher.Watch(watchCtx, folders, 3*time.Second, 30*time.Second, indexer.Changed)
	}

	// What the settings window changes takes effect here. The hotkey is not here: the window rebinds it
	// first and only saves the change if that worked.
	store.Subscribe(func(old, updated config.Config) {
		browser.Update(updated.Browser, updated.SearchURL, updated.Quicklinks)
		if !slices.Equal(old.Roots, updated.Roots) || !slices.Equal(old.Exclude, updated.Exclude) {
			indexer.SetFolders(updated.Roots, updated.Exclude)
			watchFolders(updated.Roots)
		}
		if want := engineWanted(updated); want != engineWanted(old) {
			if want {
				engineSwitch.On()
			} else {
				engineSwitch.Off()
			}
		}
	})

	app.Event.OnApplicationEvent(events.Common.ApplicationStarted, func(*application.ApplicationEvent) {
		if engineWanted(cfg) {
			engineSwitch.On()
		} else {
			engineSwitch.Off()
		}
		go indexer.Run(ctx)
		// The list of installed apps is served from its cache at once and rebuilt in the background,
		// again whenever a program is installed or removed.
		go catalog.Run(ctx)
		go watcher.Watch(ctx, apps.StartMenuFolders(), 3*time.Second, 30*time.Second, func(string) { catalog.Refresh() })
		watchFolders(cfg.Roots)
	})

	runErr := app.Run()

	stop()
	stopped := make(chan struct{})
	go func() { engineSwitch.Wait(); close(stopped) }()
	select {
	case <-stopped:
	case <-time.After(10 * time.Second):
		log.Print("the search engine did not stop in time")
	}
	return runErr
}

// statusLine is the one sentence the tray shows about the launcher, most urgent problem first.
func statusLine(hotkeyError string, engine sidecar.Status, index indexing.Status) string {
	switch {
	case hotkeyError != "":
		return hotkeyError
	case engine.State == sidecar.StateOff:
		return "File search is off. Open Settings to turn it on."
	case engine.State == sidecar.StateFailed:
		return engine.Message
	case engine.State != sidecar.StateReady:
		return "Search is starting up..."
	}
	return index.String()
}
