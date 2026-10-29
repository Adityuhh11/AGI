package hotkey

import (
	"errors"
	"testing"
)

type fakeKeys struct {
	bound  map[string]func()
	taken  map[string]bool
	events []string
}

func (f *fakeKeys) Register(a string, fn func()) error {
	if f.taken[a] {
		return errors.New("already registered")
	}
	f.bound[a] = fn
	f.events = append(f.events, "register "+a)
	return nil
}

func (f *fakeKeys) Unregister(a string) error {
	delete(f.bound, a)
	f.events = append(f.events, "unregister "+a)
	return nil
}

func TestRebindRegistersTheNewKeyBeforeDroppingTheOldOne(t *testing.T) {
	keys := &fakeKeys{bound: map[string]func(){}, taken: map[string]bool{"Ctrl+Alt+Z": true}}
	slot := NewSlot(keys)
	ran := 0
	if err := slot.Rebind("Ctrl+Alt+A", func() { ran++ }); err != nil {
		t.Fatal(err)
	}
	keys.bound["Ctrl+Alt+A"]()
	if ran != 1 {
		t.Fatal("the hotkey did not run its function")
	}
	if err := slot.Rebind("Ctrl+Alt+B", func() {}); err != nil {
		t.Fatal(err)
	}
	want := []string{"register Ctrl+Alt+A", "register Ctrl+Alt+B", "unregister Ctrl+Alt+A"}
	for i, w := range want {
		if keys.events[i] != w {
			t.Fatalf("events = %v, want %v", keys.events, want)
		}
	}
	if err := slot.Rebind("Ctrl+Alt+Z", func() {}); err == nil {
		t.Fatal("a combination another program owns was accepted")
	}
	if _, ok := keys.bound["Ctrl+Alt+B"]; !ok {
		t.Fatal("a refused combination must leave the working one in place")
	}
}

func TestAnEmptyAcceleratorTurnsTheHotkeyOffAndTheSameOneIsLeftAlone(t *testing.T) {
	keys := &fakeKeys{bound: map[string]func(){}, taken: map[string]bool{}}
	slot := NewSlot(keys)
	_ = slot.Rebind("Ctrl+Alt+A", func() {})
	_ = slot.Rebind("Ctrl+Alt+A", func() {})
	if len(keys.events) != 1 {
		t.Fatalf("the same combination was registered again: %v", keys.events)
	}
	if err := slot.Rebind("", nil); err != nil || len(keys.bound) != 0 {
		t.Fatalf("turning it off: %v, still bound %v", err, keys.bound)
	}
	slot.SetError("x")
	if slot.Error() != "x" {
		t.Fatal("SetError did not keep the message")
	}
	_ = slot.Rebind("Ctrl+Alt+B", func() {})
	if slot.Error() != "" {
		t.Fatal("a working hotkey must clear the error")
	}
}
