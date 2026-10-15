package files

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"image"
	"image/color"
	stddraw "image/draw"
	_ "image/gif" // registers the decoders image.Decode picks from
	"image/jpeg"
	_ "image/png"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strconv"

	_ "golang.org/x/image/bmp"
	"golang.org/x/image/draw"
	_ "golang.org/x/image/webp"
)

const (
	// thumbSize is the long side of a thumbnail in pixels: the row shows 64 DIP, up to 2.5x on a
	// dense screen.
	thumbSize = 160
	// maxPixels refuses images whose decoded size would run into hundreds of megabytes.
	maxPixels = 100_000_000
	// concurrentThumbs keeps ten rows of fresh images from taking every core while the user types.
	concurrentThumbs = 2
)

// Thumbnails returns middleware for the asset server that answers GET /thumb?path=... with a small
// JPEG of the image at path, and passes every other request on. Only images inside roots are
// served, so the page cannot use it to read other files. Thumbnails are cached in cacheDir, keyed by
// path, size and modification time.
func Thumbnails(roots []string, cacheDir string) func(http.Handler) http.Handler {
	t := &thumbs{roots: roots, dir: cacheDir, slots: make(chan struct{}, concurrentThumbs)}
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path != "/thumb" {
				next.ServeHTTP(w, r)
				return
			}
			t.serve(w, r)
		})
	}
}

type thumbs struct {
	roots []string
	dir   string
	slots chan struct{}
}

func (t *thumbs) serve(w http.ResponseWriter, r *http.Request) {
	path := r.URL.Query().Get("path")
	if !inside(t.roots, path) {
		http.Error(w, "not inside a folder the launcher searches", http.StatusForbidden)
		return
	}
	// Lstat: a link is never followed, as in the engine's scanner.
	info, err := os.Lstat(path)
	if err != nil || !info.Mode().IsRegular() {
		http.NotFound(w, r)
		return
	}
	sum := sha256.Sum256([]byte(path + "|" + strconv.FormatInt(info.Size(), 10) + "|" + strconv.FormatInt(info.ModTime().UnixNano(), 10)))
	cached := filepath.Join(t.dir, hex.EncodeToString(sum[:16])+".jpg")
	if _, err := os.Stat(cached); err != nil {
		t.slots <- struct{}{}
		err := t.build(path, cached)
		<-t.slots
		if err != nil {
			log.Printf("thumbnail of %s: %v", filepath.Base(path), err)
			http.NotFound(w, r) // the row falls back to its icon
			return
		}
	}
	http.ServeFile(w, r, cached)
}

// build decodes path, scales it to thumbSize and writes it to dest. The write goes through a
// temporary file so a half-written thumbnail is never served.
func (t *thumbs) build(path, dest string) error {
	file, err := os.Open(path)
	if err != nil {
		return err
	}
	defer file.Close()
	config, _, err := image.DecodeConfig(file)
	if err != nil {
		return fmt.Errorf("read the image header: %w", err)
	}
	if config.Width*config.Height > maxPixels {
		return fmt.Errorf("image is %dx%d, too large to thumbnail", config.Width, config.Height)
	}
	if _, err := file.Seek(0, 0); err != nil {
		return err
	}
	src, _, err := image.Decode(file)
	if err != nil {
		return fmt.Errorf("decode: %w", err)
	}

	// ponytail: EXIF orientation is ignored, so some phone photos show sideways; read the tag when it bothers.
	bounds := src.Bounds()
	width, height := bounds.Dx(), bounds.Dy()
	if long := max(width, height); long > thumbSize {
		width, height = max(1, width*thumbSize/long), max(1, height*thumbSize/long)
	}
	out := image.NewRGBA(image.Rect(0, 0, width, height))
	// Transparent pixels would turn black in a JPEG; a dark grey reads as the row's background.
	stddraw.Draw(out, out.Bounds(), image.NewUniform(color.RGBA{0x20, 0x20, 0x20, 0xff}), image.Point{}, stddraw.Src)
	draw.CatmullRom.Scale(out, out.Bounds(), src, bounds, draw.Over, nil)

	if err := os.MkdirAll(t.dir, 0o755); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(t.dir, "building-*")
	if err != nil {
		return err
	}
	if err := jpeg.Encode(tmp, out, &jpeg.Options{Quality: 80}); err != nil {
		tmp.Close()
		os.Remove(tmp.Name())
		return fmt.Errorf("encode: %w", err)
	}
	if err := tmp.Close(); err != nil {
		os.Remove(tmp.Name())
		return err
	}
	return os.Rename(tmp.Name(), dest)
}
