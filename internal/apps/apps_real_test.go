package apps

import (
	"context"
	"encoding/json"
	"os"
	"sort"
	"testing"
	"time"
)

// TestRealStartMenu measures the index on this machine's real Start Menu. It is skipped unless
// OMNI_REAL_APPS=1, because it runs PowerShell and its numbers belong to the machine it runs on.
func TestRealStartMenu(t *testing.T) {
	if os.Getenv("OMNI_REAL_APPS") != "1" {
		t.Skip("set OMNI_REAL_APPS=1 to measure the real Start Menu")
	}
	start := time.Now()
	data, err := startApps(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("Get-StartApps: %v", time.Since(start).Round(time.Millisecond))

	var all []startApp
	if err := json.Unmarshal(data, &all); err != nil {
		t.Fatal(err)
	}
	kept, err := parse(data)
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("%d entries from Windows, %d kept as applications", len(all), len(kept))
	keptIDs := map[string]bool{}
	for _, a := range kept {
		keptIDs[a.ID] = true
	}
	for _, e := range all {
		if !keptIDs[e.AppID] {
			t.Logf("dropped: %-45s %s", e.Name, e.AppID)
		}
	}

	index := &Index{}
	index.Set(kept)
	// Every prefix of every kept name, as a user might type it: the worst case for one keystroke.
	var timings []time.Duration
	for _, app := range kept {
		runes := []rune(app.Name)
		for n := 1; n <= len(runes); n++ {
			begin := time.Now()
			index.Query(string(runes[:n]), 5, history{}, now)
			timings = append(timings, time.Since(begin))
		}
	}
	sort.Slice(timings, func(a, b int) bool { return timings[a] < timings[b] })
	t.Logf("query over %d apps: %d keystrokes, p50 %v, p99 %v, max %v", len(kept), len(timings),
		timings[len(timings)/2], timings[len(timings)*99/100], timings[len(timings)-1])

	for _, typed := range []string{"vsc", "code", "calc", "chr", "zen", "exc", "ppt", "word", "set", "term", "disc", "note"} {
		var top []string
		for _, m := range index.Query(typed, 3, history{}, now) {
			mark := ""
			if !m.Strong {
				mark = "?"
			}
			top = append(top, m.Name+mark)
		}
		t.Logf("%-5s -> %v", typed, top)
	}
}

// loadReal returns the kept apps of this machine's real Start Menu.
func loadReal(t *testing.T) []App {
	t.Helper()
	data, err := startApps(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	kept, err := parse(data)
	if err != nil {
		t.Fatal(err)
	}
	return withoutDead(kept)
}
