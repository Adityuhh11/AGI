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

func TestRemovingTheDataDeletesAGIsFoldersAndTheTwoInsideAChosenOneButNothingBeside(t *testing.T) {
	root := t.TempDir()
	appData, settings, chosen := filepath.Join(root, "Local", "AGI"), filepath.Join(root, "Roaming", "AGI"), filepath.Join(root, "D", "AGI")
	neighbour := filepath.Join(root, "Local", "OtherProgram", "keep.txt")
	otherInChosen := filepath.Join(chosen, "notes.txt")
	fill(t, filepath.Join(appData, "engine", "e.bin"), filepath.Join(appData, "memory", "meta.db"), filepath.Join(appData, "logs", "agi.log"),
		filepath.Join(settings, "config.toml"), filepath.Join(chosen, "engine", "e.bin"), filepath.Join(chosen, "memory", "meta.db"), neighbour, otherInChosen)

	if err := removeData(appData, settings, chosen); err != nil {
		t.Fatal(err)
	}
	for _, gone := range []string{appData, settings, filepath.Join(chosen, "engine"), filepath.Join(chosen, "memory")} {
		if exists(gone) {
			t.Errorf("%s is still there", gone)
		}
	}
	for _, kept := range []string{neighbour, otherInChosen} {
		if !exists(kept) {
			t.Errorf("%s was deleted, and was not AGI's", kept)
		}
	}
}

func TestAFolderThatIsNotCalledAGIIsNeverDeleted(t *testing.T) {
	root := t.TempDir()
	notAGI := filepath.Join(root, "Documents")
	fill(t, filepath.Join(notAGI, "thesis.docx"), filepath.Join(notAGI, "engine", "a"))
	if err := removeData(notAGI, notAGI, notAGI); err == nil {
		t.Fatal("deleting a folder that is not AGI's must be refused")
	}
	if !exists(filepath.Join(notAGI, "thesis.docx")) || !exists(filepath.Join(notAGI, "engine", "a")) {
		t.Fatal("a folder that is not AGI's lost files")
	}
}

func TestRemovingWhatIsNotThereIsFine(t *testing.T) {
	root := t.TempDir()
	if err := removeData(filepath.Join(root, "Local", "AGI"), filepath.Join(root, "Roaming", "AGI"), ""); err != nil {
		t.Fatal(err)
	}
}
