package palette

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"agi/internal/apps"
	"agi/internal/config"
	"agi/internal/state"
	"agi/internal/web"
)

// fakeBrowser records what the bar opened.
type fakeBrowser struct {
	links    []config.Quicklink
	searched []string
	opened   []string
}

func (b *fakeBrowser) Search(text string) error  { b.searched = append(b.searched, text); return nil }
func (b *fakeBrowser) Open(address string) error { b.opened = append(b.opened, address); return nil }
func (b *fakeBrowser) Quicklink(typed string) (config.Quicklink, string, bool) {
	return web.New("", "", b.links).Quicklink(typed)
}
func (b *fakeBrowser) Find(trigger string) (config.Quicklink, bool) {
	return web.New("", "", b.links).Find(trigger)
}

// fakeFiles records what was opened.
type fakeFiles struct{ opened, revealed []string }

func (f *fakeFiles) Open(path string, _ int) error { f.opened = append(f.opened, path); return nil }
func (f *fakeFiles) Reveal(path string) error      { f.revealed = append(f.revealed, path); return nil }

var gh = config.Quicklink{Trigger: "gh", Title: "GitHub", URL: "https://github.com/search?q={q}"}

func newService(t *testing.T, names ...string) (*Service, *int) {
	s, dismissed, _ := newServiceWithBrowser(t, names...)
	return s, dismissed
}

func newServiceWithBrowser(t *testing.T, names ...string) (*Service, *int, *fakeBrowser) {
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
	browser := &fakeBrowser{links: []config.Quicklink{gh}}
	return New(catalog, history, browser, &fakeFiles{}, &fakeClips{}, func() { dismissed++ }), &dismissed, browser
}

// titles lists the app rows of an answer; other kinds have their own tests.
func titles(a Answer) string {
	var out []string
	for _, item := range a.Items {
		if item.Kind == "app" {
			out = append(out, item.Title)
		}
	}
	return strings.Join(out, "|")
}

func kinds(a Answer) string {
	var out []string
	for _, item := range a.Items {
		out = append(out, item.Kind)
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
		{"? rust lifetimes", ModeAll, ModeWeb, "rust lifetimes"},
		{"?rust", ModeFiles, ModeWeb, "rust"},
		{"web: rust", ModeAll, ModeWeb, "rust"},
		{"what is this? a question", ModeAll, ModeAll, "what is this? a question"}, // only a leading ? counts
		{"#work budget", ModeAll, ModeTags, "work budget"},
		{"work", ModeTags, ModeTags, "work"},   // the chip alone is enough
		{"#work", ModeFiles, ModeTags, "work"}, // the prefix wins over the chip
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

func TestAWebSearchIsTheLastRowInEveryModeButAppsAndFiles(t *testing.T) {
	s, _ := newService(t, "Visual Studio Code")
	if got := kinds(s.Query("vsc", ModeAll)); got != "app|web" {
		t.Errorf("all showed %q, want the app then the web row", got)
	}
	a := s.Query("rust lifetimes", ModeAll)
	if got := kinds(a); got != "web" || a.Items[0].Title != "Search the web for “rust lifetimes”" || a.Items[0].ID != "web:rust lifetimes" {
		t.Errorf("a question showed %q %+v", got, a.Items)
	}
	if got := kinds(s.Query("vsc", ModeWeb)); got != "web" {
		t.Errorf("the Web mode showed %q, want only the web row", got)
	}
	if got := kinds(s.Query("? rust", ModeAll)); got != "web" {
		t.Errorf("the ? prefix showed %q", got)
	}
	for _, mode := range []string{ModeApps, ModeFiles} {
		if got := kinds(s.Query("rust lifetimes", mode)); strings.Contains(got, "web") {
			t.Errorf("%s mode must not offer a web search, got %q", mode, got)
		}
	}
	if got := kinds(s.Query("x", ModeAll)); strings.Contains(got, "web") {
		t.Errorf("one letter is not a question, got %q", got)
	}
}

func TestAQuicklinkTriggerPutsItsRowFirst(t *testing.T) {
	s, _ := newService(t, "Gh Tool")
	a := s.Query("gh rust lifetimes", ModeAll)
	if got := kinds(a); got != "quicklink|web" {
		t.Fatalf("kinds %q, want the quicklink first and the web search last", got)
	}
	if a.Items[0].Title != "GitHub: rust lifetimes" || a.Items[0].ID != "quicklink:gh:rust lifetimes" {
		t.Errorf("quicklink row = %+v", a.Items[0])
	}
	if got := kinds(s.Query("gh rust", ModeApps)); strings.Contains(got, "quicklink") {
		t.Errorf("the Apps mode must not show quicklinks, got %q", got)
	}
	if got := kinds(s.Query("gh", ModeAll)); strings.Contains(got, "quicklink") {
		t.Errorf("a trigger alone is an ordinary search, got %q", got)
	}
}

func TestOpeningWebRowsGoesToTheBrowserAndClosesTheBar(t *testing.T) {
	s, dismissed, browser := newServiceWithBrowser(t)
	if err := s.Do("web:rust lifetimes", "open", "rust lifetimes"); err != nil {
		t.Fatal(err)
	}
	if len(browser.searched) != 1 || browser.searched[0] != "rust lifetimes" || *dismissed != 1 {
		t.Fatalf("searched %v, dismissed %d", browser.searched, *dismissed)
	}
	if err := s.Do("quicklink:gh:a b&c", "open", "gh a b&c"); err != nil {
		t.Fatal(err)
	}
	if len(browser.opened) != 1 || browser.opened[0] != "https://github.com/search?q=a+b%26c" || *dismissed != 2 {
		t.Fatalf("opened %v, dismissed %d", browser.opened, *dismissed)
	}
	if err := s.Do("quicklink:nope:x", "open", ""); err == nil {
		t.Error("a quicklink that is gone must be an error")
	}
	if err := s.Do("web:x", "admin", ""); err == nil {
		t.Error("a web row has no administrator action")
	}
}

func TestAtMostFiveStrongMatchesShow(t *testing.T) {
	s, _ := newService(t, "Alpha 1", "Alpha 2", "Alpha 3", "Alpha 4", "Alpha 5", "Alpha 6", "Alpha 7")
	if n := len(strings.Split(titles(s.Query("alpha", ModeAll)), "|")); n != strongRows {
		t.Errorf("%d rows, want %d", n, strongRows)
	}
}

func TestAnItemNamesItsIconAndOnlyOffersActionsItCanDo(t *testing.T) {
	catalog := apps.NewCatalog("", "")
	file := filepath.Join(t.TempDir(), "tool.exe")
	catalog.Index.Set([]apps.App{{ID: file, Name: "Tool"}, {ID: "Pkg!App", Name: "Store Thing"}})
	history, _ := state.Open(filepath.Join(t.TempDir(), "state.json"))
	s := New(catalog, history, &fakeBrowser{}, &fakeFiles{}, &fakeClips{}, func() {})
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
	if err := s.Do("tag:x", "open", ""); err == nil {
		t.Error("an id of another kind must be refused here")
	}
}

func TestTheTagsModeListsTagsThenWhatCarriesOne(t *testing.T) {
	s, _ := newService(t, "Visual Studio Code", "Calculator")
	file := filepath.Join(t.TempDir(), "budget.xlsx")
	if err := os.WriteFile(file, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	gone := filepath.Join(t.TempDir(), "deleted.pdf") // tagged once, deleted since
	for _, id := range []string{"app:id:Visual Studio Code", "file:" + file, "file:" + gone} {
		if _, err := s.Tag(id, "Work"); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := s.Tag("app:id:Calculator", "math"); err != nil {
		t.Fatal(err)
	}

	list := s.Query("#", ModeAll)
	if list.Mode != ModeTags || kinds(list) != "tag|tag" || list.Items[0].Title != "#work" || list.Items[0].Subtitle != "3 items" {
		t.Fatalf("# listed %q %+v", kinds(list), list.Items)
	}
	if got := titlesOf(s.Query("#wo", ModeAll)); got != "#work" {
		t.Errorf("#wo listed %q, want the matching tag", got)
	}
	work := s.Query("#work", ModeAll)
	if got := titlesOf(work); got != "Visual Studio Code|budget.xlsx" || kinds(work) != "app|file" {
		t.Errorf("#work listed %q (%s); the deleted file must be left out", got, kinds(work))
	}
	if got := titlesOf(s.Query("#work budget", ModeAll)); got != "budget.xlsx" {
		t.Errorf("#work budget listed %q, want only the match", got)
	}
	if work.Items[1].Subtitle != filepath.Dir(file) || len(work.Items[1].Actions) == 0 {
		t.Errorf("a tagged file shows its folder and can be revealed: %+v", work.Items[1])
	}
}

func TestTaggingTogglesAndReportsWhatTheThingCarries(t *testing.T) {
	s, _ := newService(t)
	tags, err := s.Tag("app:x", "#Dev Tools")
	if err != nil || len(tags) != 1 || tags[0] != "dev-tools" {
		t.Fatalf("Tag = %v, %v", tags, err)
	}
	if got := s.TagsOf([]string{"app:x", "app:y"}); len(got) != 1 || got["app:x"][0] != "dev-tools" {
		t.Fatalf("TagsOf = %v (things without tags are left out)", got)
	}
	if tags, err := s.Tag("app:x", "dev tools"); err != nil || len(tags) != 0 {
		t.Fatalf("a second Tag must take it off, got %v, %v", tags, err)
	}
	if _, err := s.Tag("web:search", "x"); err == nil {
		t.Error("only apps and files can be tagged")
	}
	if _, err := s.Tag("app:x", "!!!"); err == nil {
		t.Error("an unusable tag must be refused")
	}
	if got := s.Tags(); len(got) != 0 {
		t.Errorf("an emptied tag must be gone, got %v", got)
	}
}

func TestTaggedFilesOpenThroughTheFilesService(t *testing.T) {
	catalog := apps.NewCatalog("", "")
	history, _ := state.Open(filepath.Join(t.TempDir(), "state.json"))
	f := &fakeFiles{}
	s := New(catalog, history, &fakeBrowser{}, f, &fakeClips{}, func() {})
	if err := s.Do(`file:C:\docs\a.pdf`, "open", ""); err != nil || len(f.opened) != 1 || f.opened[0] != `C:\docs\a.pdf` {
		t.Fatalf("opened %v, %v", f.opened, err)
	}
	if err := s.Do(`file:C:\docs\a.pdf`, "reveal", ""); err != nil || len(f.revealed) != 1 {
		t.Fatalf("revealed %v, %v", f.revealed, err)
	}
	if err := s.Do(`file:x`, "admin", ""); err == nil {
		t.Error("a file has no administrator action")
	}
}

// titlesOf lists the titles of every row of an answer.
func titlesOf(a Answer) string {
	var out []string
	for _, item := range a.Items {
		out = append(out, item.Title)
	}
	return strings.Join(out, "|")
}

func TestPinnedRowsAreInTheOrderPinnedAndGoneOnesAreLeftOut(t *testing.T) {
	s, _ := newService(t, "Visual Studio Code", "Calculator")
	file := filepath.Join(t.TempDir(), "budget.xlsx")
	if err := os.WriteFile(file, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	for _, id := range []string{"file:" + file, "quicklink:gh:", "app:id:Calculator", "app:id:Gone"} {
		if on, err := s.TogglePin(id); err != nil || !on {
			t.Fatalf("pin %s = %v, %v", id, on, err)
		}
	}
	var got []string
	for _, item := range s.Pinned() {
		got = append(got, item.Kind+":"+item.Title)
	}
	if want := "file:budget.xlsx|quicklink:GitHub|app:Calculator"; strings.Join(got, "|") != want {
		t.Fatalf("pinned rows %v, want %s (the uninstalled app is hidden, not unpinned)", got, want)
	}
	if len(s.PinIDs()) != 4 {
		t.Errorf("a gone app must stay pinned in case it comes back, ids %v", s.PinIDs())
	}
	if pinned := s.Pinned(); pinned[1].ID != "quicklink:gh:" {
		t.Errorf("a pinned quicklink is its trigger with no text, got %q", pinned[1].ID)
	}
}

func TestAQuicklinkIsPinnedAsItsTriggerAndOnlyPinnableThingsArePinned(t *testing.T) {
	s, _ := newService(t)
	if on, err := s.TogglePin("quicklink:gh:rust lifetimes"); err != nil || !on {
		t.Fatalf("pin = %v, %v", on, err)
	}
	if ids := s.PinIDs(); len(ids) != 1 || ids[0] != "quicklink:gh:" {
		t.Fatalf("pinned %v, want the trigger alone", ids)
	}
	if on, err := s.TogglePin("quicklink:gh:other text"); err != nil || on {
		t.Fatalf("the same quicklink with other text must unpin it, got %v, %v", on, err)
	}
	for _, id := range []string{"web:rust", "tag:work", "nonsense"} {
		if _, err := s.TogglePin(id); err == nil {
			t.Errorf("%q cannot be pinned", id)
		}
	}
}
