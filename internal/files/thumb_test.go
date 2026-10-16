package files

import (
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"testing"
)

func writePNG(t *testing.T, path string, width, height int) {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, width, height))
	for x := range width {
		for y := range height {
			img.Set(x, y, color.RGBA{uint8(x), uint8(y), 200, 255})
		}
	}
	file, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	if err := png.Encode(file, img); err != nil {
		t.Fatal(err)
	}
}

func get(handler http.Handler, path string) *httptest.ResponseRecorder {
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/thumb?path="+url.QueryEscape(path), nil))
	return rec
}

func TestThumbnails(t *testing.T) {
	root, cache := t.TempDir(), filepath.Join(t.TempDir(), "thumbs")
	outside := t.TempDir()
	wide := filepath.Join(root, "wide.png")
	writePNG(t, wide, 400, 200)
	small := filepath.Join(root, "small.png")
	writePNG(t, small, 40, 30)
	notAnImage := filepath.Join(root, "notes.png")
	if err := os.WriteFile(notAnImage, []byte("this is text"), 0o644); err != nil {
		t.Fatal(err)
	}
	secret := filepath.Join(outside, "secret.png")
	writePNG(t, secret, 10, 10)

	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusTeapot) })
	handler := Thumbnails([]string{root}, cache)(next)

	tests := []struct {
		name          string
		path          string
		status        int
		width, height int
	}{
		{"scales the long side down to thumbSize", wide, http.StatusOK, 160, 80},
		{"keeps a small image as it is", small, http.StatusOK, 40, 30},
		{"refuses a file outside the roots", secret, http.StatusForbidden, 0, 0},
		{"refuses a relative path", "wide.png", http.StatusForbidden, 0, 0},
		{"refuses a path that climbs out of the root", filepath.Join(root, "..", filepath.Base(outside), "secret.png"), http.StatusForbidden, 0, 0},
		{"answers 404 for something that is not an image", notAnImage, http.StatusNotFound, 0, 0},
		{"answers 404 for a missing file", filepath.Join(root, "gone.png"), http.StatusNotFound, 0, 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rec := get(handler, tt.path)
			if rec.Code != tt.status {
				t.Fatalf("status %d, want %d (%s)", rec.Code, tt.status, rec.Body.String())
			}
			if tt.status != http.StatusOK {
				return
			}
			img, err := jpeg.Decode(rec.Body)
			if err != nil {
				t.Fatalf("the answer is not a JPEG: %v", err)
			}
			if b := img.Bounds(); b.Dx() != tt.width || b.Dy() != tt.height {
				t.Errorf("thumbnail is %dx%d, want %dx%d", b.Dx(), b.Dy(), tt.width, tt.height)
			}
		})
	}

	t.Run("the second request comes from the cache", func(t *testing.T) {
		entries, err := os.ReadDir(cache)
		if err != nil {
			t.Fatal(err)
		}
		before := len(entries)
		get(handler, wide)
		entries, _ = os.ReadDir(cache)
		if before != 2 || len(entries) != before {
			t.Errorf("cache holds %d files before and %d after a repeat request, want 2 and 2", before, len(entries))
		}
	})

	t.Run("other requests pass through", func(t *testing.T) {
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/index.html", nil))
		if rec.Code != http.StatusTeapot {
			t.Errorf("status %d, want the next handler's", rec.Code)
		}
	})
}
