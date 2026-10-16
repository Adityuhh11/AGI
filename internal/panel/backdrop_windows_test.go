package panel

import (
	"bytes"
	"testing"
	"time"

	"golang.org/x/image/bmp"
)

func TestSnapshotIsADecodableShrunkPicture(t *testing.T) {
	r := Rect{X: 0, Y: 0, W: 1080, H: 176}
	start := time.Now()
	data := snapshot(r)
	t.Logf("snapshot of %dx%d took %v (%d bytes)", r.W, r.H, time.Since(start), len(data))
	if data == nil {
		t.Skip("no screen to read (a session without a desktop)")
	}
	img, err := bmp.Decode(bytes.NewReader(data))
	if err != nil {
		t.Fatalf("the picture is not a valid BMP: %v", err)
	}
	if got := img.Bounds().Size(); got.X != r.W/backdropScale || got.Y != r.H/backdropScale {
		t.Errorf("picture is %v, want %dx%d", got, r.W/backdropScale, r.H/backdropScale)
	}
}
