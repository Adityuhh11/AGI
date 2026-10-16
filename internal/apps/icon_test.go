package apps

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestIconsMiddlewareRefusesIdsOutsideTheIndexAndPassesOtherRequestsOn(t *testing.T) {
	index := &Index{}
	index.Set([]App{{ID: "known", Name: "Known"}})
	passed := false
	handler := Icons(index, t.TempDir())(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { passed = true }))

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest("GET", "/icon?id=C:%5CWindows%5Csystem32%5Ccmd.exe", nil))
	if rec.Code != http.StatusNotFound || passed {
		t.Fatalf("an id that is not in the index gave %d (passed on: %v), want 404", rec.Code, passed)
	}
	handler.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest("GET", "/index.html", nil))
	if !passed {
		t.Fatal("other requests must reach the next handler")
	}
}

func TestIconsAreServedFromTheCacheWhenThereIsOne(t *testing.T) {
	index := &Index{}
	index.Set([]App{{ID: "known", Name: "Known"}})
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, cacheName("known")), []byte("PNGDATA"), 0o644); err != nil {
		t.Fatal(err)
	}
	rec := httptest.NewRecorder()
	Icons(index, dir)(http.NotFoundHandler()).ServeHTTP(rec, httptest.NewRequest("GET", "/icon?id=known", nil))
	if rec.Code != http.StatusOK || rec.Body.String() != "PNGDATA" || rec.Header().Get("Content-Type") != "image/png" {
		t.Fatalf("cached icon: %d %q %q", rec.Code, rec.Body.String(), rec.Header().Get("Content-Type"))
	}
}

func TestWarmingMakesOnlyTheIconsThatAreMissing(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, cacheName("have")), []byte("old"), 0o644); err != nil {
		t.Fatal(err)
	}
	var asked []string
	old := extract
	extract = func(id string) ([]byte, error) {
		asked = append(asked, id)
		if id == "broken" {
			return nil, os.ErrInvalid
		}
		return []byte("new:" + id), nil
	}
	defer func() { extract = old }()
	warmIcons(context.Background(), []App{{ID: "have"}, {ID: "need"}, {ID: "broken"}}, dir)
	if len(asked) != 2 || asked[0] != "need" || asked[1] != "broken" {
		t.Fatalf("asked the shell for %v, want only the missing ones", asked)
	}
	if got, _ := os.ReadFile(filepath.Join(dir, cacheName("need"))); string(got) != "new:need" {
		t.Fatalf("the new icon was not cached: %q", got)
	}
	if got, _ := os.ReadFile(filepath.Join(dir, cacheName("have"))); string(got) != "old" {
		t.Fatal("an icon that was cached must be left alone")
	}
	if _, err := os.Stat(filepath.Join(dir, cacheName("broken"))); err == nil {
		t.Fatal("an icon that could not be made must not leave a file")
	}
}

func TestCacheNamesAreStableDistinctAndSafe(t *testing.T) {
	a, b := cacheName(`{6D80}\7-Zip\7zFM.exe`), cacheName(`Chrome`)
	if a == b || a != cacheName(`{6D80}\7-Zip\7zFM.exe`) {
		t.Fatalf("names %s and %s", a, b)
	}
	if filepath.Base(a) != a {
		t.Fatalf("%q is not a plain file name", a)
	}
}

// TestRealIcons extracts the icon of every kept app on this machine; it runs only with AGI_REAL_APPS=1.
// With AGI_ICON_DUMP=<folder> it also saves the PNGs there, to look at.
func TestRealIcons(t *testing.T) {
	if os.Getenv("AGI_REAL_APPS") != "1" {
		t.Skip("set AGI_REAL_APPS=1 to extract the real icons")
	}
	index := loadReal(t)
	dump := os.Getenv("AGI_ICON_DUMP")
	var failed []string
	var total, slowest time.Duration
	count := 0
	for _, app := range index {
		start := time.Now()
		data, err := extractIcon(app.ID)
		took := time.Since(start)
		if err != nil {
			failed = append(failed, app.Name+": "+err.Error())
			continue
		}
		count++
		total += took
		slowest = max(slowest, took)
		if dump != "" {
			_ = os.MkdirAll(dump, 0o755)
			_ = os.WriteFile(filepath.Join(dump, cacheName(app.ID)), data, 0o644)
		}
	}
	t.Logf("%d icons in %v on average (slowest %v); %d without one", count, (total / time.Duration(max(count, 1))).Round(time.Millisecond), slowest.Round(time.Millisecond), len(failed))
	for _, f := range failed {
		t.Logf("no icon: %s", f)
	}
}
