// Command agi is the launcher: a global hotkey opens a search bar over the memory engine's index.
package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/wailsapp/wails/v3/pkg/application"
	"github.com/wailsapp/wails/v3/pkg/events"

	"agi"
	"agi/internal/apps"
	"agi/internal/autostart"
	"agi/internal/clipboard"
	"agi/internal/config"
	"agi/internal/enginesetup"
	"agi/internal/files"
	"agi/internal/indexing"
	"agi/internal/palette"
	"agi/internal/panel"
	"agi/internal/search"
	"agi/internal/settings"
	"agi/internal/settingsui"
	"agi/internal/sidecar"
	"agi/internal/snippets"
	"agi/internal/system"
	"agi/internal/state"
	"agi/internal/survey"
	"agi/internal/status"
	"agi/internal/tray"
	"agi/internal/update"
	"agi/internal/watcher"
	"agi/internal/web"
)

// version is the build's version, set by scripts/release.ps1 with -ldflags "-X main.version=...".
var version = "dev"

func main() {
	if len(os.Args) == 2 && os.Args[1] == removeDataFlag {
		if err := runRemoveData(); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		return
	}
	closeLog := setUpLog()
	defer closeLog()
	if err := run(finishUpdate()); err != nil {
		log.Print(err)
		closeLog()
		os.Exit(1)
	}
}

const maxLogBytes = 5 << 20

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
	// One earlier log is kept, so the file never grows past about twice this.
	if info, err := os.Stat(filepath.Join(logDir, "agi.log")); err == nil && info.Size() > maxLogBytes {
		_ = os.Rename(filepath.Join(logDir, "agi.log"), filepath.Join(logDir, "agi.old.log"))
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

// finishUpdate is the start of a launcher that an update has just put in place: it was started with "--wait-for <the
// old launcher's process id>", and waits for that one to end (a second launcher while the first still runs would only hand
// over to it and exit). It also deletes the program the update moved aside. It says whether this start follows an update.
func finishUpdate() (afterUpdate bool) {
	if pid, ok := update.WaitForArg(os.Args[1:]); ok {
		afterUpdate = true
		if !update.WaitForExit(pid, 15*time.Second) {
			log.Printf("update: the earlier launcher (process %d) was still running after 15 s; starting anyway", pid)
		}
		log.Print("update: started as the new launcher")
	}
	if exe, err := os.Executable(); err == nil {
		if err := update.RemoveOld(exe); err != nil {
			log.Printf("update: could not delete the earlier program: %v", err)
		}
	}
	return afterUpdate
}

func run(afterUpdate bool) error {
	path, err := config.Path()
	if err != nil {
		return err
	}
	_, statErr := os.Stat(path)
	firstRun := os.IsNotExist(statErr) // no settings file yet: this is the first start on this machine
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
		// The page never sees Alt+Space (Windows takes it as the system-menu key), so the settings'
		// hotkey field hears about it from here. Emitting from a goroutine keeps the window procedure free.
		Windows: application.WindowsOptions{WndProcInterceptor: settingsui.KeyMenuFilter(func(accelerator string) {
			go application.Get().Event.Emit("hotkey:alt-space", accelerator)
		})},
		SingleInstance: &application.SingleInstanceOptions{
			UniqueID:               instanceID(),
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
		icon.SetStatus(statusLine(bar.HotkeyError(), engine.Status(), indexer.Status(), store.Get().Search.HoldIndexing))
		icon.SetPaused(indexer.Status().Paused)
	}
	layout, err := enginesetup.Locate(cfg.Search.DataDir)
	if err != nil {
		return err
	}
	engineSource, err := fs.Sub(agi.EngineSource, "sidecar")
	if err != nil {
		return err
	}
	// Turning file search on installs the engine in the background; when that has worked, file search is
	// switched on, which starts the engine (the store's subscriber below).
	var engineSwitch *sidecar.Switch // set below; an install that ends before the app has started finds it nil
	setup := enginesetup.NewManager(enginesetup.NewInstaller(layout, engineSource), func(err error) {
		if err != nil {
			if !errors.Is(err, context.Canceled) {
				log.Printf("installing file search failed: %v", err)
			}
			return
		}
		log.Print("file search is installed")
		if err := store.Update(func(c *config.Config) { c.Search.Enabled = true }); err != nil {
			log.Printf("switching file search on: %v", err)
		}
		if engineSwitch != nil && store.Get().Search.Enabled {
			engineSwitch.On() // an update at startup leaves the setting as it was, so the store's subscriber does not fire
		}
	})
	engine, err = sidecar.New(sidecar.Options{
		// Asked at each start: the folder can still be chosen until file search is first downloaded.
		Locate: func() (string, string, []string) {
			l := setup.Layout()
			return l.App(), l.Python(), l.Env()
		},
		Log: log.Writer(), OnChange: func(sidecar.Status) { refresh() },
	})
	if err != nil {
		return err
	}
	indexer = indexing.New(engine.Client(), cfg.Roots,
		// A first download waits for "Start indexing" so the choices come before any picture is embedded.
		func() bool { return engine.Status().State == sidecar.StateReady && !store.Get().Search.HoldIndexing },
		func(indexing.Status) { refresh() })
	surveys := survey.New(engine.Client(), func() bool { return engine.Status().State == sidecar.StateReady })
	surveys.Set(cfg.Roots, cfg.Exclude)
	indexer.SetExclude(cfg.Exclude)
	indexer.SetPace(cfg.Search.Pace)
	indexer.SetOptions(indexOptions(cfg.Search))
	browser := web.New(cfg.Browser, cfg.SearchURL, cfg.Quicklinks)

	// What the bar has learned about its user (what was launched, and after typing what) lives beside the
	// settings.
	history, err := state.Open(filepath.Join(filepath.Dir(path), "state.json"))
	if err != nil {
		return err
	}
	// Pinned from the start, once: the two modes that nothing else in the bar leads to. They can be unpinned.
	if _, err := history.SeedPins("cmd:clipboard", "cmd:snippets"); err != nil {
		log.Printf("could not pin the clipboard and the snippets: %v", err)
	}

	bar = panel.New(app, cfg.Hotkey)
	opener := files.New(roots, bar.HideForLaunch) // opens files and shows them in Explorer
	bar.OnHotkeyError = refresh

	// What was copied is kept only while the history is switched on in the settings. Its own copies (a paste,
	// "copy path") carry this program's name and are not kept.
	clipStore, err := clipboard.OpenStore(filepath.Join(cacheDir, "AGI", "clipboard.json"))
	if err != nil {
		return err
	}
	self, err := os.Executable()
	if err != nil {
		return fmt.Errorf("locate this program: %w", err)
	}
	clipEnabled := func() bool { return store.Get().Clipboard.Enabled }
	clips := clipboard.NewManager(clipStore,
		clipboard.NewWatcher(clipboard.WindowsSource{}, clipStore, clipEnabled,
			func() []string { return store.Get().Clipboard.Ignore }, strings.TrimSuffix(filepath.Base(self), ".exe")),
		clipboard.NewPaster(app.Clipboard.SetText, bar.Previous, bar.Hide),
		clipEnabled)
	bar.OnShow = clips.Poll // a copy a moment before the bar opened is in the list
	snips := snippets.New(func() []config.Snippet { return store.Get().Snippets },
		clipboard.NewPaster(app.Clipboard.SetText, bar.Previous, bar.Hide),
		func() string { clip, _ := (clipboard.WindowsSource{}).Read(); return clip.Text })
	settingsWindow := settingsui.NewWindow(app)
	setupWindow := settingsui.NewSetupWindow(app)
	var checker *update.Checker
	checker = update.NewChecker(version, func() {
		release, _ := checker.Found()
		icon.SetUpdate(release.Version)
	})
	togglePause := func() {
		if indexer.Status().Paused {
			indexer.Resume()
		} else {
			indexer.Pause()
		}
	}
	icon = tray.New(app, bar.Show, settingsWindow.Open, setupWindow.Open, indexer.Rescan, togglePause)
	for _, service := range []application.Service{
		application.NewService(panel.NewService(bar)),
		application.NewService(status.New(store.Get, bar.HotkeyError, engine.Status, indexer.Status)),
		application.NewService(search.New(engine.Client(), sourceRoots, engine.Status)),
		application.NewService(opener),
		application.NewService(palette.New(catalog, history, browser, opener, clips, snips, func(section string) { settingsWindow.OpenAt(section) }, bar.HideForLaunch).With(palette.Extras{
			Copy: func(text string) error {
				if !app.Clipboard.SetText(text) {
					return errors.New("the clipboard is in use by another program; try again")
				}
				return nil
			},
			RunSystem: system.Run,
		})),
		application.NewService(settingsui.New(settingsui.Options{
			Store:    store,
			Setup:    setup,
			Indexing: indexer,
			Surveys:  surveys,
			Rebind:   bar.Rebind,
			PickFolder: func() (string, error) {
				return app.Dialog.OpenFile().CanChooseDirectories(true).CanChooseFiles(false).
					SetTitle("Choose a folder to search").PromptForSingleSelection()
			},
			Browsers:             web.Installed,
			OpenFolder:           func(path string) error { return exec.Command("explorer.exe", path).Start() },
			Clipboard:            clips,
			RebindClipboard:      func(accelerator string) error { return bar.RebindFill(accelerator, "clip: ") },
			ClipboardHotkeyError: bar.FillError,
			Updates:              checker,
			Installer:            update.NewInstaller(filepath.Join(cacheDir, "AGI", "update")),
			ExePath:              self,
			Restart: func() error {
				if err := update.StartNew(self, os.Getpid()); err != nil {
					return err
				}
				app.Quit()
				return nil
			},
			OpenURL:      browser.Open,
			Open:         settingsWindow.Open,
			OpenSetup:    setupWindow.Open,
			Autostart:    autostart.Enabled,
			SetAutostart: autostart.Set,
			Engine:       engine.Status,
			LogDir:       filepath.Join(cacheDir, "AGI", "logs"),
			Place:        setup,
			Version:      version,
		})),
	} {
		app.RegisterService(service)
	}

	// The engine and the indexer report to the tray, which only exists once the app is running.
	ctx, stop := context.WithCancel(context.Background())
	engineSwitch = sidecar.NewSwitch(ctx, engine)
	// An engine from an older version is never started: its API no longer matches what this launcher sends.
	engineWanted := func(c config.Config) bool {
		st := setup.State()
		return c.Search.Enabled && st.Installed && !st.NeedsUpdate && !st.Installing
	}

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
		if old.Search.Pace != updated.Search.Pace {
			indexer.SetPace(updated.Search.Pace)
		}
		if old.Search.HoldIndexing != updated.Search.HoldIndexing {
			refresh() // the tray says whether file search waits to be started
		}
		if indexOptions(old.Search) != indexOptions(updated.Search) {
			indexer.SetOptions(indexOptions(updated.Search))
			indexer.Rescan() // the engine processes again only the kinds of file whose choice changed
		}
		if !slices.Equal(old.Roots, updated.Roots) || !slices.Equal(old.Exclude, updated.Exclude) {
			indexer.SetFolders(updated.Roots, updated.Exclude)
			surveys.Set(updated.Roots, updated.Exclude)
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
		if firstRun {
			// The launcher has no window of its own, so a first start would show nothing at all: open the setup
			// guide, which says what this is and where the hotkey is.
			go func() {
				time.Sleep(time.Second)
				setupWindow.Open()
			}()
		} else if afterUpdate {
			// The window the update was started from went with the old launcher: show the new version on its About page.
			go func() {
				time.Sleep(time.Second)
				settingsWindow.OpenAt("about")
			}()
		}
		if st := setup.State(); cfg.Search.Enabled && st.Installed && st.NeedsUpdate {
			// Updating the launcher leaves the engine of the old version on disk; bring it up to date before it runs.
			log.Print("file search is from an older version; updating it")
			engineSwitch.Off()
			if err := setup.Start(); err != nil {
				log.Printf("updating file search: %v", err)
			}
		} else if engineWanted(cfg) {
			engineSwitch.On()
		} else {
			engineSwitch.Off()
		}
		if hotkey := cfg.Clipboard.Hotkey; hotkey != "" {
			// Registering a hotkey asks the main thread, which is the thread running this handler: from a goroutine, like
			// the panel's own hotkey, or the launcher would wait for itself.
			go func() {
				if err := bar.RebindFill(hotkey, "clip: "); err != nil {
					log.Print(err)
					bar.SetFillError(err.Error())
				}
			}()
		}
		go indexer.Run(ctx)
		go surveys.Run(ctx)
		go clips.Run(ctx)
		go checker.Run(ctx, func() bool { return store.Get().CheckUpdates })
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

// indexOptions are the indexing choices of the settings in the engine's terms.
func indexOptions(s config.Search) sidecar.IndexOptions {
	return sidecar.IndexOptions{PDFPages: s.PDFPages, Photos: s.Photos, Videos: s.Videos}
}

// statusLine is the one sentence the tray shows about the launcher, most urgent problem first.
func statusLine(hotkeyError string, engine sidecar.Status, index indexing.Status, held bool) string {
	switch {
	case hotkeyError != "":
		return hotkeyError
	case engine.State == sidecar.StateOff:
		return "File search is off. Open Settings to turn it on."
	case engine.State == sidecar.StateFailed:
		return engine.Message
	case engine.State != sidecar.StateReady:
		return "Search is starting up..."
	case held:
		return "File search is ready. Open Settings to choose what to index."
	}
	return index.String()
}

// instanceID is what makes a second launcher hand over to the first one. AGI_INSTANCE (for checks in a clean profile,
// beside the launcher in use, with APPDATA and LOCALAPPDATA pointed at an empty folder) gives the second its own.
func instanceID() string {
	id := "com.adityuhh11.agi.launcher"
	if other := os.Getenv("AGI_INSTANCE"); other != "" {
		id += "." + other
	}
	return id
}
