package panel

import "math"

// Rect is a rectangle in physical pixels.
type Rect struct{ X, Y, W, H int }

const (
	// widthDIP is the panel width in device-independent pixels (96 per inch).
	widthDIP = 720
	// topFraction puts the panel's top edge this far down the usable height of the screen.
	topFraction = 0.18
)

// place returns where a panel of widthDIP x heightDIP goes on the monitor whose usable area is
// work (taskbar excluded) at the given dpi: centred horizontally, near the top, and never
// larger than the usable area or hanging off its bottom.
func place(work Rect, dpi uint32, heightDIP int) Rect {
	scale := 1.0
	if dpi > 0 {
		scale = float64(dpi) / 96
	}
	w := min(int(math.Round(widthDIP*scale)), work.W)
	h := min(int(math.Round(float64(heightDIP)*scale)), work.H)
	x := work.X + (work.W-w)/2
	y := work.Y + int(math.Round(float64(work.H)*topFraction))
	y = min(y, work.Y+work.H-h)
	return Rect{X: x, Y: y, W: w, H: h}
}

// placeAt puts a panel of the size of base where the person last left its top-left corner (x, y), pulled back
// in if it would hang off the usable area. ok is false when that corner is not on work at all (another monitor, or one
// that was unplugged), and base is returned as it came so the panel opens in its usual place.
func placeAt(base, work Rect, x, y int) (Rect, bool) {
	if x < work.X || y < work.Y || x >= work.X+work.W || y >= work.Y+work.H {
		return base, false
	}
	base.X = min(x, work.X+work.W-base.W)
	base.Y = min(y, work.Y+work.H-base.H)
	return base, true
}
