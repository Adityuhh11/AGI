package web

import (
	"errors"
	"reflect"
	"testing"

	"agi/internal/config"
)

// fake records what would have been started.
type fake struct {
	exe       string
	args      []string
	shelled   string
	installed map[string]string
}

func newWeb(f *fake, browser, searchURL string, links ...config.Quicklink) *Web {
	w := New(browser, searchURL, links)
	w.find = func(name string) (string, error) {
		if path, ok := f.installed[name]; ok {
			return path, nil
		}
		return "", errors.New("not registered")
	}
	w.start = func(exe string, args ...string) error { f.exe, f.args = exe, args; return nil }
	w.shell = func(address string) error { f.shelled = address; return nil }
	return w
}

func TestAFirefoxBasedBrowserSearchesWithItsOwnEngine(t *testing.T) {
	f := &fake{installed: map[string]string{"zen": `C:\Program Files\Zen Browser\zen.exe`}}
	if err := newWeb(f, "zen", "").Search("rust lifetimes & borrowing"); err != nil {
		t.Fatal(err)
	}
	if f.exe != `C:\Program Files\Zen Browser\zen.exe` || !reflect.DeepEqual(f.args, []string{"-search", "rust lifetimes & borrowing"}) {
		t.Fatalf("started %q %q", f.exe, f.args)
	}
}

func TestOtherBrowsersAndAnExplicitTemplateSearchByAddress(t *testing.T) {
	f := &fake{installed: map[string]string{"chrome": `C:\chrome.exe`, "zen": `C:\zen.exe`}}
	if err := newWeb(f, "chrome", "").Search("a b&c"); err != nil {
		t.Fatal(err)
	}
	if f.exe != `C:\chrome.exe` || !reflect.DeepEqual(f.args, []string{"https://duckduckgo.com/?q=a+b%26c"}) {
		t.Fatalf("chrome started %q %q", f.exe, f.args)
	}
	if err := newWeb(f, "zen", "https://www.google.com/search?q={q}").Search("go"); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(f.args, []string{"https://www.google.com/search?q=go"}) {
		t.Fatalf("an explicit template must win over -search, got %q", f.args)
	}
}

func TestNoBrowserSetUsesTheWindowsDefault(t *testing.T) {
	f := &fake{}
	if err := newWeb(f, "", "").Search("hello world"); err != nil {
		t.Fatal(err)
	}
	if f.shelled != "https://duckduckgo.com/?q=hello+world" || f.exe != "" {
		t.Fatalf("shelled %q, started %q", f.shelled, f.exe)
	}
	if err := newWeb(f, "", "").Open("https://example.com"); err != nil || f.shelled != "https://example.com" {
		t.Fatalf("Open shelled %q, %v", f.shelled, err)
	}
}

func TestABrowserThatIsNotInstalledIsAnErrorNotAFallback(t *testing.T) {
	f := &fake{}
	err := newWeb(f, "zen", "").Search("x")
	if err == nil {
		t.Fatal("a missing browser must be an error")
	}
	if f.exe != "" || f.shelled != "" {
		t.Fatal("nothing may be started when the browser is missing (no silent fallback to another browser)")
	}
	if err := newWeb(f, "zen", "").Search("  "); err == nil {
		t.Fatal("nothing to search for must be an error")
	}
}

func TestABrowserCanBeGivenAsAPathOrNameInAnyForm(t *testing.T) {
	for _, name := range []string{"Zen", "zen.exe", `C:\Program Files\Zen Browser\zen.exe`} {
		if got := browserName(name); got != "zen" {
			t.Errorf("browserName(%q) = %q, want zen", name, got)
		}
	}
	f := &fake{}
	if err := newWeb(f, `C:\Apps\zen.exe`, "").Search("x"); err != nil || f.exe != `C:\Apps\zen.exe` {
		t.Fatalf("a path is used as it is, got %q, %v", f.exe, err)
	}
}

func TestExpandEscapesForAnAddress(t *testing.T) {
	tests := []struct{ template, text, want string }{
		{"https://x/?q={q}", "a b", "https://x/?q=a+b"},
		{"https://x/?q={q}", "a&b=c", "https://x/?q=a%26b%3Dc"},
		{"https://x/?q={q}", "café 日本", "https://x/?q=caf%C3%A9+%E6%97%A5%E6%9C%AC"},
		{"https://x/{q}/y", "50% off", "https://x/50%25+off/y"},
	}
	for _, tt := range tests {
		if got := Expand(tt.template, tt.text); got != tt.want {
			t.Errorf("Expand(%q, %q) = %q, want %q", tt.template, tt.text, got, tt.want)
		}
	}
}

func TestQuicklinksNeedTheTriggerAndSomeText(t *testing.T) {
	gh := config.Quicklink{Trigger: "gh", Title: "GitHub", URL: "https://github.com/search?q={q}"}
	w := newWeb(&fake{}, "", "", gh)
	tests := []struct {
		typed string
		ok    bool
		text  string
	}{
		{"gh rust lifetimes", true, "rust lifetimes"},
		{"  GH   rust ", true, "rust"},
		{"gh", false, ""},         // the trigger alone is an ordinary search
		{"gh ", false, ""},        // so is a trigger with nothing after it
		{"ghost rust", false, ""}, // a word that merely starts like the trigger
		{"rust gh", false, ""},
	}
	for _, tt := range tests {
		link, text, ok := w.Quicklink(tt.typed)
		if ok != tt.ok || text != tt.text || (ok && link.Trigger != "gh") {
			t.Errorf("Quicklink(%q) = %v, %q, %v; want %v, %q", tt.typed, link.Trigger, text, ok, tt.ok, tt.text)
		}
	}
	if _, ok := w.Find("GH"); !ok {
		t.Error("Find must ignore case")
	}
}

func TestUpdateChangesTheBrowserAndTheQuicklinksOfARunningWeb(t *testing.T) {
	f := &fake{installed: map[string]string{"zen": `C:\zen.exe`, "chrome": `C:\chrome.exe`}}
	w := newWeb(f, "zen", "", config.Quicklink{Trigger: "gh", Title: "GitHub", URL: "https://github.com/search?q={q}"})
	if _, _, ok := w.Quicklink("gh rust"); !ok {
		t.Fatal("the first quicklink is missing")
	}
	w.Update("chrome", "", []config.Quicklink{{Trigger: "yt", Title: "YouTube", URL: "https://youtube.com/results?search_query={q}"}})
	if _, _, ok := w.Quicklink("gh rust"); ok {
		t.Fatal("the old quicklink survived the update")
	}
	if link, _, ok := w.Quicklink("yt cats"); !ok || link.Title != "YouTube" {
		t.Fatal("the new quicklink is missing")
	}
	if err := w.Search("go"); err != nil || f.exe != `C:\chrome.exe` {
		t.Fatalf("searched with %q (%v), want the new browser", f.exe, err)
	}
}
