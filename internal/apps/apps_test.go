package apps

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func names(apps []App) string {
	var out []string
	for _, a := range apps {
		out = append(out, a.Name)
	}
	return strings.Join(out, "|")
}

// history is a fake History.
type history struct {
	uses    map[string][]time.Time
	learned map[string]int // typed + "\x00" + id
}

func (h history) Uses(id string) []time.Time   { return h.uses[id] }
func (h history) Learned(typed, id string) int { return h.learned[typed+"\x00"+id] }

var now = time.Date(2026, 10, 17, 12, 0, 0, 0, time.UTC)

func indexOf(names ...string) *Index {
	var apps []App
	for _, n := range names {
		apps = append(apps, App{ID: "id:" + n, Name: n})
	}
	i := &Index{}
	i.Set(apps)
	return i
}

func TestQueryRanksByMatchThenByUse(t *testing.T) {
	i := indexOf("Chrome", "Character Map", "Google Chrome", "Calculator")
	got := i.Query("chr", 5, history{}, now)
	if len(got) < 2 || got[0].Name != "Chrome" || !got[0].Strong {
		t.Fatalf("chr gave %+v, want Chrome first and strong", got)
	}
	if got[len(got)-1].Name != "Character Map" || got[len(got)-1].Strong {
		t.Fatalf("chr gave %+v, want the weak Character Map last", got)
	}
	// Using Google Chrome a lot lifts it above Chrome, both being strong matches.
	used := history{uses: map[string][]time.Time{"id:Google Chrome": {now.Add(-time.Hour), now.Add(-2 * time.Hour), now.Add(-3 * time.Hour)}}}
	if got := i.Query("chr", 5, used, now); got[0].Name != "Google Chrome" {
		t.Fatalf("a favourite should win, got %s first", got[0].Name)
	}
}

func TestQueryLearnsWhatYouChooseForATypedPrefix(t *testing.T) {
	i := indexOf("Notes", "Notepad", "Notion")
	if got := i.Query("no", 5, history{}, now); got[0].Name != "Notes" {
		t.Fatalf("by default the shortest wins, got %s", got[0].Name)
	}
	learned := history{learned: map[string]int{"no\x00id:Notion": 3}}
	if got := i.Query("no", 5, learned, now); got[0].Name != "Notion" {
		t.Fatalf("after choosing Notion three times for \"no\" it should come first, got %s", got[0].Name)
	}
	if got := i.Query("not", 5, learned, now); got[0].Name == "Notion" && got[0].Score > got[1].Score*2 {
		t.Fatalf("what was learned for \"no\" must not carry over to \"not\"")
	}
}

func TestQueryLimitsAndIgnoresEmptyText(t *testing.T) {
	i := indexOf("Alpha", "Alps", "Alto", "Alder")
	if got := i.Query("al", 2, history{}, now); len(got) != 2 {
		t.Fatalf("limit not applied: %d results", len(got))
	}
	if got := i.Query("  ", 5, history{}, now); got != nil {
		t.Fatalf("empty text must give nothing, got %v", got)
	}
}

func TestRemoveDropsAnAppThatIsGone(t *testing.T) {
	i := indexOf("Alpha", "Beta")
	i.Remove("id:Alpha")
	if _, ok := i.Find("id:Alpha"); ok || i.Len() != 1 {
		t.Fatalf("Alpha should be gone, %d left", i.Len())
	}
}

func TestCacheRoundTripAndMissingAndCorrupt(t *testing.T) {
	path := filepath.Join(t.TempDir(), "sub", "apps.json")
	if apps, _, err := loadCache(path); apps != nil || err != nil {
		t.Fatalf("a missing cache is no list and no error, got %v, %v", apps, err)
	}
	want := []App{{ID: "a", Name: "Alpha"}}
	if err := saveCache(path, want, now); err != nil {
		t.Fatal(err)
	}
	got, saved, err := loadCache(path)
	if err != nil || len(got) != 1 || got[0] != want[0] || !saved.Equal(now) {
		t.Fatalf("round trip gave %v, %v, %v", got, saved, err)
	}
	if err := os.WriteFile(path, []byte("{broken"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, _, err := loadCache(path); err == nil {
		t.Fatal("a corrupt cache must be an error")
	}
}

func newCatalog(t *testing.T, list func(context.Context) ([]App, error)) *Catalog {
	c := NewCatalog(filepath.Join(t.TempDir(), "apps.json"), "")
	c.list = list
	c.alive = func(apps []App) []App { return apps } // whether 7-Zip is installed is not what these tests are about
	c.now = func() time.Time { return now }
	return c
}

func TestRebuildReplacesTheListAndCachesIt(t *testing.T) {
	c := newCatalog(t, func(context.Context) ([]App, error) { return []App{{ID: "a", Name: "Alpha"}, {ID: "b", Name: "Beta"}}, nil })
	c.rebuild(context.Background())
	if c.Index.Len() != 2 {
		t.Fatalf("index holds %d apps, want 2", c.Index.Len())
	}
	if cached, _, _ := loadCache(c.cache); len(cached) != 2 {
		t.Fatalf("cache holds %d apps, want 2", len(cached))
	}
}

func TestAFailedRebuildKeepsTheListThereWas(t *testing.T) {
	c := newCatalog(t, func(context.Context) ([]App, error) { return nil, errors.New("the system would not list them") })
	c.Index.Set([]App{{ID: "a", Name: "Alpha"}})
	c.rebuild(context.Background())
	if c.Index.Len() != 1 {
		t.Fatal("a failed rebuild must keep the old list")
	}
	c.list = func(context.Context) ([]App, error) { return nil, nil }
	c.rebuild(context.Background())
	if c.Index.Len() != 1 {
		t.Fatal("an empty list from the system must not wipe the old one")
	}
}

func TestRunServesTheCacheAtOnceAndRebuildsOnlyWhenStale(t *testing.T) {
	rebuilt := make(chan struct{}, 4)
	c := newCatalog(t, func(context.Context) ([]App, error) {
		rebuilt <- struct{}{}
		return []App{{ID: "fresh", Name: "Fresh"}}, nil
	})
	if err := saveCache(c.cache, []App{{ID: "old", Name: "Old"}}, now.Add(-time.Hour)); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() { c.Run(ctx); close(done) }()
	time.Sleep(100 * time.Millisecond)
	if _, ok := c.Index.Find("old"); !ok {
		t.Fatal("the cached list must be served")
	}
	select {
	case <-rebuilt:
		t.Fatal("a fresh cache must not be rebuilt at start")
	default:
	}
	c.Refresh()
	select {
	case <-rebuilt:
	case <-time.After(2 * time.Second):
		t.Fatal("Refresh did not rebuild")
	}
	cancel()
	<-done
	if _, ok := c.Index.Find("fresh"); !ok {
		t.Fatal("the rebuilt list must replace the cached one")
	}
}

func TestRunRebuildsAStaleOrMissingCacheAtStart(t *testing.T) {
	c := newCatalog(t, func(context.Context) ([]App, error) { return []App{{ID: "fresh", Name: "Fresh"}}, nil })
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() { c.Run(ctx); close(done) }()
	time.Sleep(150 * time.Millisecond)
	cancel()
	<-done
	if _, ok := c.Index.Find("fresh"); !ok {
		t.Fatal("with no cache the list must be built at start")
	}
}

func TestAnEmptyListIsNotAskedForAgainAtEveryOpening(t *testing.T) {
	c := newCatalog(t, func(context.Context) ([]App, error) { return nil, nil })
	c.rebuild(context.Background()) // finds nothing, and keeps the old list
	c.RefreshIfStale(time.Minute)
	select {
	case <-c.poke:
		t.Fatal("a list that was tried a moment ago was asked for again")
	default:
	}
}

func TestRefreshIfStaleOnlyAsksWhenTheListIsOlderThanTheAge(t *testing.T) {
	c := newCatalog(t, func(context.Context) ([]App, error) { return []App{{ID: "a", Name: "Alpha"}}, nil })
	c.rebuild(context.Background()) // built at `now`
	c.RefreshIfStale(time.Minute)
	select {
	case <-c.poke:
		t.Fatal("a list made just now was rebuilt")
	default:
	}
	c.now = func() time.Time { return now.Add(2 * time.Minute) }
	c.RefreshIfStale(time.Minute)
	select {
	case <-c.poke:
	default:
		t.Fatal("a list two minutes old was not rebuilt")
	}
}
