package atomicfile

import (
	"os"
	"path/filepath"
	"testing"
)

func TestWriteReplacesTheFileAndLeavesNothingBehind(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "a.toml")
	for _, text := range []string{"first", "second, longer than the first", ""} {
		if err := Write(path, []byte(text)); err != nil {
			t.Fatal(err)
		}
		if got, err := os.ReadFile(path); err != nil || string(got) != text {
			t.Fatalf("read %q, %v; want %q", got, err, text)
		}
	}
	if entries, _ := os.ReadDir(dir); len(entries) != 1 {
		t.Errorf("the folder holds %d files, want only the one written", len(entries))
	}
}

func TestWriteFailsCleanlyWhenTheFolderIsMissing(t *testing.T) {
	if err := Write(filepath.Join(t.TempDir(), "no", "such", "a.txt"), []byte("x")); err == nil {
		t.Fatal("writing into a missing folder was accepted")
	}
}
