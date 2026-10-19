package settingsui

import (
	"errors"
	"path/filepath"
	"strings"
	"testing"

	"agi/internal/config"
	"agi/internal/enginesetup"
	"agi/internal/indexing"
	"agi/internal/settings"
)

// fakeSetup plays the installer: it is installed or not, and says what it was asked to do.
type fakeSetup struct {
	state      enginesetup.State
	started    int
	removed    int
	cancels    int
	failRemove error
}

func (f *fakeSetup) Installed() bool          { return f.state.Installed }
func (f *fakeSetup) Start() error             { f.started++; f.state.Installing = true; return nil }
func (f *fakeSetup) Cancel()                  { f.cancels++ }
func (f *fakeSetup) State() enginesetup.State { return f.state }
func (f *fakeSetup) Remove() error            { f.removed++; return f.failRemove }

// fakeIndexing plays the indexing manager.
type fakeIndexing struct {
	status  indexing.Status
	folders []indexing.FolderStatus
	paused  bool
}

func (f *fakeIndexing) Pause()                           { f.paused = true; f.status.Paused = true }
func (f *fakeIndexing) Resume()                          { f.paused = false; f.status.Paused = false }
func (f *fakeIndexing) Status() indexing.Status          { return f.status }
func (f *fakeIndexing) Folders() []indexing.FolderStatus { return f.folders }

type harness struct {
	index   *fakeIndexing
	setup   *fakeSetup
	svc     *Service
	store   *settings.Store
	hotkeys []string // every accelerator Rebind was asked for, in order
	picked  string
	opened  string
	startup bool
}

func newHarness(t *testing.T, refuse ...string) *harness {
	t.Helper()
	store, err := settings.Open(filepath.Join(t.TempDir(), "AGI", "config.toml"))
	if err != nil {
		t.Fatal(err)
	}
	h := &harness{store: store, setup: &fakeSetup{}, index: &fakeIndexing{}}
	h.svc = New(Options{
		Store:    store,
		Setup:    h.setup,
		Indexing: h.index,
		Rebind: func(a string) error {
			h.hotkeys = append(h.hotkeys, a)
			for _, r := range refuse {
				if a == r {
					return errors.New("the hotkey " + a + " cannot be used: taken")
				}
			}
			return nil
		},
		PickFolder:   func() (string, error) { return h.picked, nil },
		Browsers:     func() []string { return []string{"chrome"} },
		OpenFolder:   func(p string) error { h.opened = p; return nil },
		Engine:       func() (string, string) { return "off", "" },
		Autostart:    func() bool { return h.startup },
		SetAutostart: func(on bool) error { h.startup = on; return nil },
		LogDir:       `C:\logs`,
		Version:      "test",
	})
	return h
}

func TestSavingTheGeneralSettingsRebindsTheHotkeyAndWritesTheRest(t *testing.T) {
	h := newHarness(t)
	if msg := h.svc.SaveGeneral(" Ctrl+Alt+K ", "zen", "https://duckduckgo.com/?q={q}"); msg != "" {
		t.Fatal(msg)
	}
	got := h.store.Get()
	if got.Hotkey != "Ctrl+Alt+K" || got.Browser != "zen" || got.SearchURL != "https://duckduckgo.com/?q={q}" {
		t.Fatalf("saved %+v", got)
	}
	if len(h.hotkeys) != 1 || h.hotkeys[0] != "Ctrl+Alt+K" {
		t.Fatalf("rebind calls = %v", h.hotkeys)
	}
}

func TestAHotkeyThatCannotBeBoundIsRefusedAndNothingIsSaved(t *testing.T) {
	h := newHarness(t, "Ctrl+Alt+Delete")
	msg := h.svc.SaveGeneral("Ctrl+Alt+Delete", "zen", "")
	if !strings.Contains(msg, "cannot be used") {
		t.Fatalf("message = %q", msg)
	}
	if got := h.store.Get(); got.Hotkey != config.DefaultHotkey || got.Browser != "" {
		t.Fatalf("a refused change was saved: %+v", got)
	}
}

func TestAnInvalidChangePutsTheOldHotkeyBack(t *testing.T) {
	h := newHarness(t)
	msg := h.svc.SaveGeneral("Ctrl+Alt+K", "", "example.com/?q={q}")
	if !strings.Contains(msg, "must start with https://") {
		t.Fatalf("message = %q", msg)
	}
	if want := []string{"Ctrl+Alt+K", config.DefaultHotkey}; strings.Join(h.hotkeys, "|") != strings.Join(want, "|") {
		t.Fatalf("rebind calls = %v, want %v (new, then back)", h.hotkeys, want)
	}
	if h.store.Get().Hotkey != config.DefaultHotkey {
		t.Fatal("the hotkey in the file changed")
	}
}

func TestQuicklinksAreValidatedWithTheSameRulesAsTheFile(t *testing.T) {
	h := newHarness(t)
	bad := []config.Quicklink{{Trigger: "g h", Title: "x", URL: "https://x/?q={q}"}}
	if msg := h.svc.SaveQuicklinks(bad); !strings.Contains(msg, "one word") {
		t.Fatalf("message = %q", msg)
	}
	good := []config.Quicklink{{Trigger: "gh", Title: "GitHub", URL: "https://github.com/search?q={q}"}}
	if msg := h.svc.SaveQuicklinks(good); msg != "" {
		t.Fatal(msg)
	}
	if got := h.svc.View().Quicklinks; len(got) != 1 || got[0].Trigger != "gh" {
		t.Fatalf("view quicklinks = %+v", got)
	}
}

func TestFoldersAreAddedFromThePickerAndRemovedWithTheirExclusions(t *testing.T) {
	h := newHarness(t)
	h.picked = `C:\Docs\`
	if msg := h.svc.AddFolder(); msg != "" {
		t.Fatal(msg)
	}
	if roots := h.store.Get().Roots; len(roots) != 1 || roots[0] != `C:\Docs` {
		t.Fatalf("roots = %v", roots)
	}
	if msg := h.svc.AddFolder(); !strings.Contains(msg, "repeats") {
		t.Fatalf("adding the same folder twice: %q", msg)
	}
	h.picked = ""
	if msg := h.svc.AddFolder(); msg != "" || len(h.store.Get().Roots) != 1 {
		t.Fatalf("cancelling the picker changed something: %q, %v", msg, h.store.Get().Roots)
	}
	if err := h.store.Update(func(c *config.Config) { c.Exclude = []string{`C:\Docs\vendor`} }); err != nil {
		t.Fatal(err)
	}
	if msg := h.svc.RemoveFolder(`c:\docs`); msg != "" {
		t.Fatal(msg)
	}
	if got := h.store.Get(); len(got.Roots) != 0 || len(got.Exclude) != 0 {
		t.Fatalf("after removing: %+v", got)
	}
}

func TestPaceMustBeOneOfTheKnownSpeeds(t *testing.T) {
	h := newHarness(t)
	if msg := h.svc.SetPace("turbo"); !strings.Contains(msg, "must be one of") {
		t.Fatalf("message = %q", msg)
	}
	if msg := h.svc.SetPace("gentle"); msg != "" || h.store.Get().Search.Pace != "gentle" {
		t.Fatalf("SetPace(gentle) = %q, pace %q", msg, h.store.Get().Search.Pace)
	}
}

func TestOpenFolderKnowsTheConfigAndLogFoldersOnly(t *testing.T) {
	h := newHarness(t)
	if msg := h.svc.OpenFolder("logs"); msg != "" || h.opened != `C:\logs` {
		t.Fatalf("logs: %q, opened %q", msg, h.opened)
	}
	if msg := h.svc.OpenFolder("config"); msg != "" || filepath.Base(h.opened) != "AGI" {
		t.Fatalf("config: %q, opened %q", msg, h.opened)
	}
	if msg := h.svc.OpenFolder(`C:\Windows`); msg == "" {
		t.Fatal("an arbitrary folder was opened")
	}
}

func TestDiagnosticsNameNoFilesOrFolders(t *testing.T) {
	h := newHarness(t)
	h.picked = `C:\Private\Taxes`
	h.svc.AddFolder()
	text := h.svc.Diagnostics()
	if strings.Contains(text, "Taxes") || !strings.Contains(text, "folders: 1") || !strings.Contains(text, "engine=off") {
		t.Fatalf("diagnostics:\n%s", text)
	}
}

func enable(t *testing.T, h *harness) {
	t.Helper()
	if err := h.store.Update(func(c *config.Config) { c.Search.Enabled = true }); err != nil {
		t.Fatal(err)
	}
}

func TestTurningOnWithTheEngineInstalledSwitchesItOnAtOnce(t *testing.T) {
	h := newHarness(t)
	h.setup.state.Installed = true
	if msg := h.svc.TurnOn(); msg != "" {
		t.Fatal(msg)
	}
	if !h.store.Get().Search.Enabled || h.setup.started != 0 {
		t.Fatalf("enabled %v, installs started %d", h.store.Get().Search.Enabled, h.setup.started)
	}
}

func TestTurningOnWithoutTheEngineStartsTheDownloadAndLeavesSearchOffUntilItEnds(t *testing.T) {
	h := newHarness(t)
	if msg := h.svc.TurnOn(); msg != "" {
		t.Fatal(msg)
	}
	if h.setup.started != 1 || h.store.Get().Search.Enabled {
		t.Fatalf("installs started %d, enabled %v", h.setup.started, h.store.Get().Search.Enabled)
	}
	if msg := h.svc.TurnOn(); !strings.Contains(msg, "already being installed") || h.setup.started != 1 {
		t.Fatalf("a second TurnOn during the install: %q, installs started %d", msg, h.setup.started)
	}
}

func TestUpdatingStopsTheEngineFirstBecauseItsFilesAreInUse(t *testing.T) {
	h := newHarness(t)
	h.setup.state = enginesetup.State{Installed: true, NeedsUpdate: true}
	enable(t, h)
	if msg := h.svc.TurnOn(); msg != "" { // an out-of-date engine is updated, not just switched on
		t.Fatal(msg)
	}
	if h.setup.started != 1 || h.store.Get().Search.Enabled {
		t.Fatalf("installs started %d, enabled %v: the engine must be off while its files are replaced", h.setup.started, h.store.Get().Search.Enabled)
	}
}

func TestTurningOffKeepsTheFilesAndRemovingThemTurnsOffFirst(t *testing.T) {
	h := newHarness(t)
	h.setup.state.Installed = true
	enable(t, h)
	if msg := h.svc.TurnOff(); msg != "" || h.store.Get().Search.Enabled || h.setup.removed != 0 {
		t.Fatalf("TurnOff: %q, enabled %v, removed %d", msg, h.store.Get().Search.Enabled, h.setup.removed)
	}
	enable(t, h)
	if msg := h.svc.RemoveFiles(); msg != "" || h.store.Get().Search.Enabled || h.setup.removed != 1 {
		t.Fatalf("RemoveFiles: %q, enabled %v, removed %d", msg, h.store.Get().Search.Enabled, h.setup.removed)
	}
	h.setup.failRemove = errors.New("the file is in use")
	if msg := h.svc.RemoveFiles(); !strings.Contains(msg, "in use") {
		t.Fatalf("a failed removal was not reported: %q", msg)
	}
}

func TestCancelPassesThroughAndSearchReportsTheInstallAndTheEngine(t *testing.T) {
	h := newHarness(t)
	h.setup.state = enginesetup.State{
		Installing: true,
		Progress:   enginesetup.Progress{Step: 5, Steps: 6, Name: "Downloading the models", Done: 100, Total: 400, Message: "x"},
		Error:      "an earlier failure",
	}
	h.svc.CancelInstall()
	view := h.svc.Search()
	if h.setup.cancels != 1 || !view.Installing || view.Step != 5 || view.StepName != "Downloading the models" ||
		view.Done != 100 || view.Total != 400 || view.Error != "an earlier failure" || view.EngineState != "off" {
		t.Fatalf("cancels %d, view %+v", h.setup.cancels, view)
	}
	if len(view.Hosts) == 0 || view.DownloadMB < 900 || view.Pace != "balanced" {
		t.Fatalf("the consent facts are missing: %+v", view)
	}
}

func TestFoldersAreDescribedInWordsForEachState(t *testing.T) {
	h := newHarness(t)
	h.index.folders = []indexing.FolderStatus{
		{Root: `C:`, State: indexing.Indexing, FilesDone: 40, FilesTotal: 96},
		{Root: `C:`, State: indexing.Indexing},
		{Root: `C:\c`, State: indexing.UpToDate, Files: 12, Chunks: 300},
		{Root: `C:\d`, State: indexing.Failed, Message: "disk on fire"},
		{Root: `C:\e`, State: indexing.Waiting},
	}
	h.setup.state.Installed = true
	h.svc.o.Engine = func() (string, string) { return "ready", "" }
	got := h.svc.Search().Folders
	want := []string{"Indexing: 40 of 96 files", "Looking at what is in it", "12 files searchable", "Could not be indexed: disk on fire", "Waiting"}
	for i, text := range want {
		if got[i].Text != text {
			t.Errorf("folder %d says %q, want %q", i, got[i].Text, text)
		}
	}
	h.svc.o.Engine = func() (string, string) { return "off", "" }
	if got := h.svc.Search().Folders[2]; got.State != "waiting" || !strings.Contains(got.Text, "turned on") {
		t.Errorf("with the engine off a folder says %+v", got)
	}
}

func TestPausingAndResumingIndexingGoToTheManagerAndShowInTheView(t *testing.T) {
	h := newHarness(t)
	h.svc.PauseIndexing()
	if !h.index.paused || !h.svc.Search().Paused {
		t.Fatal("a pause did not reach the manager")
	}
	h.svc.ResumeIndexing()
	if h.index.paused || h.svc.Search().Paused {
		t.Fatal("a resume did not reach the manager")
	}
}

func TestAutostartIsShownAndChanged(t *testing.T) {
	h := newHarness(t)
	if h.svc.View().Autostart {
		t.Fatal("autostart shown as on before it was turned on")
	}
	if msg := h.svc.SetAutostart(true); msg != "" || !h.svc.View().Autostart {
		t.Fatalf("turned on: message %q, view %v", msg, h.svc.View().Autostart)
	}
}
