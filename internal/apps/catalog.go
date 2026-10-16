package apps

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"time"
)

// refreshEvery is how often the list is rebuilt when nothing asks sooner: a Store app can be installed
// without touching any folder the launcher watches.
const refreshEvery = 24 * time.Hour

// Catalog keeps an Index true to what is installed: it loads the last list from disk, then rebuilds it
// from Windows in the background.
type Catalog struct {
	// Index is what searches read.
	Index *Index

	cache string
	list  func(ctx context.Context) ([]byte, error) // Get-StartApps as JSON; tests replace it
	now   func() time.Time
	poke  chan struct{}
}

// NewCatalog returns a catalog that caches its list in the file cache.
func NewCatalog(cache string) *Catalog {
	return &Catalog{Index: &Index{}, cache: cache, list: startApps, now: time.Now, poke: make(chan struct{}, 1)}
}

// Refresh asks for the list to be rebuilt soon. Asking twice before it runs is the same as once.
func (c *Catalog) Refresh() {
	select {
	case c.poke <- struct{}{}:
	default:
	}
}

// Run serves the cached list at once, rebuilds it if there is none or it is old, and rebuilds it again
// on Refresh and every day, until ctx ends.
func (c *Catalog) Run(ctx context.Context) {
	apps, saved, err := loadCache(c.cache)
	if err != nil {
		log.Printf("apps: ignoring the cached list: %v", err)
	}
	c.Index.Set(apps)
	if len(apps) == 0 || c.now().Sub(saved) > refreshEvery {
		c.rebuild(ctx)
	}
	for {
		select {
		case <-ctx.Done():
			return
		case <-c.poke:
		case <-time.After(refreshEvery):
		}
		c.rebuild(ctx)
	}
}

// rebuild reads the list from Windows. A failure keeps the list there was.
func (c *Catalog) rebuild(ctx context.Context) {
	start := time.Now()
	data, err := c.list(ctx)
	if err != nil {
		log.Printf("apps: could not list the installed apps: %v", err)
		return
	}
	apps, err := parse(data)
	if err != nil || len(apps) == 0 {
		log.Printf("apps: the list from Windows was unusable (%d apps, error %v); keeping the old one", len(apps), err)
		return
	}
	c.Index.Set(apps)
	if err := saveCache(c.cache, apps, c.now()); err != nil {
		log.Printf("apps: could not cache the list: %v", err)
	}
	log.Printf("apps: %d installed apps in %v", len(apps), time.Since(start).Round(time.Millisecond))
}

// cacheFile is the cache's content.
type cacheFile struct {
	Saved int64 `json:"saved"` // unix seconds
	Apps  []App `json:"apps"`
}

// loadCache reads the cached list and when it was saved. A missing file is no list and no error; a file
// that cannot be read is no list and an error (the caller rebuilds either way).
func loadCache(path string) ([]App, time.Time, error) {
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil, time.Time{}, nil
	}
	if err != nil {
		return nil, time.Time{}, err
	}
	var file cacheFile
	if err := json.Unmarshal(data, &file); err != nil {
		return nil, time.Time{}, fmt.Errorf("parse %s: %w", filepath.Base(path), err)
	}
	return file.Apps, time.Unix(file.Saved, 0), nil
}

// saveCache writes the list to a temporary file and moves it into place, so a crash never leaves half a file.
func saveCache(path string, apps []App, now time.Time) error {
	data, err := json.Marshal(cacheFile{Saved: now.Unix(), Apps: apps})
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	temp := path + ".tmp"
	if err := os.WriteFile(temp, data, 0o644); err != nil {
		return err
	}
	return os.Rename(temp, path)
}
