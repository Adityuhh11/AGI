package palette

import (
	"path/filepath"
	"strings"
	"testing"

	"agi/internal/apps"
	"agi/internal/state"
)

func newService(t *testing.T, names ...string) (*Service, *int) {
	t.Helper()
	catalog := apps.NewCatalog("", "")
	var list []apps.App
	for _, n := range names {
		list = append(list, apps.App{ID: "id:" + n, Name: n})
	}
	catalog.Index.Set(list)
	history, err := state.Open(filepath.Join(t.TempDir(), "state.json"))
	if err != nil {
		t.Fatal(err)
	}
	dismissed := 0
	return New(catalog, history, func() { dismissed++ }), &dismissed
}

func titles(a Answer) string {
	var out []string
	for _, item := range a.Items {
		out = append(out, item.Title)
	}
	return strings.Join(out, "|")
}

func TestParseTakesAModePrefixOffTheText(t *testing.T) {
	tests := []struct {
		raw, chip, mode, text string
	}{
		{"chrome", "", ModeAll, "chrome"},
		{"  chrome ", ModeFiles, ModeFiles, "chrome"},
		{"app: chrome", ModeAll, ModeApps, "chrome"},
		{"APPS:chrome", ModeFiles, ModeApps, "chrome"}, // the prefix wins over the chip
		{"note to self: buy milk", ModeAll, ModeAll, "note to self: buy milk"},
		{"code: retry", ModeAll, ModeAll, "code: retry"}, // a file scope: the file search handles it
		{"app:", ModeAll, ModeApps, ""},
	}
	for _, tt := range tests {
		if mode, text := parse(tt.raw, tt.chip); mode != tt.mode || text != tt.text {
			t.Errorf("parse(%q, %q) = %q, %q; want %q, %q", tt.raw, tt.chip, mode, text, tt.mode, tt.text)
		}
	}
}

func TestStrongMatchesShowBesideFilesAndWeakOnesOnlyForShortcuts(t *testing.T) {
	s, _ := newService(t, "Visual Studio Code", "PowerPoint", "Notes", "Command Prompt")

	if got := titles(s.Query("vsc", ModeAll)); got != "Visual Studio Code" {
		t.Errorf("vsc showed %q", got)
	}
	// ppt matches PowerPoint only weakly, but it is short enough to be a shortcut.
	if got := titles(s.Query("ppt", ModeAll)); !strings.HasPrefix(got, "PowerPoint") {
		t.Errorf("ppt showed %q, want PowerPoint first", got)
	}
	// A question is not a shortcut: its letters appearing in an app name is a coincidence.
	if got := titles(s.Query("meeting notes from last march", ModeAll)); got != "" {
		t.Errorf("a question showed apps: %q", got)
	}
	if got := titles(s.Query("   ", ModeAll)); got != "" {
		t.Errorf("empty text showed %q", got)
	}
}

func TestTheAppsModeListsWeakMatchesTooAndFilesModeNone(t *testing.T) {
	s, _ := newService(t, "PowerPoint", "Command Prompt")
	if got := titles(s.Query("ppt", ModeApps)); !strings.HasPrefix(got, "PowerPoint") {
		t.Errorf("Apps mode showed %q", got)
	}
	if got := titles(s.Query("app: ppt", ModeAll)); !strings.HasPrefix(got, "PowerPoint") {
		t.Errorf("the app: prefix showed %q", got)
	}
	if a := s.Query("app: ppt", ModeAll); a.Mode != ModeApps || a.Text != "ppt" {
		t.Errorf("the answer must report the mode and text it used, got %+v", a)
	}
	if got := titles(s.Query("power", ModeFiles)); got != "" {
		t.Errorf("Files mode showed apps: %q", got)
	}
}

func TestAtMostFiveStrongMatchesShow(t *testing.T) {
	s, _ := newService(t, "Alpha 1", "Alpha 2", "Alpha 3", "Alpha 4", "Alpha 5", "Alpha 6", "Alpha 7")
	if n := len(s.Query("alpha", ModeAll).Items); n != strongRows {
		t.Errorf("%d rows, want %d", n, strongRows)
	}
}

func TestAnItemNamesItsIconAndOnlyOffersActionsItCanDo(t *testing.T) {
	catalog := apps.NewCatalog("", "")
	file := filepath.Join(t.TempDir(), "tool.exe")
	catalog.Index.Set([]apps.App{{ID: file, Name: "Tool"}, {ID: "Pkg!App", Name: "Store Thing"}})
	history, _ := state.Open(filepath.Join(t.TempDir(), "state.json"))
	s := New(catalog, history, func() {})
	byTitle := map[string]Item{}
	for _, item := range s.Query("t", ModeApps).Items {
		byTitle[item.Title] = item
	}
	tool, store := byTitle["Tool"], byTitle["Store Thing"]
	if !strings.HasPrefix(tool.Icon, "/icon?id=") || tool.Kind != "app" || !strings.HasPrefix(tool.ID, "app:") {
		t.Errorf("item = %+v", tool)
	}
	if len(tool.Actions) != 2 {
		t.Errorf("an app with a file offers reveal and admin, got %v", tool.Actions)
	}
	if len(store.Actions) != 0 {
		t.Errorf("a Store app has no file to reveal or elevate, got %v", store.Actions)
	}
}

func TestRunningAnAppThatWasUninstalledRemovesItAndDoesNotCloseTheBar(t *testing.T) {
	s, dismissed := newService(t)
	gone := filepath.Join(t.TempDir(), "uninstalled.exe")
	s.catalog.Index.Set([]apps.App{{ID: gone, Name: "Uninstalled"}})
	err := s.Do("app:"+gone, "open", "unin")
	if err == nil || !strings.Contains(err.Error(), "no longer installed") {
		t.Fatalf("Do = %v", err)
	}
	if s.catalog.Index.Len() != 0 {
		t.Error("the dead entry must be dropped from the list")
	}
	if *dismissed != 0 {
		t.Error("the bar must stay open to show the problem")
	}
	if err := s.Do("app:nothing", "open", ""); err == nil {
		t.Error("an app that is not in the list must be an error")
	}
	if err := s.Do("file:x", "open", ""); err == nil {
		t.Error("an id of another kind must be refused here")
	}
}
