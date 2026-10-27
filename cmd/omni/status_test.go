package main

import (
	"testing"

	"omni/internal/indexing"
	"omni/internal/sidecar"
)

func TestTheTrayStatusLineNamesTheMostUrgentThing(t *testing.T) {
	ready := sidecar.Status{State: sidecar.StateReady}
	done := indexing.Status{State: indexing.UpToDate, Files: 2, Chunks: 9}
	tests := []struct {
		name   string
		hotkey string
		engine sidecar.Status
		held   bool
		want   string
	}{
		{"a taken hotkey beats everything", "Alt+Space is taken", ready, true, "Alt+Space is taken"},
		{"off", "", sidecar.Status{State: sidecar.StateOff}, false, "File search is off. Open Settings to turn it on."},
		{"held after a first download", "", ready, true, "File search is ready. Open Settings to choose what to index."},
		{"not held", "", ready, false, "Up to date: 9 chunks in 2 files"},
	}
	for _, tt := range tests {
		if got := statusLine(tt.hotkey, tt.engine, done, tt.held); got != tt.want {
			t.Errorf("%s: %q, want %q", tt.name, got, tt.want)
		}
	}
}
