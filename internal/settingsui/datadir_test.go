package settingsui

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"agi/internal/enginesetup"
)

func TestChoosingAFolderKeepsFileSearchInAnAGIFolderInsideItAndGoingBackIsPossible(t *testing.T) {
	h := newHarness(t)
	picked := t.TempDir()
	h.picked = picked
	if msg := h.svc.ChooseDataFolder(); msg != "" {
		t.Fatal(msg)
	}
	want := filepath.Join(picked, "AGI")
	if got := h.store.Get().Search.DataDir; got != want {
		t.Fatalf("data_dir = %q, want %q", got, want)
	}
	if h.place.layout.Root != filepath.Join(want, "engine") || h.place.layout.Memory != filepath.Join(want, "memory") {
		t.Fatalf("the engine was told %+v", h.place.layout)
	}
	if _, err := os.Stat(want); err != nil {
		t.Fatalf("the folder was not made: %v", err)
	}
	if left, _ := filepath.Glob(filepath.Join(want, ".agi-write-test-*")); len(left) != 0 {
		t.Fatalf("the write test left %v behind", left)
	}
	view := h.svc.Search().Data
	if !view.Chosen || view.Engine != h.place.layout.Root {
		t.Fatalf("view = %+v", view)
	}

	if msg := h.svc.UseDefaultDataFolder(); msg != "" || h.store.Get().Search.DataDir != "" || h.svc.Search().Data.Chosen {
		t.Fatalf("back to the default: %q, data_dir %q", msg, h.store.Get().Search.DataDir)
	}
}

func TestAFolderAlreadyCalledAGIIsUsedAsItIs(t *testing.T) {
	picked := filepath.Join(t.TempDir(), "AGI")
	if got := dataDirIn(picked); got != picked {
		t.Errorf("dataDirIn(%q) = %q", picked, got)
	}
	if got, want := dataDirIn(`D:\Files`), `D:\Files\AGI`; got != want {
		t.Errorf("dataDirIn = %q, want %q", got, want)
	}
}

func TestACancelledPickerARefusedMoveAndAFolderThatCannotBeWrittenChangeNothing(t *testing.T) {
	h := newHarness(t)
	h.picked = ""
	if msg := h.svc.ChooseDataFolder(); msg != "" || h.store.Get().Search.DataDir != "" {
		t.Fatalf("cancelling: %q, data_dir %q", msg, h.store.Get().Search.DataDir)
	}

	h.picked = t.TempDir()
	h.place.refuse = errors.New("file search is already installed in C:\\x")
	if msg := h.svc.ChooseDataFolder(); !strings.Contains(msg, "already installed") || h.store.Get().Search.DataDir != "" {
		t.Fatalf("a refused move: %q, data_dir %q", msg, h.store.Get().Search.DataDir)
	}
	h.place.refuse = nil

	file := filepath.Join(t.TempDir(), "a file")
	if err := os.WriteFile(file, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	h.picked = file // AGI would have to be made inside a file
	if msg := h.svc.ChooseDataFolder(); !strings.Contains(msg, "cannot keep its files") || h.store.Get().Search.DataDir != "" {
		t.Fatalf("an unusable folder: %q, data_dir %q", msg, h.store.Get().Search.DataDir)
	}
	if h.place.layout.Root != `C:\default\engine` {
		t.Fatalf("a refused folder still moved the engine: %+v", h.place.layout)
	}
}

func TestTheFolderCanBeChangedOnlyBeforeAnythingIsInstalled(t *testing.T) {
	h := newHarness(t)
	if !h.svc.Search().Data.CanChange {
		t.Fatal("a fresh install can choose")
	}
	h.setup.state = enginesetup.State{Installing: true}
	if h.svc.Search().Data.CanChange {
		t.Fatal("not while it downloads")
	}
	h.setup.state = enginesetup.State{Installed: true}
	if h.svc.Search().Data.CanChange {
		t.Fatal("not once it is installed")
	}
}
