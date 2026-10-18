package panel

import (
	"errors"
	"strings"
	"testing"
)

// fakeKeys plays Wails' shortcut manager: a set of registered accelerators, some of them owned by others.
type fakeKeys struct {
	taken      map[string]bool
	registered map[string]bool
}

func newFakeKeys(taken ...string) *fakeKeys {
	k := &fakeKeys{taken: map[string]bool{}, registered: map[string]bool{}}
	for _, a := range taken {
		k.taken[a] = true
	}
	return k
}

func (k *fakeKeys) Register(a string, _ func()) error {
	if k.taken[a] || k.registered[a] {
		return errors.New("already registered")
	}
	k.registered[a] = true
	return nil
}

func (k *fakeKeys) Unregister(a string) error {
	if !k.registered[a] {
		return errors.New("not registered")
	}
	delete(k.registered, a)
	return nil
}

func newPanel(keys *fakeKeys, hotkey string) *Panel {
	keys.registered[hotkey] = true
	return &Panel{keys: keys, hotkey: hotkey}
}

func TestRebindRegistersTheNewHotkeyThenReleasesTheOldOne(t *testing.T) {
	keys := newFakeKeys()
	p := newPanel(keys, "Alt+Shift+Space")
	refreshed := 0
	p.OnHotkeyError = func() { refreshed++ }
	if err := p.Rebind("Ctrl+Alt+K"); err != nil {
		t.Fatal(err)
	}
	if !keys.registered["Ctrl+Alt+K"] || keys.registered["Alt+Shift+Space"] || len(keys.registered) != 1 {
		t.Fatalf("registered = %v, want only the new one", keys.registered)
	}
	if refreshed != 1 || p.HotkeyError() != "" {
		t.Fatalf("refreshed %d times, error %q", refreshed, p.HotkeyError())
	}
}

func TestAHotkeyThatCannotBeUsedLeavesTheWorkingOneInPlace(t *testing.T) {
	keys := newFakeKeys("Ctrl+Alt+Delete")
	p := newPanel(keys, "Alt+Shift+Space")
	err := p.Rebind("Ctrl+Alt+Delete")
	if err == nil || !strings.Contains(err.Error(), "Ctrl+Alt+Delete cannot be used") {
		t.Fatalf("Rebind error = %v", err)
	}
	if !keys.registered["Alt+Shift+Space"] || p.hotkey != "Alt+Shift+Space" {
		t.Fatalf("the working hotkey was lost: registered %v, hotkey %q", keys.registered, p.hotkey)
	}
}

func TestRebindToTheSameHotkeyDoesNothing(t *testing.T) {
	keys := newFakeKeys()
	p := newPanel(keys, "Alt+Shift+Space")
	if err := p.Rebind("Alt+Shift+Space"); err != nil {
		t.Fatal(err)
	}
	if !keys.registered["Alt+Shift+Space"] {
		t.Fatal("the hotkey was released")
	}
}

func TestATakenHotkeyCanBeTriedAgainOrReplaced(t *testing.T) {
	keys := newFakeKeys()
	p := &Panel{keys: keys, hotkey: "Alt+Shift+Space", hotkeyErrMsg: "the hotkey Alt+Shift+Space cannot be used: taken"}
	if err := p.Rebind("Alt+Shift+Space"); err != nil { // the other program let go of it
		t.Fatal(err)
	}
	if p.HotkeyError() != "" || !keys.registered["Alt+Shift+Space"] {
		t.Fatalf("retry did not bind: error %q, registered %v", p.HotkeyError(), keys.registered)
	}
}
