package web

import (
	"os"
	"testing"
)

// TestRealAppPath looks Zen up in this machine's registry; it runs only with AGI_REAL_WEB=1.
func TestRealAppPath(t *testing.T) {
	if os.Getenv("AGI_REAL_WEB") != "1" {
		t.Skip("set AGI_REAL_WEB=1 to read the registry")
	}
	path, err := appPath("zen")
	if err != nil {
		t.Fatalf("zen is not registered: %v", err)
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("zen is registered at %s, which does not exist", path)
	}
	t.Logf("zen: %s", path)
	if _, err := appPath("definitely-not-a-browser"); err == nil {
		t.Fatal("an unknown name must not be found")
	}
}
