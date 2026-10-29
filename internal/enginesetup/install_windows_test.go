package enginesetup

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestRemovalWaitsForFilesThatAreStillHeldForAMoment(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "app")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	held, err := os.Create(filepath.Join(dir, "in-use.py")) // Windows refuses to delete a file that is open
	if err != nil {
		t.Fatal(err)
	}
	go func() {
		time.Sleep(700 * time.Millisecond) // the killed process lets go
		_ = held.Close()
	}()
	start := time.Now()
	if err := removeAll(dir); err != nil {
		t.Fatalf("removeAll gave up on a file that was released after %v: %v", time.Since(start), err)
	}
	if _, err := os.Stat(dir); !os.IsNotExist(err) {
		t.Fatal("the folder is still there")
	}
	if time.Since(start) < 500*time.Millisecond {
		t.Fatal("the file was deleted while it was still open, so the test did not test the wait")
	}
}
