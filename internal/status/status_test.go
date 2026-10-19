package status

import (
	"testing"

	"agi/internal/config"
	"agi/internal/indexing"
	"agi/internal/sidecar"
)

func infoFor(cfg config.Config, engine sidecar.State, index indexing.State) Info {
	return New(
		func() config.Config { return cfg },
		func() string { return "" },
		func() sidecar.Status { return sidecar.Status{State: engine} },
		func() indexing.Status { return indexing.Status{State: index} },
	).Info()
}

func TestInfoReportsTheCurrentSettings(t *testing.T) {
	cfg := config.Default()
	cfg.Hotkey, cfg.Roots, cfg.Search.Enabled = "Ctrl+Alt+K", []string{`C:\Docs`}, true
	got := infoFor(cfg, sidecar.StateReady, indexing.UpToDate)
	if got.Hotkey != "Ctrl+Alt+K" || len(got.Roots) != 1 || !got.FileSearch || got.Engine != "ready" {
		t.Fatalf("got %+v", got)
	}
}

func TestAnEngineThatIsOffLeavesNothingWaitingInTheIndexStatus(t *testing.T) {
	got := infoFor(config.Default(), sidecar.StateOff, indexing.Waiting)
	if got.Engine != "off" || got.IndexText != "" || got.FileSearch {
		t.Fatalf("got %+v, want an off engine with no index text", got)
	}
}
