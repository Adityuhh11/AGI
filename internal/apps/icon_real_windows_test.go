package apps

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

// TestRealIcons extracts the icon of every kept app on this machine; it runs only with OMNI_REAL_APPS=1.
// With OMNI_ICON_DUMP=<folder> it also saves the PNGs there, to look at.
func TestRealIcons(t *testing.T) {
	if os.Getenv("OMNI_REAL_APPS") != "1" {
		t.Skip("set OMNI_REAL_APPS=1 to extract the real icons")
	}
	index := loadReal(t)
	dump := os.Getenv("OMNI_ICON_DUMP")
	var failed []string
	var total, slowest time.Duration
	count := 0
	for _, app := range index {
		start := time.Now()
		data, err := extractIcon(app.ID)
		took := time.Since(start)
		if err != nil {
			failed = append(failed, app.Name+": "+err.Error())
			continue
		}
		count++
		total += took
		slowest = max(slowest, took)
		if dump != "" {
			_ = os.MkdirAll(dump, 0o755)
			_ = os.WriteFile(filepath.Join(dump, cacheName(app.ID)), data, 0o644)
		}
	}
	t.Logf("%d icons in %v on average (slowest %v); %d without one", count, (total / time.Duration(max(count, 1))).Round(time.Millisecond), slowest.Round(time.Millisecond), len(failed))
	for _, f := range failed {
		t.Logf("no icon: %s", f)
	}
}
