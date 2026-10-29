package main

import (
	"os"
	"path/filepath"
	"testing"
)

func fill(t *testing.T, paths ...string) {
	t.Helper()
	for _, p := range paths {
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
}

func exists(p string) bool { _, err := os.Stat(p); return err == nil }

func TestRemovingTheDataDeletesOmnisFoldersAndTheTwoInsideAChosenOneButNothingBeside(t *testing.T) {
	root := t.TempDir()
	appData, settings, chosen := filepath.Join(root, "Local", "Omni"), filepath.Join(root, "Roaming", "Omni"), filepath.Join(root, "D", "Omni")
	neighbour := filepath.Join(root, "Local", "OtherProgram", "keep.txt")
	otherInChosen := filepath.Join(chosen, "notes.txt")
	fill(t, filepath.Join(appData, "engine", "e.bin"), filepath.Join(appData, "memory", "meta.db"), filepath.Join(appData, "logs", "omni.log"),
		filepath.Join(settings, "config.toml"), filepath.Join(chosen, "engine", "e.bin"), filepath.Join(chosen, "memory", "meta.db"), filepath.Join(chosen, "chat", "m.gguf"), neighbour, otherInChosen)

	if err := removeData(appData, settings, chosen); err != nil {
		t.Fatal(err)
	}
	for _, gone := range []string{appData, settings, filepath.Join(chosen, "engine"), filepath.Join(chosen, "memory"), filepath.Join(chosen, "chat")} {
		if exists(gone) {
			t.Errorf("%s is still there", gone)
		}
	}
	for _, kept := range []string{neighbour, otherInChosen} {
		if !exists(kept) {
			t.Errorf("%s was deleted, and was not Omni's", kept)
		}
	}
}

func TestAFolderThatIsNotCalledOmniIsNeverDeleted(t *testing.T) {
	root := t.TempDir()
	notOmni := filepath.Join(root, "Documents")
	fill(t, filepath.Join(notOmni, "thesis.docx"), filepath.Join(notOmni, "engine", "a"))
	if err := removeData(notOmni, notOmni, notOmni); err == nil {
		t.Fatal("deleting a folder that is not Omni's must be refused")
	}
	if !exists(filepath.Join(notOmni, "thesis.docx")) || !exists(filepath.Join(notOmni, "engine", "a")) {
		t.Fatal("a folder that is not Omni's lost files")
	}
}

func TestRemovingWhatIsNotThereIsFine(t *testing.T) {
	root := t.TempDir()
	if err := removeData(filepath.Join(root, "Local", "Omni"), filepath.Join(root, "Roaming", "Omni"), ""); err != nil {
		t.Fatal(err)
	}
}
