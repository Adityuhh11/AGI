package settingsui

import (
	"errors"
	"path/filepath"
	"strings"
	"testing"

	"agi/internal/config"
	"agi/internal/settings"
)

type harness struct {
	svc     *Service
	store   *settings.Store
	hotkeys []string // every accelerator Rebind was asked for, in order
	picked  string
	opened  string
}

func newHarness(t *testing.T, refuse ...string) *harness {
	t.Helper()
	store, err := settings.Open(filepath.Join(t.TempDir(), "AGI", "config.toml"))
	if err != nil {
		t.Fatal(err)
	}
	h := &harness{store: store}
	h.svc = New(Options{
		Store: store,
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
		Engine:     func() (string, string) { return "off", "" },
		LogDir:     `C:\logs`,
		Version:    "test",
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
