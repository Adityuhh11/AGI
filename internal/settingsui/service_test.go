package settingsui

import (
	"context"
	"errors"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"omni/internal/config"
	"omni/internal/enginesetup"
	"omni/internal/indexing"
	"omni/internal/settings"
	"omni/internal/sidecar"
	"omni/internal/survey"
	"omni/internal/testpath"
	"omni/internal/update"
)

// fakeSetup plays the installer: it is installed or not, and says what it was asked to do.
type fakeSetup struct {
	state        enginesetup.State
	started      int
	removed      int
	indexRemoved int
	cancels      int
	failRemove   error
}

func (f *fakeSetup) Installed() bool          { return f.state.Installed }
func (f *fakeSetup) Start() error             { f.started++; f.state.Installing = true; return nil }
func (f *fakeSetup) Cancel()                  { f.cancels++ }
func (f *fakeSetup) State() enginesetup.State { return f.state }
func (f *fakeSetup) Remove() error            { f.removed++; return f.failRemove }
func (f *fakeSetup) RemoveIndex() error       { f.indexRemoved++; return nil }

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

// fakeUpdates plays the update checker.
type fakeUpdates struct {
	rel    update.Release
	newer  bool
	err    error
	looked bool
}

func (f *fakeUpdates) Check(context.Context) (update.Release, bool, error) {
	f.looked = f.newer && f.err == nil
	return f.rel, f.newer, f.err
}

func (f *fakeUpdates) Found() (update.Release, bool) {
	if f.looked {
		return f.rel, true
	}
	return update.Release{}, false
}

// fakeHistory plays the clipboard history.
type fakeHistory struct{ count, cleared int }

func (f *fakeHistory) Len() int     { return f.count }
func (f *fakeHistory) Path() string { return "C:/cache/clipboard.json" }
func (f *fakeHistory) Clear() error { f.count = 0; f.cleared++; return nil }

// fakeSurveys plays the survey manager.
type fakeSurveys struct{ results map[string]survey.Result }

func (f *fakeSurveys) Results() map[string]survey.Result { return f.results }

// fakePlace plays the install manager's folder: where file search lives, and a refusal to move it.
type fakePlace struct {
	layout enginesetup.Layout
	refuse error
}

func (f *fakePlace) Layout() enginesetup.Layout { return f.layout }
func (f *fakePlace) Move(to enginesetup.Layout) error {
	if f.refuse != nil {
		return f.refuse
	}
	f.layout = to
	return nil
}

type harness struct {
	place           *fakePlace
	surveys         *fakeSurveys
	installer       *fakeInstaller
	restarts        int
	restartErr      error
	history         *fakeHistory
	clipHotkeys     []string // every accelerator RebindClipboard was asked for, in order
	clipHotkeyError string
	noteHotkeys     []string // every accelerator RebindNotes was asked for, in order
	updates         *fakeUpdates
	openedURL       string
	index           *fakeIndexing
	setup           *fakeSetup
	svc             *Service
	store           *settings.Store
	hotkeys         []string // every accelerator Rebind was asked for, in order
	picked          string
	opened          string
	startup         bool
}

func newHarness(t *testing.T, refuse ...string) *harness {
	t.Helper()
	store, err := settings.Open(filepath.Join(t.TempDir(), "Omni", "config.toml"))
	if err != nil {
		t.Fatal(err)
	}
	h := &harness{place: &fakePlace{layout: enginesetup.Layout{Root: testpath.P(`C:\default\engine`), Memory: testpath.P(`C:\default\memory`)}}, surveys: &fakeSurveys{}, store: store, setup: &fakeSetup{}, index: &fakeIndexing{}, updates: &fakeUpdates{}, installer: &fakeInstaller{}, history: &fakeHistory{count: 3}}
	h.svc = New(Options{
		Store:    store,
		Setup:    h.setup,
		Indexing: h.index,
		Surveys:  h.surveys,
		Place:    h.place,
		Rebind: func(a string) error {
			h.hotkeys = append(h.hotkeys, a)
			for _, r := range refuse {
				if a == r {
					return errors.New("the hotkey " + a + " cannot be used: taken")
				}
			}
			return nil
		},
		PickFolder: func() (string, error) { return h.picked, nil },
		Browsers:   func() []string { return []string{"chrome"} },
		OpenFolder: func(p string) error { h.opened = p; return nil },
		Engine:     func() sidecar.Status { return sidecar.Status{State: sidecar.StateOff} },
		Clipboard:  h.history,
		RebindClipboard: func(a string) error {
			h.clipHotkeys = append(h.clipHotkeys, a)
			for _, r := range refuse {
				if a == r {
					return errors.New("the hotkey " + a + " cannot be used: taken")
				}
			}
			return nil
		},
		ClipboardHotkeyError: func() string { return h.clipHotkeyError },
		RebindNotes: func(a string) error {
			h.noteHotkeys = append(h.noteHotkeys, a)
			for _, r := range refuse {
				if a == r {
					return errors.New("the hotkey " + a + " cannot be used: taken")
				}
			}
			return nil
		},
		NotesHotkeyError: func() string { return "" },
		Updates:          h.updates,
		Installer:        h.installer,
		ExePath:          "C:/app/omni.exe",
		Restart:          func() error { h.restarts++; return h.restartErr },
		OpenURL:          func(a string) error { h.openedURL = a; return nil },
		Autostart:        func() bool { return h.startup },
		SetAutostart:     func(on bool) error { h.startup = on; return nil },
		LogDir:           testpath.P(`C:\logs`),
		Version:          "test",
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
	h.picked = testpath.P(`C:\Docs\`)
	if msg := h.svc.AddFolder(); msg != "" {
		t.Fatal(msg)
	}
	if roots := h.store.Get().Roots; len(roots) != 1 || roots[0] != testpath.P(`C:\Docs`) {
		t.Fatalf("roots = %v", roots)
	}
	if msg := h.svc.AddFolder(); !strings.Contains(msg, "repeats") {
		t.Fatalf("adding the same folder twice: %q", msg)
	}
	h.picked = ""
	if msg := h.svc.AddFolder(); msg != "" || len(h.store.Get().Roots) != 1 {
		t.Fatalf("cancelling the picker changed something: %q, %v", msg, h.store.Get().Roots)
	}
	if err := h.store.Update(func(c *config.Config) { c.Exclude = []string{testpath.P(`C:\Docs\vendor`)} }); err != nil {
		t.Fatal(err)
	}
	if msg := h.svc.RemoveFolder(testpath.P(`C:\Docs`)); msg != "" {
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
	if msg := h.svc.OpenFolder("logs"); msg != "" || h.opened != testpath.P(`C:\logs`) {
		t.Fatalf("logs: %q, opened %q", msg, h.opened)
	}
	if msg := h.svc.OpenFolder("config"); msg != "" || filepath.Base(h.opened) != "Omni" {
		t.Fatalf("config: %q, opened %q", msg, h.opened)
	}
	if msg := h.svc.OpenFolder(testpath.P(`C:\Windows`)); msg == "" {
		t.Fatal("an arbitrary folder was opened")
	}
}

func TestDiagnosticsNameNoFilesOrFolders(t *testing.T) {
	h := newHarness(t)
	h.picked = testpath.P(`C:\Private\Taxes`)
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

func TestTurningOnBeforeAnyFolderIsChosenHoldsIndexingAndAfterwardsDoesNot(t *testing.T) {
	h := newHarness(t)
	h.setup.state.Installed = true
	if msg := h.svc.TurnOn(); msg != "" || !h.store.Get().Search.HoldIndexing {
		t.Fatalf("no folders yet: %q, hold %v", msg, h.store.Get().Search.HoldIndexing)
	}
	h = newHarness(t)
	h.setup.state.Installed = true
	_ = h.svc.SetRoots([]string{testpath.P(`C:\a`)})
	if msg := h.svc.TurnOn(); msg != "" || h.store.Get().Search.HoldIndexing {
		t.Fatalf("folders already chosen (file search was switched off): %q, hold %v", msg, h.store.Get().Search.HoldIndexing)
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

func TestAFirstDownloadHoldsIndexingUntilItIsStartedAndAnUpdateDoesNot(t *testing.T) {
	h := newHarness(t)
	if msg := h.svc.TurnOn(); msg != "" {
		t.Fatal(msg)
	}
	if !h.store.Get().Search.HoldIndexing || !h.svc.Search().HoldIndexing {
		t.Fatal("a first download must hold indexing, so the choices come first")
	}
	h.index.folders = []indexing.FolderStatus{{Root: testpath.P(`C:\a`), State: indexing.Waiting}}
	h.setup.state = enginesetup.State{Installed: true}
	enable(t, h)
	if got := h.svc.Search().Folders; len(got) != 1 {
		t.Fatalf("folders = %+v", got)
	}
	if msg := h.svc.StartIndexing(); msg != "" || h.store.Get().Search.HoldIndexing {
		t.Fatalf("StartIndexing: %q, hold %v", msg, h.store.Get().Search.HoldIndexing)
	}

	h = newHarness(t)
	h.setup.state = enginesetup.State{Installed: true, NeedsUpdate: true}
	if msg := h.svc.Update(); msg != "" || h.store.Get().Search.HoldIndexing {
		t.Fatalf("an update must not hold indexing: %q, hold %v", msg, h.store.Get().Search.HoldIndexing)
	}
}

func TestWhileHeldTheFoldersSayTheyWaitForTheStartAndTheViewCountsTheIndex(t *testing.T) {
	h := newHarness(t)
	h.setup.state.Installed = true
	h.index.folders = []indexing.FolderStatus{
		{Root: testpath.P(`C:\a`), State: indexing.UpToDate, Files: 7},
		{Root: testpath.P(`C:\b`), State: indexing.UpToDate, Files: 5},
	}
	h.svc.o.Engine = func() sidecar.Status { return sidecar.Status{State: sidecar.StateReady} }
	if err := h.store.Update(func(c *config.Config) { c.Search.Enabled, c.Search.HoldIndexing = true, true }); err != nil {
		t.Fatal(err)
	}
	view := h.svc.Search()
	if view.Indexed != 12 {
		t.Fatalf("Indexed = %d, want 12", view.Indexed)
	}
	if len(view.Folders) != 2 || view.Folders[0].Text != "Waits for you to start indexing" {
		t.Fatalf("folders = %+v", view.Folders)
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
		{Root: testpath.P(`C:\a`), State: indexing.Indexing, FilesDone: 40, FilesTotal: 96},
		{Root: testpath.P(`C:\b`), State: indexing.Indexing},
		{Root: testpath.P(`C:\c`), State: indexing.UpToDate, Files: 12, Chunks: 300},
		{Root: testpath.P(`C:\d`), State: indexing.Failed, Message: "disk on fire"},
		{Root: testpath.P(`C:\e`), State: indexing.Waiting},
	}
	h.setup.state.Installed = true
	h.svc.o.Engine = func() sidecar.Status { return sidecar.Status{State: sidecar.StateReady} }
	got := h.svc.Search().Folders
	want := []string{"Indexing: 40 of 96 files", "Looking at what is in it", "12 files searchable", "Could not be indexed: disk on fire", "Waiting"}
	for i, text := range want {
		if got[i].Text != text {
			t.Errorf("folder %d says %q, want %q", i, got[i].Text, text)
		}
	}
	h.svc.o.Engine = func() sidecar.Status { return sidecar.Status{State: sidecar.StateOff} }
	if got := h.svc.Search().Folders[2]; got.State != "waiting" || !strings.Contains(got.Text, "turned on") {
		t.Errorf("with the engine off a folder says %+v", got)
	}
}

func TestAFolderWithNothingReadableSaysSoAndWhereWindowsMovedIt(t *testing.T) {
	h := newHarness(t)
	h.index.folders = []indexing.FolderStatus{
		{Root: testpath.P(`C:\Users\me\Documents`), State: indexing.UpToDate},
		{Root: testpath.P(`C:\Empty`), State: indexing.UpToDate},
	}
	h.svc.o.Engine = func() sidecar.Status { return sidecar.Status{State: sidecar.StateReady} }
	h.svc.movedTo = func(root string) string {
		if root == testpath.P(`C:\Users\me\Documents`) {
			return testpath.P(`C:\Users\me\OneDrive\Documents`)
		}
		return ""
	}
	got := h.svc.Search().Folders
	if want := "Nothing here Omni can read. Windows keeps this folder in " + testpath.P(`C:\Users\me\OneDrive\Documents`) + " now: add that one instead"; got[0].Text != want {
		t.Errorf("a moved folder says %q", got[0].Text)
	}
	if got[1].Text != "Nothing here Omni can read" {
		t.Errorf("an empty folder says %q", got[1].Text)
	}
}

func TestTheIndexingChoicesAreShownSavedAndValidated(t *testing.T) {
	h := newHarness(t)
	h.svc.o.Engine = func() sidecar.Status { return sidecar.Status{State: sidecar.StateReady, Device: "cpu"} }
	view := h.svc.Search()
	if view.PDFPages != "auto" || view.Photos != "content" || view.Videos != "name" || view.Audio != "name" || view.Device != "cpu" {
		t.Fatalf("view = %+v", view)
	}
	if msg := h.svc.SetIndexing("text", "name", "content", "content"); msg != "" {
		t.Fatal(msg)
	}
	if s := h.store.Get().Search; s.PDFPages != "text" || s.Photos != "name" || s.Videos != "content" || s.Audio != "content" {
		t.Fatalf("saved %+v", s)
	}
	if msg := h.svc.SetIndexing("some", "name", "content", "content"); !strings.Contains(msg, "pdf_pages") {
		t.Fatalf("a bad choice was not refused: %q", msg)
	}
	if msg := h.svc.SetIndexing("auto", "name", "content", "on"); !strings.Contains(msg, "audio") {
		t.Fatalf("a bad audio choice was not refused: %q", msg)
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

func TestUpdateCheckReportsWhatGitHubSaid(t *testing.T) {
	h := newHarness(t)
	h.updates.rel, h.updates.newer = update.Release{Version: "0.3.0", URL: "https://github.com/Adityuhh11/Omni/releases/tag/v0.3.0"}, true
	if got := h.svc.CheckUpdate(); !got.Available || got.Latest != "0.3.0" || got.Error != "" {
		t.Fatalf("got %+v", got)
	}
	if got := h.svc.View().UpdateFound; got != "0.3.0" {
		t.Fatalf("the page would not show the update it was just told about: %q", got)
	}
	h.updates.err = errors.New("no network")
	if got := h.svc.CheckUpdate(); got.Error == "" || got.Available {
		t.Fatalf("a failed check looked like %+v", got)
	}
}

func TestOpenUpdateOpensOnlyWhatACheckFound(t *testing.T) {
	h := newHarness(t)
	if msg := h.svc.OpenUpdate(); msg == "" || h.openedURL != "" {
		t.Fatalf("opened %q although nothing was found (message %q)", h.openedURL, msg)
	}
	h.updates.rel, h.updates.newer = update.Release{Version: "0.3.0", URL: "https://example.test/release"}, true
	h.svc.CheckUpdate()
	if msg := h.svc.OpenUpdate(); msg != "" || h.openedURL != "https://example.test/release" {
		t.Fatalf("message %q, opened %q", msg, h.openedURL)
	}
}

func TestCheckUpdatesIsSavedAndStartsOff(t *testing.T) {
	h := newHarness(t)
	if h.svc.View().CheckUpdates {
		t.Fatal("the update check is on before anyone asked for it")
	}
	if msg := h.svc.SetCheckUpdates(true); msg != "" || !h.store.Get().CheckUpdates {
		t.Fatalf("message %q, saved %v", msg, h.store.Get().CheckUpdates)
	}
}

func TestTheClipboardPageShowsTheSwitchTheCountAndWhatIsLeftOut(t *testing.T) {
	h := newHarness(t)
	v := h.svc.Clipboard()
	if v.Enabled || v.Count != 3 || len(v.Ignore) != 0 || len(v.Defaults) == 0 || v.File == "" {
		t.Fatalf("a fresh install's page: %+v (the history is off, the defaults are listed)", v)
	}
}

func TestSavingTheClipboardSettingsKeepsTrimmedNamesAndDropsBlankLines(t *testing.T) {
	h := newHarness(t)
	if msg := h.svc.SaveClipboard(true, []string{" MyVault.exe ", "", "  ", "Notepad"}, ""); msg != "" {
		t.Fatal(msg)
	}
	got := h.store.Get().Clipboard
	if !got.Enabled || strings.Join(got.Ignore, "|") != "MyVault.exe|Notepad" {
		t.Fatalf("saved %+v", got)
	}
	if msg := h.svc.SaveClipboard(false, nil, ""); msg != "" || h.store.Get().Clipboard.Enabled {
		t.Fatalf("switching off: %q, %+v", msg, h.store.Get().Clipboard)
	}
}

func TestClearingTheClipboardForgetsEverything(t *testing.T) {
	h := newHarness(t)
	if msg := h.svc.ClearClipboard(); msg != "" || h.history.cleared != 1 || h.svc.Clipboard().Count != 0 {
		t.Fatalf("message %q, cleared %d, count %d", msg, h.history.cleared, h.svc.Clipboard().Count)
	}
}

func TestSavingSnippetsStoresThemAndRefusesWhatTheConfigRefuses(t *testing.T) {
	h := newHarness(t)
	good := []config.Snippet{{Name: "Address", Keyword: "addr", Text: "12 Example Street\nSpringfield"}}
	if msg := h.svc.SaveSnippets(good); msg != "" {
		t.Fatal(msg)
	}
	if got := h.svc.View().Snippets; len(got) != 1 || got[0].Text != good[0].Text {
		t.Fatalf("the page would show %+v", got)
	}
	twice := []config.Snippet{{Name: "A", Text: "x"}, {Name: "a", Text: "y"}}
	if msg := h.svc.SaveSnippets(twice); !strings.Contains(msg, "used twice") {
		t.Fatalf("message %q", msg)
	}
	if got := h.store.Get().Snippets; len(got) != 1 || got[0].Name != "Address" {
		t.Fatalf("a refused save changed the snippets: %+v", got)
	}
}

func TestTheClipboardHotkeyIsRebornBeforeItIsSavedAndARefusedOneChangesNothing(t *testing.T) {
	h := newHarness(t, "Ctrl+Shift+C")
	if msg := h.svc.SaveClipboard(true, nil, " Ctrl+Shift+V "); msg != "" {
		t.Fatal(msg)
	}
	if got := h.store.Get().Clipboard.Hotkey; got != "Ctrl+Shift+V" || h.svc.Clipboard().Hotkey != "Ctrl+Shift+V" {
		t.Fatalf("saved %q", got)
	}
	if msg := h.svc.SaveClipboard(true, nil, "Ctrl+Shift+C"); !strings.Contains(msg, "Ctrl+Shift+C") {
		t.Fatalf("message %q: the combination that cannot be used must be named", msg)
	}
	if got := h.store.Get().Clipboard.Hotkey; got != "Ctrl+Shift+V" {
		t.Fatalf("a refused hotkey changed the saved one to %q", got)
	}
	if msg := h.svc.SaveClipboard(true, nil, ""); msg != "" || h.store.Get().Clipboard.Hotkey != "" {
		t.Fatalf("turning the hotkey off: %q, %q", msg, h.store.Get().Clipboard.Hotkey)
	}
	if last := h.clipHotkeys[len(h.clipHotkeys)-1]; last != "" {
		t.Fatalf("the last rebind was %q, want the hotkey turned off", last)
	}
}

func TestAnUnchangedClipboardHotkeyIsNotRebound(t *testing.T) {
	h := newHarness(t)
	if msg := h.svc.SaveClipboard(true, nil, "Ctrl+Shift+V"); msg != "" {
		t.Fatal(msg)
	}
	before := len(h.clipHotkeys)
	if msg := h.svc.SaveClipboard(false, []string{"Notepad"}, "Ctrl+Shift+V"); msg != "" || len(h.clipHotkeys) != before {
		t.Fatalf("message %q, %d rebinds after %d", msg, len(h.clipHotkeys), before)
	}
}

func TestTheHotkeyThatCouldNotBeBoundAtStartIsOnThePage(t *testing.T) {
	h := newHarness(t)
	h.clipHotkeyError = "the hotkey Ctrl+Shift+V cannot be used: taken"
	if got := h.svc.Clipboard().HotkeyError; got != h.clipHotkeyError {
		t.Fatalf("page error %q", got)
	}
}

func TestARefusedSaveGivesTheOldClipboardHotkeyBack(t *testing.T) {
	h := newHarness(t)
	if msg := h.svc.SaveClipboard(true, nil, "Ctrl+Shift+V"); msg != "" {
		t.Fatal(msg)
	}
	// the main hotkey is the one thing the config refuses for the clipboard, though the shortcut manager would take it
	if msg := h.svc.SaveClipboard(true, nil, h.store.Get().Hotkey); msg == "" {
		t.Fatal("the launcher's own hotkey was accepted")
	}
	if last := h.clipHotkeys[len(h.clipHotkeys)-1]; last != "Ctrl+Shift+V" {
		t.Fatalf("the last rebind was %q: the working hotkey must be put back", last)
	}
	if got := h.store.Get().Clipboard.Hotkey; got != "Ctrl+Shift+V" {
		t.Fatalf("saved %q", got)
	}
}

func TestTheViewSaysWhatEachFolderHoldsAndWhatEachChoiceWouldCost(t *testing.T) {
	h := newHarness(t)
	_ = h.svc.SetRoots([]string{testpath.P(`C:\a`), testpath.P(`C:\b`)})
	pdfs := sidecar.Survey{Routes: map[string]sidecar.RouteCount{"pdf": {Files: 10, Bytes: 100 << 20}, "image": {Files: 50, Bytes: 1 << 30}}}
	h.surveys.results = map[string]survey.Result{
		testpath.P(`C:\a`): {State: survey.Done, Survey: pdfs},
		testpath.P(`C:\b`): {State: survey.Running},
	}
	view := h.svc.Search()
	if len(view.Surveys) != 2 || view.Surveys[0].Files != 60 || view.Surveys[0].Seconds == 0 || view.Surveys[1].State != "running" || view.Surveys[1].Seconds != 0 {
		t.Fatalf("surveys = %+v", view.Surveys)
	}
	if view.Estimate.Complete || view.Estimate.Seconds == 0 {
		t.Fatalf("with one folder still being counted the total is partial: %+v", view.Estimate)
	}
	est := view.Estimate
	if !(est.ByChoice["PDFPages:all"] > est.ByChoice["PDFPages:auto"] && est.ByChoice["PDFPages:auto"] > est.ByChoice["PDFPages:text"]) {
		t.Errorf("PDF choices: %v", est.ByChoice)
	}
	if est.ByChoice["Photos:content"] <= est.ByChoice["Photos:name"] {
		t.Errorf("photo choices: %v", est.ByChoice)
	}
	if !(est.ByPace["gentle"] > est.ByPace["balanced"] && est.ByPace["balanced"] > est.ByPace["fast"]) {
		t.Errorf("paces: %v", est.ByPace)
	}
	h.surveys.results[testpath.P(`C:\b`)] = survey.Result{State: survey.Done, Survey: pdfs}
	if !h.svc.Search().Estimate.Complete {
		t.Error("every folder counted: the estimate is complete")
	}
}

func TestRemovingEverythingForgetsTheFoldersAndChoicesButKeepsWhereItGoes(t *testing.T) {
	h := newHarness(t)
	h.setup.state.Installed = true
	enable(t, h)
	if err := h.store.Update(func(c *config.Config) {
		c.Roots, c.Search.DataDir, c.Search.Pace, c.Search.Photos, c.Search.InLauncher = []string{testpath.P(`C:\Docs`)}, testpath.P(`D:\Omni`), "fast", "name", true
	}); err != nil {
		t.Fatal(err)
	}
	if msg := h.svc.RemoveEverything(); msg != "" {
		t.Fatal(msg)
	}
	got := h.store.Get()
	if got.Search.Enabled || len(got.Roots) != 0 || got.Search.Pace != "balanced" || got.Search.Photos != "content" || got.Search.InLauncher {
		t.Errorf("settings were not reset: %+v, roots %v", got.Search, got.Roots)
	}
	if got.Search.DataDir != testpath.P(`D:\Omni`) {
		t.Errorf("the chosen data folder was forgotten: %q", got.Search.DataDir)
	}
	if h.setup.removed != 1 || h.setup.indexRemoved != 1 {
		t.Errorf("engine removed %d times, index %d times, want once each", h.setup.removed, h.setup.indexRemoved)
	}
}

func TestRemovingEverythingWaitsForARunningInstall(t *testing.T) {
	h := newHarness(t)
	h.setup.state.Installing = true
	if msg := h.svc.RemoveEverything(); !strings.Contains(msg, "cancel") || h.setup.removed != 0 {
		t.Fatalf("message %q, removed %d", msg, h.setup.removed)
	}
}

func TestTheFeelAndTimerSettingsAreShownAndChanged(t *testing.T) {
	h := newHarness(t)
	if v := h.svc.View(); !v.SelectionEffect || v.Timer.FocusMinutes != 25 {
		t.Fatalf("view = %+v", v)
	}
	if msg := h.svc.SetSelectionEffect(false); msg != "" || h.svc.View().SelectionEffect {
		t.Fatalf("SetSelectionEffect: %q", msg)
	}
	if msg := h.svc.SetGroupResults(false); msg != "" || h.store.Get().Search.GroupResults {
		t.Fatalf("SetGroupResults: %q", msg)
	}
	want := config.Timer{FocusMinutes: 50, ShortBreakMinutes: 10, LongBreakMinutes: 20, Rounds: 3, AutoNext: true, PresetMinutes: []int{7, 90}}
	if msg := h.svc.SaveTimer(want); msg != "" || !reflect.DeepEqual(h.store.Get().Timer, want) {
		t.Fatalf("SaveTimer: %q, got %+v", msg, h.store.Get().Timer)
	}
	if msg := h.svc.SaveTimer(config.Timer{}); msg == "" || !reflect.DeepEqual(h.store.Get().Timer, want) {
		t.Fatalf("a timer of zero was accepted: %q", msg)
	}
}

func TestTheDailyHoursAndTheThemeAreSavedAndABadValueIsRefused(t *testing.T) {
	h := newHarness(t)
	if msg := h.svc.SetIndexBetween(" 22:00-07:00 "); msg != "" || h.svc.Search().IndexBetween != "22:00-07:00" {
		t.Fatalf("SetIndexBetween: %q, page shows %q", msg, h.svc.Search().IndexBetween)
	}
	if msg := h.svc.SetIndexBetween("tonight"); msg == "" || h.store.Get().Search.IndexBetween != "22:00-07:00" {
		t.Fatalf("a bad window was accepted: %q", msg)
	}
	if msg := h.svc.SetIndexBetween(""); msg != "" || h.store.Get().Search.IndexBetween != "" {
		t.Fatalf("turning the hours off: %q", msg)
	}
	if msg := h.svc.SetTheme("cream"); msg != "" || h.svc.Theme() != "cream" {
		t.Fatalf("SetTheme: %q", msg)
	}
	if msg := h.svc.SetTheme("neon"); msg == "" || h.svc.Theme() != "cream" {
		t.Fatalf("an unknown theme was accepted: %q", msg)
	}
}
