package llm

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"omni/internal/enginesetup"
)

// TestRealChatServerRunsOnAMac unpacks the real llama.cpp build the chat model needs and starts it, which proves the pin (its size, its SHA-256, its
// links, its signature and libraries) on a Mac. It is for the Mac CI job: OMNI_REAL_INSTALL=1.
func TestRealChatServerRunsOnAMac(t *testing.T) {
	if os.Getenv("OMNI_REAL_INSTALL") != "1" {
		t.Skip("set OMNI_REAL_INSTALL=1 to download llama.cpp and run it")
	}
	layout := Layout{Root: filepath.Join(t.TempDir(), "chat")}
	if err := (enginesetup.Downloader{}).Unpack(t.Context(), Llama, layout.Dir(), func(int64, int64) {}); err != nil {
		t.Fatalf("unpack llama.cpp: %v", err)
	}
	out, err := exec.Command(layout.Server(), "--version").CombinedOutput()
	if err != nil {
		t.Fatalf("llama-server does not run: %v\n%s", err, out)
	}
	t.Logf("llama-server: %s", strings.TrimSpace(string(out)))
	if devices, err := exec.Command(layout.Server(), "--list-devices").CombinedOutput(); err == nil {
		t.Logf("devices:\n%s", devices)
	}
}
