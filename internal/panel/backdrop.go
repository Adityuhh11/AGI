package panel

import (
	"net/http"
	"strconv"
)

// backdropPicture takes the picture of the screen behind the bar at at, the area the bare bar will
// cover. Reading the screen costs time in proportion to the area (about 10 ms for the bar, 25 ms for a
// full card), so only the bar is read; the page stretches it over the card when results open, and the
// blur and tint over it hide that.
func backdropPicture(at Rect, dpi uint32) []byte {
	bar := int(float64(barHeightDIP) * float64(dpi) / 96)
	return snapshot(Rect{X: at.X, Y: at.Y, W: at.W, H: min(at.H, bar)})
}

// ServeBackdrop answers GET /backdrop with the picture, for the asset server's middleware.
func (p *Panel) ServeBackdrop(w http.ResponseWriter, _ *http.Request) {
	p.mu.Lock()
	picture := p.backdrop
	p.mu.Unlock()
	if picture == nil {
		http.Error(w, "no picture", http.StatusNotFound)
		return
	}
	w.Header().Set("Content-Type", "image/bmp")
	w.Header().Set("Content-Length", strconv.Itoa(len(picture)))
	w.Header().Set("Cache-Control", "no-store")
	_, _ = w.Write(picture)
}
