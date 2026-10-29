// Command omni is the launcher: a global hotkey opens a search bar over the memory engine's index.
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
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/wailsapp/wails/v3/pkg/application"
	"github.com/wailsapp/wails/v3/pkg/events"

	"omni"
	"omni/internal/apps"
	"omni/internal/autostart"
	"omni/internal/chat"
	"omni/internal/clipboard"
	"omni/internal/config"
	"omni/internal/enginesetup"
	"omni/internal/files"
	"omni/internal/hotkey"
	"omni/internal/indexing"
	"omni/internal/llm"
	"omni/internal/notes"
	"omni/internal/palette"
	"omni/internal/panel"
	"omni/internal/power"
	"omni/internal/search"
	"omni/internal/settings"
	"omni/internal/settingsui"
	"omni/internal/sidecar"
	"omni/internal/snippets"
	"omni/internal/soundoffer"
	"omni/internal/state"
	"omni/internal/status"
	"omni/internal/survey"
	"omni/internal/system"
	"omni/internal/timer"
	"omni/internal/tray"
	"omni/internal/update"
	"omni/internal/watcher"
	"omni/internal/web"
	"omni/internal/websearch"
)

// version is the build's version, set by scripts/release.ps1 with -ldflags "-X main.version=...".
var version = "dev"

func main() {
	if runHelper() {
		return
	}
	if len(os.Args) == 2 && os.Args[1] == removeDataFlag {
		if err := runRemoveData(); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		return
	}
	if err := migrateEarlierVersions(); err != nil {
		fmt.Fprintln(os.Stderr, err) // the log is not open yet; the program still starts, with settings of its own
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

// setUpLog sends the log to the file omni.log in config.LogDir() as well as to the
// console, if there is one: the release build has no console window, so the file is the only record.
func setUpLog() (closeLog func()) {
	logDir, err := config.LogDir()
	if err != nil {
		return func() {}
	}
	if err := os.MkdirAll(logDir, 0o755); err != nil {
		return func() {}
	}
	// One earlier log is kept, so the file never grows past about twice this.
	if info, err := os.Stat(filepath.Join(logDir, "omni.log")); err == nil && info.Size() > maxLogBytes {
		_ = os.Rename(filepath.Join(logDir, "omni.log"), filepath.Join(logDir, "omni.old.log"))
	}
	file, err := os.OpenFile(filepath.Join(logDir, "omni.log"), os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return func() {}
	}
	log.SetOutput(io.MultiWriter(file, bestEffort{os.Stderr}))
	log.Printf("omni starting (pid %d)", os.Getpid())
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
	assets, err := fs.Sub(omni.Frontend, "frontend/dist")
	if err != nil {
		return err
	}

	// A second launch (the shortcut clicked again) does not start a second launcher or engine: it
	// exits right inside application.New and the running one shows the search bar instead.
	dataDir, err := config.DataDir()
	if err != nil {
		return err
	}
	logDir, err := config.LogDir()
	if err != nil {
		return err
	}
	var bar *panel.Panel
	iconDir := filepath.Join(dataDir, "icons")
	catalog := apps.NewCatalog(filepath.Join(dataDir, "apps.json"), iconDir)
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
	thumbnails := files.Thumbnails(roots, filepath.Join(dataDir, "thumbs"))
	icons := apps.Icons(catalog.Index, iconDir)
	options := application.Options{
		Name: "Omni",
		Assets: application.AssetOptions{
			Handler: application.AssetFileServerFS(assets),
			// /thumb serves picture thumbnails, /icon the icons of apps; anything else is the page.
			Middleware: func(next http.Handler) http.Handler {
				return thumbnails(icons(next))
			},
		},
		SingleInstance: &application.SingleInstanceOptions{
			UniqueID:               instanceID(),
			OnSecondInstanceLaunch: func(application.SecondInstanceData) { bar.Show() },
		},
	}
	platformOptions(&options)
	app := application.New(options)
	platformSetup(app)

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
	engineSource, err := fs.Sub(omni.EngineSource, "sidecar")
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
	// Counting folders needs no model (POST /survey answers while it loads), so the guide's estimates need not wait for it.
	surveys := survey.New(engine.Client(), func() bool {
		state := engine.Status().State
		return state == sidecar.StateReady || state == sidecar.StateWarming
	})
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
	clipStore, err := clipboard.OpenStore(filepath.Join(dataDir, "clipboard.json"))
	if err != nil {
		return err
	}
	self, err := os.Executable()
	if err != nil {
		return fmt.Errorf("locate this program: %w", err)
	}
	clipEnabled := func() bool { return store.Get().Clipboard.Enabled }
	clips := clipboard.NewManager(clipStore,
		clipboard.NewWatcher(clipboard.NewSource(), clipStore, clipEnabled,
			func() []string { return store.Get().Clipboard.Ignore }, strings.TrimSuffix(filepath.Base(self), ".exe")),
		clipboard.NewPaster(app.Clipboard.SetText, bar.Previous, bar.Hide),
		clipEnabled)
	bar.OnShow = func() {
		clips.Poll() // a copy a moment before the bar opened is in the list
		appsShown(catalog)
	}
	snips := snippets.New(func() []config.Snippet { return store.Get().Snippets },
		clipboard.NewPaster(app.Clipboard.SetText, bar.Previous, bar.Hide),
		func() string { clip, _ := clipboard.NewSource().Read(); return clip.Text })
	settingsWindow := settingsui.NewWindow(app)
	setupWindow := settingsui.NewSetupWindow(app)
	notesWindow := settingsui.NewNotesWindow(app)
	timerWindow := settingsui.NewTimerWindow(app)
	chatWindow := settingsui.NewChatWindow(app, func() bool { return store.Get().Chat.OnTop })
	// A notes folder that cannot be opened (read-only, a strange profile) costs the notes, not the launcher.
	noteStore, err := notes.Open(filepath.Join(filepath.Dir(path), "notes"))
	var noteBook palette.NoteBook
	if err != nil {
		log.Printf("notes are off: %v", err)
		noteStore = nil
	} else {
		noteBook = noteStore // not assigned when nil: a nil *Store inside the interface would not look nil to the bar
	}
	// Omni AI: the model's files and server (internal/llm) and the conversations (internal/chat). It is separate from file
	// search and needs nothing of it. A chats folder that cannot be opened costs the history, not the chat.
	chatLayout, err := llm.Locate(cfg.Search.DataDir)
	if err != nil {
		return err
	}
	var chatSvc *chat.Service // set below; the model reports to it once the app runs
	chatSetup := llm.NewSetup(chatLayout, func(err error) {
		if err != nil {
			if !errors.Is(err, context.Canceled) {
				log.Printf("installing Omni AI failed: %v", err)
			}
			return
		}
		log.Print("Omni AI is installed")
		if err := store.Update(func(c *config.Config) { c.Chat.Enabled = true }); err != nil {
			log.Printf("switching Omni AI on: %v", err)
		}
	})
	chatModel := llm.NewServer(llm.Options{
		Layout:        chatSetup.Layout,
		ContextTokens: func() int { return store.Get().Chat.ContextTokens },
		IdleMinutes:   func() int { return store.Get().Chat.UnloadAfterMinutes },
		OnChange: func(st llm.Status) {
			if chatSvc != nil {
				chatSvc.ModelChanged(st)
			}
		},
		Logf: log.Printf,
	})
	chatStore, err := chat.Open(filepath.Join(filepath.Dir(path), "chats"))
	var chatHistory settingsui.ChatHistory
	if err != nil {
		log.Printf("conversations are not kept: %v", err)
		chatStore = nil
	} else {
		chatHistory = chatStore // not assigned when nil: a nil *Store inside the interface would not look nil
	}
	chatSvc = chat.NewService(chat.Options{
		Store:        chatStore,
		Model:        chatModel,
		Settings:     func() config.Chat { return store.Get().Chat },
		SetThinking:  func(on bool) error { return store.Update(func(c *config.Config) { c.Chat.Thinking = on }) },
		SetWebSearch: func(on bool) error { return store.Update(func(c *config.Config) { c.Chat.WebSearch = on }) },
		Web:          websearch.New(),
		Installed:    func() bool { return chatSetup.Layout().Installed() },
		Emit:         func(event string, data any) { app.Event.Emit(event, data) },
		OpenSettings: func(section string) { settingsWindow.OpenAt(section) },
		Logf:         log.Printf,
	})
	openChat := func() {
		chatWindow.Open()
		chatSvc.Warm()
	}
	// The chat's own hotkey shows the window and closes it again when it is the one in use.
	chatKey := hotkey.NewSlot(app.GlobalShortcut)
	toggleChat := func() {
		chatWindow.Toggle()
		chatSvc.Warm()
	}
	// The notes hotkey works the same way: it shows the window and closes it again when it is the one in use.
	notesKey := hotkey.NewSlot(app.GlobalShortcut)
	// The timer lives here, not in its window: the window opens when one starts and closes when it ends.
	countdown := timer.New(
		func() config.Timer { return store.Get().Timer },
		func(timer.State) { system.Beep() },
		func(active bool) {
			if active {
				timerWindow.Open()
			} else {
				timerWindow.Close()
			}
		})
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
	icon = tray.New(app, bar.Show, notesWindow.Open, openChat, settingsWindow.Open, setupWindow.Open, indexer.Rescan, togglePause)
	for _, service := range []application.Service{
		application.NewService(panel.NewService(bar)),
		application.NewService(status.New(store.Get, bar.HotkeyError, engine.Status, indexer.Status)),
		application.NewService(search.New(engine.Client(), sourceRoots, engine.Status)),
		application.NewService(opener),
		application.NewService(palette.With(palette.New(catalog, history, browser, opener, clips, snips, func(section string) { settingsWindow.OpenAt(section) }, bar.HideForLaunch), palette.Extras{
			Copy: func(text string) error {
				ok := app.Clipboard.SetText(text)
				clipboard.NoteOwnCopy() // the history must not keep what this program copied
				if !ok {
					return errors.New("the clipboard is in use by another program; try again")
				}
				return nil
			},
			RunSystem: system.Run,
			Notes:     noteBook,
			OpenNote:  notesWindow.OpenAt,
			Assistant: chatSvc,
			AskChat: func(text string) {
				chatSvc.Ask(text)
				chatWindow.OpenAt("new")
			},
			OpenChat: chatWindow.OpenAt,
			Timer:    countdown,
			Sound: &soundoffer.Service{
				Settings: store, Surveys: surveys, Waivers: history, Now: time.Now,
				// An engine from before audio search would refuse the sync the choice starts: it is offered after the update.
				Ready: func() bool { return engine.Status().State == sidecar.StateReady && !setup.State().NeedsUpdate },
				OnCPU: func() bool { return engine.Status().Device == "cpu" },
			},
		})),
		application.NewService(notes.NewService(noteStore, notesWindow.SetAlwaysOnTop)),
		application.NewService(timer.NewService(countdown)),
		application.NewService(chatSvc),
		application.NewService(settingsui.NewOmniAI(settingsui.OmniAIOptions{
			Store:        store,
			Setup:        chatSetup,
			Model:        chatModel,
			History:      chatHistory,
			ClearHistory: func() error { _, err := chatSvc.ClearHistory(); return err },
			RebindHotkey: func(accelerator string) error { return chatKey.Rebind(accelerator, toggleChat) },
			HotkeyError:  chatKey.Error,
			OpenFolder:   openFolder,
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
			OpenFolder:           openFolder,
			Clipboard:            clips,
			RebindClipboard:      func(accelerator string) error { return bar.RebindFill(accelerator, "clip: ") },
			ClipboardHotkeyError: bar.FillError,
			RebindNotes:          func(accelerator string) error { return notesKey.Rebind(accelerator, notesWindow.Toggle) },
			NotesHotkeyError:     notesKey.Error,
			Updates:              checker,
			Installer:            update.NewInstaller(filepath.Join(dataDir, "update")),
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
			LogDir:       logDir,
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
		if old.Search.PauseOnBattery != updated.Search.PauseOnBattery {
			indexer.SetPauseOnBattery(updated.Search.PauseOnBattery)
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
		if old.Chat != updated.Chat {
			chatSvc.Refresh()
			icon.SetChat(updated.Chat.Enabled)
			if old.Chat.UnloadAfterMinutes != updated.Chat.UnloadAfterMinutes {
				chatModel.Rearm()
			}
			if old.Chat.Enabled && !updated.Chat.Enabled {
				chatModel.Unload()
			}
			if old.Chat.OnTop != updated.Chat.OnTop {
				chatWindow.SetAlwaysOnTop(updated.Chat.OnTop)
			}
			if updated.Chat.Enabled && updated.Chat.Load == "at_start" && (!old.Chat.Enabled || old.Chat.Load != "at_start") {
				chatSvc.Warm()
			}
		}
		if old.Search.DataDir != updated.Search.DataDir { // the folder chosen for the downloads holds the chat model as well
			if layout, err := llm.Locate(updated.Search.DataDir); err != nil {
				log.Print(err)
			} else if err := chatSetup.Move(layout); err != nil {
				log.Printf("Omni AI stays where it is: %v", err)
			}
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
		icon.SetChat(cfg.Chat.Enabled)
		go chatSvc.Housekeeping()
		if hk := cfg.Notes.Hotkey; hk != "" {
			go func() {
				if err := notesKey.Rebind(hk, notesWindow.Toggle); err != nil {
					log.Print(err)
					notesKey.SetError(err.Error())
				}
			}()
		}
		if hk := cfg.Chat.Hotkey; hk != "" {
			go func() { // like the other hotkeys: registering asks the main thread, which is running this handler
				if err := chatKey.Rebind(hk, toggleChat); err != nil {
					log.Print(err)
					chatKey.SetError(err.Error())
				}
			}()
		}
		if cfg.Chat.Enabled && cfg.Chat.Load == "at_start" {
			go func() {
				time.Sleep(15 * time.Second) // never on the way to the first bar
				chatSvc.Warm()
			}()
		}
		// Set here, not with the other indexer settings: being unplugged reports a change, and the tray is not made yet there.
		indexer.WatchBattery(power.OnBattery)
		indexer.SetPauseOnBattery(cfg.Search.PauseOnBattery)
		go indexer.Run(ctx)
		go countdown.Run(ctx)
		go surveys.Run(ctx)
		go clips.Run(ctx)
		go checker.Run(ctx, func() bool { return store.Get().CheckUpdates })
		// The list of installed apps is served from its cache at once and rebuilt in the background,
		// again whenever a program is installed or removed.
		go catalog.Run(ctx)
		go watcher.Watch(ctx, apps.WatchedFolders(), 3*time.Second, 30*time.Second, func(string) { catalog.Refresh() })
		watchFolders(cfg.Roots)
	})

	runErr := app.Run()

	stop()
	chatModel.Unload()
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
	return sidecar.IndexOptions{PDFPages: s.PDFPages, Photos: s.Photos, Videos: s.Videos, Audio: s.Audio}
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

// instanceID is what makes a second launcher hand over to the first one. OMNI_INSTANCE (for checks in a clean profile,
// beside the launcher in use, with APPDATA and LOCALAPPDATA pointed at an empty folder) gives the second its own.
func instanceID() string {
	id := "com.adityuhh11.omni.launcher"
	if other := os.Getenv("OMNI_INSTANCE"); other != "" {
		id += "." + other
	}
	return id
}
