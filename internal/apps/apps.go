// Package apps knows which applications are installed, finds them as the user types, and starts them.
//
// The list comes from Windows itself: everything the Start Menu shows (desktop programs, Store apps
// and games), as `Get-StartApps` reports it. It is kept in memory, cached on disk so it is there before
// the first keystroke, and refreshed in the background, so a keystroke never waits for Windows.
package apps

import (
	"sort"
	"strings"
	"sync"
	"time"

	"omni/internal/rank"
)

// App is one entry of the Start Menu.
type App struct {
	// ID is what Windows calls the entry (its AppUserModelID, or the path of its executable); it is
	// stable across refreshes, so history and tags can hang on it.
	ID   string `json:"id"`
	Name string `json:"name"`
}

// History says how the user has used things; *state.Store satisfies it.
type History interface {
	// Uses are the times the thing with this id was launched, most recent last.
	Uses(id string) []time.Time
	// Learned is how many times the thing was chosen after typing exactly this.
	Learned(typed, id string) int
}

// Match is an app that fits what was typed.
type Match struct {
	App
	// Score orders matches; higher is better.
	Score float64
	// Strong is true when the name starts with, or has words or initials starting like, what was typed.
	Strong bool
}

// Index is the list of installed apps, safe for concurrent use.
type Index struct {
	mu   sync.RWMutex
	apps []App
}

// Set replaces the list.
func (i *Index) Set(apps []App) {
	i.mu.Lock()
	i.apps = apps
	i.mu.Unlock()
}

// All returns a copy of the list.
func (i *Index) All() []App {
	i.mu.RLock()
	defer i.mu.RUnlock()
	return append([]App(nil), i.apps...)
}

// Len is how many apps there are.
func (i *Index) Len() int {
	i.mu.RLock()
	defer i.mu.RUnlock()
	return len(i.apps)
}

// Find returns the app with this id.
func (i *Index) Find(id string) (App, bool) {
	i.mu.RLock()
	defer i.mu.RUnlock()
	for _, app := range i.apps {
		if app.ID == id {
			return app, true
		}
	}
	return App{}, false
}

// Remove drops an app that turned out not to exist any more.
func (i *Index) Remove(id string) {
	i.mu.Lock()
	defer i.mu.Unlock()
	kept := i.apps[:0:0]
	for _, app := range i.apps {
		if app.ID != id {
			kept = append(kept, app)
		}
	}
	i.apps = kept
}

// Query returns up to limit apps that fit typed, best first. An app's score is how well its name
// matches, lifted by how recently and how often it was launched and by how often it was chosen after
// typing exactly this. Strong matches always come before weak ones.
func (i *Index) Query(typed string, limit int, history History, now time.Time) []Match {
	typed = strings.ToLower(strings.TrimSpace(typed))
	if typed == "" {
		return nil
	}
	i.mu.RLock()
	var found []Match
	for _, app := range i.apps {
		r := rank.Match(typed, app.Name)
		if r.Score == 0 {
			continue
		}
		boost := rank.Frecency(history.Uses(app.ID), now) + float64(history.Learned(typed, app.ID))
		found = append(found, Match{App: app, Score: rank.Combine(r.Score, boost), Strong: r.Strong})
	}
	i.mu.RUnlock()
	sort.SliceStable(found, func(a, b int) bool {
		if found[a].Strong != found[b].Strong {
			return found[a].Strong
		}
		if found[a].Score != found[b].Score {
			return found[a].Score > found[b].Score
		}
		return found[a].Name < found[b].Name
	})
	if len(found) > limit {
		found = found[:limit]
	}
	return found
}
