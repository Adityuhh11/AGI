package estimate

import (
	"math"
	"testing"
	"time"

	"agi/internal/sidecar"
)

// sampleData is what a survey of the repository's Sample Data folder (96 files) reports.
var sampleData = map[string]sidecar.RouteCount{
	"code":     {Files: 60, Bytes: 235595},
	"markdown": {Files: 3, Bytes: 9120},
	"text":     {Files: 5, Bytes: 49738},
	"image":    {Files: 26, Bytes: 13171209},
	"pdf":      {Files: 2, Bytes: 1281078},
}

// The times indexing that folder really took, one machine, no pacing (docs/eval/2026-10-20-indexing-choices.md for the
// graphics card, docs/eval/2026-10-24-estimate.md for the processor). The estimate is a plan: within about a third.
func TestTheEstimateIsWithinAThirdOfWhatSampleDataTook(t *testing.T) {
	tests := []struct {
		name     string
		options  sidecar.IndexOptions
		onCPU    bool
		measured time.Duration
	}{
		{"defaults on the graphics card", sidecar.IndexOptions{}, false, 16500 * time.Millisecond},
		{"cheapest on the graphics card", sidecar.IndexOptions{PDFPages: "text", Photos: "name"}, false, 8000 * time.Millisecond},
		{"defaults on the processor", sidecar.IndexOptions{}, true, 466 * time.Second},
		{"cheapest on the processor", sidecar.IndexOptions{PDFPages: "text", Photos: "name"}, true, 292 * time.Second},
	}
	for _, tt := range tests {
		got := WorkFor(sampleData, tt.options).Duration("fast", tt.onCPU)
		if off := float64(got-tt.measured) / float64(tt.measured); math.Abs(off) > 0.33 {
			t.Errorf("%s: estimated %v, took %v (%+.0f%%)", tt.name, got.Round(time.Second), tt.measured, off*100)
		}
	}
}

func TestWhatEachChoiceAddsToTheWork(t *testing.T) {
	routes := map[string]sidecar.RouteCount{
		"pdf":   {Files: 4, Bytes: 10 << 20},
		"image": {Files: 100, Bytes: 1 << 30},
		"video": {Files: 3, Bytes: 1 << 30},
	}
	pictures := func(o sidecar.IndexOptions) float64 { return WorkFor(routes, o).Pictures }
	if all, auto, text := pictures(sidecar.IndexOptions{PDFPages: "all", Photos: "name"}), pictures(sidecar.IndexOptions{PDFPages: "auto", Photos: "name"}),
		pictures(sidecar.IndexOptions{PDFPages: "text", Photos: "name"}); !(all > auto && auto > text && text == 0) {
		t.Errorf("PDF pictures: all %.0f, auto %.0f, words only %.0f", all, auto, text)
	}
	if got, want := pictures(sidecar.IndexOptions{PDFPages: "text", Photos: "content"}), 100.0; got != want {
		t.Errorf("photos by what they show are %.0f pictures, want %.0f", got, want)
	}
	if got, want := pictures(sidecar.IndexOptions{PDFPages: "text", Photos: "name", Videos: "content"}), 24.0; got != want {
		t.Errorf("eight frames of 3 videos are %.0f pictures, want %.0f", got, want)
	}
	// A name is one text chunk: photos by name and videos by name cost text, not pictures.
	byName := WorkFor(routes, sidecar.IndexOptions{PDFPages: "text", Photos: "name", Videos: "name"})
	if byName.Pictures != 0 || byName.TextChunks < 103 {
		t.Errorf("by name: %+v", byName)
	}
}

func TestPaceStretchesTheTimeAndAnUnknownPaceIsBalanced(t *testing.T) {
	w := Work{TextChunks: 530}
	fast, balanced, gentle := w.Duration("fast", false), w.Duration("balanced", false), w.Duration("gentle", false)
	if fast != 10*time.Second || balanced != 19*time.Second || gentle != 59*time.Second {
		t.Errorf("fast %v, balanced %v, gentle %v", fast, balanced, gentle)
	}
	if w.Duration("", false) != balanced {
		t.Error("no pace should be balanced")
	}
}

func TestSmallFoldersHaveAtLeastOneChunkAFile(t *testing.T) {
	w := WorkFor(map[string]sidecar.RouteCount{"code": {Files: 10, Bytes: 100}}, sidecar.IndexOptions{})
	if w.TextChunks != 10 {
		t.Errorf("10 tiny files are %.0f chunks, want 10", w.TextChunks)
	}
}
