package settingsui

import (
	"strings"
	"testing"
)

func TestTheNotesHotkeyIsBoundBeforeItIsSavedAndARefusedOneChangesNothing(t *testing.T) {
	h := newHarness(t, "Ctrl+Alt+X")
	if msg := h.svc.SetNotesHotkey(" Ctrl+Alt+N "); msg != "" {
		t.Fatal(msg)
	}
	if got := h.store.Get().Notes.Hotkey; got != "Ctrl+Alt+N" || h.svc.NotesHotkey().Hotkey != "Ctrl+Alt+N" {
		t.Fatalf("saved %q", got)
	}
	if msg := h.svc.SetNotesHotkey("Ctrl+Alt+X"); !strings.Contains(msg, "Ctrl+Alt+X") {
		t.Fatalf("message %q: the combination that cannot be used must be named", msg)
	}
	if got := h.store.Get().Notes.Hotkey; got != "Ctrl+Alt+N" {
		t.Fatalf("a refused hotkey changed the saved one to %q", got)
	}
	if msg := h.svc.SetNotesHotkey(""); msg != "" || h.store.Get().Notes.Hotkey != "" {
		t.Fatalf("turning the hotkey off: %q, %q", msg, h.store.Get().Notes.Hotkey)
	}
}

func TestTheNotesHotkeyMayNotBeAnotherHotkeysAndIsNeverBoundThen(t *testing.T) {
	h := newHarness(t)
	if msg := h.svc.SetNotesHotkey(h.store.Get().Hotkey); msg == "" {
		t.Fatal("the launcher's own hotkey was accepted")
	}
	if len(h.noteHotkeys) != 0 {
		t.Fatalf("a refused combination was registered: %v", h.noteHotkeys)
	}
}

func TestAnUnchangedNotesHotkeyIsNotRebound(t *testing.T) {
	h := newHarness(t)
	h.svc.SetNotesHotkey("Ctrl+Alt+N")
	before := len(h.noteHotkeys)
	if msg := h.svc.SetNotesHotkey("Ctrl+Alt+N"); msg != "" || len(h.noteHotkeys) != before {
		t.Fatalf("message %q, %d rebinds after %d", msg, len(h.noteHotkeys), before)
	}
}
