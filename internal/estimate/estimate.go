// Package estimate says how long indexing will take on this computer, before it starts, from a survey of what the
// folders hold (counted by kind, without opening a file) and the choices made. It is a plan, not a promise: it is
// right to within a third on the folders it was checked against (docs/eval/2026-10-24-estimate.md), and the time
// left that indexing reports once it runs corrects it.
package estimate

import (
	"math"
	"time"

	"omni/internal/sidecar"
)

// Rates measured on the development laptop (docs/eval/2026-10-20-indexing-choices.md): text is embedded in chunks,
// a picture, a PDF page or a video frame is one vision pass, and the processor is the fallback when no graphics card
// can run the model.
const (
	textChunksPerSecond      = 53.0 // on a graphics card
	textChunksPerSecondCPU   = 1.4
	secondsPerPicture        = 0.16
	secondsPerPictureCPU     = 4.9
	secondsPerFile           = 0.03 // reading, hashing, chunking and storing, whatever the file
	secondsPerVideoWithFrame = 0.4  // seeking to eight frames
	framesPerVideo           = 8
)

// Chunks and pictures per unit, from the index of 3,795 files the owner uses (docs/eval/2026-10-24-estimate.md). Text
// is chunked by size, so it scales with bytes; a PDF's pages scale with its size too, which holds in total even though
// single PDFs vary from one page to hundreds.
const (
	bytesPerCodeChunk     = 1086.0
	bytesPerMarkdownChunk = 829.0
	bytesPerTextChunk     = 1691.0
	pdfTextChunksPerMB    = 15.9
	pdfPicturesPerMBAll   = 17.7 // every page
	pdfPicturesPerMBAuto  = 8.8  // pages that have a picture, little text or many drawn shapes
	bytesPerMB            = 1 << 20
)

// paceFactors is how many times as long indexing takes at each speed (docs/eval/2026-10-18-pace.md).
var paceFactors = map[string]float64{"fast": 1, "balanced": 1.9, "gentle": 5.9}

// Work is what indexing a folder amounts to under some choices.
type Work struct {
	TextChunks float64
	Pictures   float64
	Files      int
	Videos     int // videos whose frames are read
}

// WorkFor counts the chunks and pictures that indexing a surveyed folder embeds under options.
func WorkFor(routes map[string]sidecar.RouteCount, o sidecar.IndexOptions) Work {
	o = withDefaults(o)
	var w Work
	for route, c := range routes {
		w.Files += c.Files
		switch route {
		case "code":
			w.TextChunks += bySize(c, bytesPerCodeChunk)
		case "markdown":
			w.TextChunks += bySize(c, bytesPerMarkdownChunk)
		case "text":
			w.TextChunks += bySize(c, bytesPerTextChunk)
		case "pdf":
			mb := float64(c.Bytes) / bytesPerMB
			w.TextChunks += math.Max(float64(c.Files), mb*pdfTextChunksPerMB)
			switch o.PDFPages {
			case "all":
				w.Pictures += math.Max(float64(c.Files), mb*pdfPicturesPerMBAll)
			case "auto":
				w.Pictures += mb * pdfPicturesPerMBAuto
			}
		case "image":
			if o.Photos == "content" {
				w.Pictures += float64(c.Files)
			} else {
				w.TextChunks += float64(c.Files) // its file name
			}
		case "video":
			w.TextChunks += float64(c.Files) // the name, in either choice
			if o.Videos == "content" {
				w.Pictures += float64(c.Files * framesPerVideo)
				w.Videos += c.Files
			}
		}
	}
	return w
}

// Duration is how long embedding the work takes, at a speed (fast, balanced or gentle), on the graphics card or, with
// onCPU, on the processor.
func (w Work) Duration(pace string, onCPU bool) time.Duration {
	textRate, pictureTime := textChunksPerSecond, secondsPerPicture
	if onCPU {
		textRate, pictureTime = textChunksPerSecondCPU, secondsPerPictureCPU
	}
	seconds := w.TextChunks/textRate + w.Pictures*pictureTime + float64(w.Files)*secondsPerFile +
		float64(w.Videos)*secondsPerVideoWithFrame
	factor, ok := paceFactors[pace]
	if !ok {
		factor = paceFactors["balanced"]
	}
	return time.Duration(seconds * factor * float64(time.Second))
}

// bySize is the chunks of a kind of file chunked by size: at least one a file.
func bySize(c sidecar.RouteCount, bytesPerChunk float64) float64 {
	return math.Max(float64(c.Files), float64(c.Bytes)/bytesPerChunk)
}

// withDefaults fills the choices a caller left empty with the engine's defaults.
func withDefaults(o sidecar.IndexOptions) sidecar.IndexOptions {
	if o.PDFPages == "" {
		o.PDFPages = sidecar.DefaultIndexOptions.PDFPages
	}
	if o.Photos == "" {
		o.Photos = sidecar.DefaultIndexOptions.Photos
	}
	if o.Videos == "" {
		o.Videos = sidecar.DefaultIndexOptions.Videos
	}
	return o
}
