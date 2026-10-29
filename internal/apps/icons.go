package apps

import (
	"context"
	"crypto/sha1"
	"encoding/hex"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
)

// iconSize is the edge of the icons in pixels: the row shows 44 DIP, up to 1.5x on a dense screen.
const iconSize = 64

// Icons returns middleware for the asset server that answers GET /icon?id=... with the PNG icon of the app
// with that id, and passes every other request on. Only ids in the index are served, so the page cannot use
// it to ask the shell about anything else. Icons are cached in cacheDir.
func Icons(index *Index, cacheDir string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path != "/icon" {
				next.ServeHTTP(w, r)
				return
			}
			serveIcon(w, r, index, cacheDir)
		})
	}
}

// extract makes the PNG icon of an app. Each system provides extractIcon (icon_windows.go, icon_darwin.go); tests replace this.
var extract = extractIcon

// warmIcons makes and caches the icon of every app that has none yet, so that no row waits for the shell
// (about 120 ms an icon, 22 s for the whole list) while the user types. It stops when ctx ends.
func warmIcons(ctx context.Context, apps []App, cacheDir string) {
	if err := os.MkdirAll(cacheDir, 0o755); err != nil {
		return
	}
	for _, app := range apps {
		if ctx.Err() != nil {
			return
		}
		file := filepath.Join(cacheDir, cacheName(app.ID))
		if _, err := os.Stat(file); err == nil {
			continue
		}
		if data, err := extract(app.ID); err == nil {
			_ = os.WriteFile(file, data, 0o644)
		}
	}
}

func serveIcon(w http.ResponseWriter, r *http.Request, index *Index, cacheDir string) {
	id := r.URL.Query().Get("id")
	if _, ok := index.Find(id); !ok {
		http.Error(w, "not an installed app", http.StatusNotFound)
		return
	}
	file := filepath.Join(cacheDir, cacheName(id))
	data, err := os.ReadFile(file)
	if err != nil {
		if data, err = extract(id); err != nil {
			http.Error(w, err.Error(), http.StatusNotFound)
			return
		}
		if err := os.MkdirAll(cacheDir, 0o755); err == nil {
			_ = os.WriteFile(file, data, 0o644)
		}
	}
	w.Header().Set("Content-Type", "image/png")
	w.Header().Set("Cache-Control", "private, max-age=3600")
	_, _ = w.Write(data)
}

// cacheName is the icon's file name: the id hashed (ids hold characters a file name cannot) with the size,
// so a change of size never serves old pixels.
func cacheName(id string) string {
	sum := sha1.Sum([]byte(id))
	return fmt.Sprintf("%s-%d.png", hex.EncodeToString(sum[:8]), iconSize)
}
