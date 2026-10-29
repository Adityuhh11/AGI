package settingsui

import (
	"errors"
	"path/filepath"
	"testing"
	"time"

	"omni/internal/chat"
	"omni/internal/llm"
	"omni/internal/settings"
)

type fakeChatSetup struct {
	state   llm.InstallState
	started int
	removed int
}

func (f *fakeChatSetup) State() llm.InstallState { return f.state }
func (f *fakeChatSetup) Start() error            { f.started++; return nil }
func (f *fakeChatSetup) Cancel()                 {}
func (f *fakeChatSetup) Remove() error           { f.removed++; return nil }
func (f *fakeChatSetup) Layout() llm.Layout      { return llm.Layout{} }

type fakeChatModel struct {
	status   llm.Status
	unloaded int
	rearmed  int
}

func (f *fakeChatModel) Status() llm.Status { return f.status }
func (f *fakeChatModel) Unload()            { f.unloaded++ }
func (f *fakeChatModel) Rearm()             { f.rearmed++ }

type fakeChatHistory struct{ n int }

func (f fakeChatHistory) Dir() string { return `C:\chats` }
func (f fakeChatHistory) Search(string, int) ([]chat.Summary, error) {
	return make([]chat.Summary, f.n), nil
}

type omniRig struct {
	page    *OmniAI
	store   *settings.Store
	setup   *fakeChatSetup
	model   *fakeChatModel
	hotkeys []string
	cleared int
	rebind  error
}

func newOmniRig(t *testing.T) *omniRig {
	t.Helper()
	store, err := settings.Open(filepath.Join(t.TempDir(), "Omni", "config.toml"))
	if err != nil {
		t.Fatal(err)
	}
	r := &omniRig{store: store, setup: &fakeChatSetup{}, model: &fakeChatModel{}}
	r.page = NewOmniAI(OmniAIOptions{
		Store: store, Setup: r.setup, Model: r.model, History: fakeChatHistory{n: 4},
		ClearHistory: func() error { r.cleared++; return nil },
		RebindHotkey: func(a string) error { r.hotkeys = append(r.hotkeys, a); return r.rebind },
		HotkeyError:  func() string { return "" },
		OpenFolder:   func(string) error { return errors.New("no explorer") },
	})
	return r
}

func TestTheViewShowsTheStateTheChoicesAndTheDefaults(t *testing.T) {
	r := newOmniRig(t)
	r.model.status = llm.Status{State: llm.Ready, Device: "gpu", UnloadsAt: time.UnixMilli(5000)}
	v := r.page.View()
	if v.Enabled || v.Installed || v.Load != "when_asked" || v.UnloadAfterMinutes != 10 || v.ContextTokens != 8192 || !v.KeepHistory || v.HistoryDays != 90 {
		t.Fatalf("defaults = %+v", v)
	}
	if v.ModelState != "ready" || v.Device != "gpu" || v.UnloadsAt != 5000 || v.Conversations != 4 || v.HistoryFolder != `C:\chats` {
		t.Fatalf("model and history = %+v", v)
	}
	if len(v.Hosts) != 2 || v.DownloadMB < 1000 || v.LicenseURL == "" || len(v.ContextChoices) != 4 || len(v.Loads) != 2 {
		t.Fatalf("what the page tells before the download = %+v", v)
	}
}

func TestTurnOnInstallsFirstAndOnlySwitchesOnWhenInstalled(t *testing.T) {
	r := newOmniRig(t)
	if msg := r.page.TurnOn(); msg != "" || r.setup.started != 1 || r.store.Get().Chat.Enabled {
		t.Fatalf("not installed: msg %q, started %d, enabled %v; the download starts and main switches on when it ends", msg, r.setup.started, r.store.Get().Chat.Enabled)
	}
	r.setup.state = llm.InstallState{Installed: true}
	if msg := r.page.TurnOn(); msg != "" || !r.store.Get().Chat.Enabled || r.setup.started != 1 {
		t.Fatalf("installed: msg %q, enabled %v, started %d", msg, r.store.Get().Chat.Enabled, r.setup.started)
	}
	r.setup.state = llm.InstallState{Installed: true, NeedsUpdate: true}
	if r.page.TurnOn(); r.setup.started != 2 || r.store.Get().Chat.Enabled {
		t.Fatalf("an out-of-date install is updated first: started %d", r.setup.started)
	}
	r.setup.state = llm.InstallState{Installing: true}
	if msg := r.page.TurnOn(); msg == "" {
		t.Fatal("a second download was started")
	}
}

func TestTurnOffUnloadsAndRemoveDeletesTheFiles(t *testing.T) {
	r := newOmniRig(t)
	r.setup.state = llm.InstallState{Installed: true}
	r.page.TurnOn()
	if msg := r.page.TurnOff(); msg != "" || r.store.Get().Chat.Enabled || r.model.unloaded != 1 {
		t.Fatalf("TurnOff: %q, enabled %v, unloaded %d", msg, r.store.Get().Chat.Enabled, r.model.unloaded)
	}
	if msg := r.page.Remove(); msg != "" || r.setup.removed != 1 || r.model.unloaded != 2 {
		t.Fatalf("Remove: %q, removed %d, unloaded %d", msg, r.setup.removed, r.model.unloaded)
	}
}

func TestBehaviourIsValidatedAndANewContextSizeUnloadsTheModel(t *testing.T) {
	r := newOmniRig(t)
	if msg := r.page.SetBehaviour("at_start", 30, 16384); msg != "" {
		t.Fatal(msg)
	}
	got := r.store.Get().Chat
	if got.Load != "at_start" || got.UnloadAfterMinutes != 30 || got.ContextTokens != 16384 || r.model.unloaded != 1 {
		t.Fatalf("chat = %+v, unloaded %d", got, r.model.unloaded)
	}
	if r.page.SetBehaviour("at_start", 5, 16384); r.model.unloaded != 1 {
		t.Fatal("the model was unloaded though the context size did not change")
	}
	for name, msg := range map[string]string{
		"load":    r.page.SetBehaviour("always", 5, 8192),
		"minutes": r.page.SetBehaviour("when_asked", 7, 8192),
		"context": r.page.SetBehaviour("when_asked", 5, 5000),
		"days":    r.page.SetHistory(true, 400),
	} {
		if msg == "" {
			t.Errorf("%s: an invalid value was accepted", name)
		}
	}
	if got := r.store.Get().Chat; got.Load != "at_start" || got.ContextTokens != 16384 {
		t.Fatalf("a refused change must leave the settings alone: %+v", got)
	}
}

func TestTheHotkeyIsBoundBeforeItIsSavedAndARefusedOneChangesNothing(t *testing.T) {
	r := newOmniRig(t)
	if msg := r.page.SetHotkey(" Ctrl+Alt+A "); msg != "" || r.store.Get().Chat.Hotkey != "Ctrl+Alt+A" || len(r.hotkeys) != 1 {
		t.Fatalf("msg %q, hotkey %q, bound %v", msg, r.store.Get().Chat.Hotkey, r.hotkeys)
	}
	r.rebind = errors.New("another program owns it")
	if msg := r.page.SetHotkey("Ctrl+Alt+B"); msg == "" || r.store.Get().Chat.Hotkey != "Ctrl+Alt+A" {
		t.Fatalf("a hotkey that cannot be bound was saved: %q", r.store.Get().Chat.Hotkey)
	}
	r.rebind = nil
	before := len(r.hotkeys)
	if msg := r.page.SetHotkey("alt+shift+space"); msg == "" || len(r.hotkeys) != before {
		t.Fatalf("the launcher's own hotkey must be refused before it is registered: %q, bound %v", msg, r.hotkeys)
	}
	if msg := r.page.SetHotkey(""); msg != "" || r.store.Get().Chat.Hotkey != "" {
		t.Fatalf("turning it off: %q", msg)
	}
}

func TestHistorySettingsAndClearing(t *testing.T) {
	r := newOmniRig(t)
	if msg := r.page.SetHistory(false, 30); msg != "" || r.store.Get().Chat.KeepHistory || r.store.Get().Chat.HistoryDays != 30 {
		t.Fatalf("SetHistory: %q %+v", msg, r.store.Get().Chat)
	}
	if msg := r.page.ClearHistory(); msg != "" || r.cleared != 1 {
		t.Fatalf("ClearHistory: %q, cleared %d", msg, r.cleared)
	}
	if msg := r.page.OpenHistoryFolder(); msg != "no explorer" {
		t.Fatalf("OpenHistoryFolder = %q", msg)
	}
	if msg := r.page.SetThinking(true); msg != "" || !r.store.Get().Chat.Thinking {
		t.Fatalf("SetThinking: %q", msg)
	}
	if !r.page.View().OnTop {
		t.Fatal("the chat window stays on top by default")
	}
	if msg := r.page.SetOnTop(false); msg != "" || r.store.Get().Chat.OnTop || r.page.View().OnTop {
		t.Fatalf("SetOnTop: %q", msg)
	}
}
