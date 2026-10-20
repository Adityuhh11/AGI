package config

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestPathIsUnderTheUsersConfigFolder(t *testing.T) {
	t.Setenv("APPDATA", `C:\Users\someone\AppData\Roaming`)
	got, err := Path()
	if err != nil {
		t.Fatal(err)
	}
	if want := `C:\Users\someone\AppData\Roaming\AGI\config.toml`; got != want {
		t.Fatalf("Path() = %q, want %q", got, want)
	}
}

func TestLoadCreatesTheFileWithDefaultsWhenMissing(t *testing.T) {
	path := filepath.Join(t.TempDir(), "AGI", "config.toml")
	got, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if got.Hotkey != DefaultHotkey {
		t.Fatalf("hotkey = %q, want the default %q", got.Hotkey, DefaultHotkey)
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("the config file was not created: %v", err)
	}
	again, err := Load(path)
	if err != nil {
		t.Fatalf("the created file does not load: %v", err)
	}
	if again.Hotkey != got.Hotkey || strings.Join(again.Roots, "|") != strings.Join(got.Roots, "|") ||
		again.Sidecar != got.Sidecar {
		t.Fatalf("round trip changed the config: %+v became %+v", got, again)
	}
}

func TestLoadKeepsDefaultsForKeysThatAreNotSet(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.toml")
	writeFile(t, path, "hotkey = \"Ctrl+Alt+K\"\n")
	got, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if got.Hotkey != "Ctrl+Alt+K" {
		t.Fatalf("hotkey = %q, want Ctrl+Alt+K", got.Hotkey)
	}
	if want := Default(); got.Search != want.Search {
		t.Fatalf("search = %+v, want the default %+v", got.Search, want.Search)
	}
}

func TestLoadReplacesTheRootListWhenItIsSet(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.toml")
	writeFile(t, path, "roots = ['C:\\Docs', 'D:\\Photos']\n")
	got, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Roots) != 2 || got.Roots[0] != `C:\Docs` || got.Roots[1] != `D:\Photos` {
		t.Fatalf("roots = %v", got.Roots)
	}
}

func TestLoadRejectsBadFiles(t *testing.T) {
	tests := []struct {
		name    string
		content string
		wantErr string
	}{
		{"not toml", "hotkey = = =", "parse"},
		{"unknown key", "hotkeys = 'x'\n", "parse"},
		{"unknown table key", "[sidecar]\nfolder = 'x'\n", "parse"},
		{"wrong type", "roots = 'C:\\Docs'\n", "parse"},
		{"empty hotkey", "hotkey = ''\n", "hotkey must not be empty"},
		{"relative root", "roots = ['docs']\n", "must be an absolute path"},
		{"duplicate root", "roots = ['C:\\Docs', 'c:\\docs\\']\n", "repeats"},
		{"relative exclude", "roots = ['C:\\Docs']\nexclude = ['vendor']\n", "must be an absolute path"},
		{"exclude outside every root", "roots = ['C:\\Docs']\nexclude = ['D:\\Other']\n", "not inside any root"},
		{"search_url without a place for the text", "search_url = 'https://example.com/'\n", "must contain {q}"},
		{"search_url that is not a web address", "search_url = 'example.com/?q={q}'\n", "must start with https://"},
		{"quicklink without a trigger", "[[quicklink]]\ntrigger = ''\ntitle = 'x'\nurl = 'https://x/?q={q}'\n", "one word"},
		{"quicklink with a two-word trigger", "[[quicklink]]\ntrigger = 'a b'\ntitle = 'x'\nurl = 'https://x/?q={q}'\n", "one word"},
		{"quicklink without a title", "[[quicklink]]\ntrigger = 'gh'\nurl = 'https://x/?q={q}'\n", "needs a title"},
		{"quicklink twice", "[[quicklink]]\ntrigger = 'gh'\ntitle = 'a'\nurl = 'https://x/?q={q}'\n[[quicklink]]\ntrigger = 'GH'\ntitle = 'b'\nurl = 'https://y/?q={q}'\n", "used twice"},
		{"quicklink with two places for the text", "[[quicklink]]\ntrigger = 'gh'\ntitle = 'a'\nurl = 'https://x/{q}/{q}'\n", "once"},
		{"snippet without a name", "[[snippet]]\ntext = 'x'\n", "needs a name"},
		{"snippet without a text", "[[snippet]]\nname = 'Address'\n", "needs a text"},
		{"snippet name twice", "[[snippet]]\nname = 'Address'\ntext = 'a'\n[[snippet]]\nname = 'ADDRESS'\ntext = 'b'\n", "used twice"},
		{"snippet keyword with a space", "[[snippet]]\nname = 'A'\nkeyword = 'two words'\ntext = 'a'\n", "one word"},
		{"snippet keyword used twice", "[[snippet]]\nname = 'A'\nkeyword = 'ad'\ntext = 'a'\n[[snippet]]\nname = 'B'\nkeyword = 'AD'\ntext = 'b'\n", "already"},
		{"snippet keyword that is a quicklink trigger", "[[quicklink]]\ntrigger = 'gh'\ntitle = 'GitHub'\nurl = 'https://x/?q={q}'\n[[snippet]]\nname = 'A'\nkeyword = 'GH'\ntext = 'a'\n", "quicklink trigger"},
		{"clipboard hotkey that is the main hotkey", "hotkey = 'Alt+Shift+Space'\n[clipboard]\nhotkey = 'alt + shift + space'\n", "launcher's own hotkey"},
		{"clipboard ignore with an empty program name", "[clipboard]\nignore = ['Notepad', ' ']\n", "program name must not be empty"},
		{"exclude that is a root", "roots = ['C:\\Docs']\nexclude = ['C:\\Docs']\n", "not inside any root"},
		{"unknown pdf mode", "[search]\npace = 'fast'\npdf_pages = 'some'\n", "pdf_pages"},
		{"unknown photo mode", "[search]\npace = 'fast'\nphotos = 'pixels'\n", "photos"},
		{"unknown video mode", "[search]\npace = 'fast'\nvideos = 'off'\n", "videos"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "config.toml")
			writeFile(t, path, tt.content)
			_, err := Load(path)
			if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("Load error = %v, want one containing %q", err, tt.wantErr)
			}
		})
	}
}

func TestSaveCreatesMissingFolders(t *testing.T) {
	path := filepath.Join(t.TempDir(), "a", "b", "config.toml")
	if err := (Config{Hotkey: "Alt+Shift+Space"}).Save(path); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatal(err)
	}
}

func TestSourceIDsAreStableCaseInsensitiveAndAcceptedByTheEngine(t *testing.T) {
	enginePattern := regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]{0,63}$`)
	tests := []struct {
		a, b string
		same bool
	}{
		{`C:\Users\me\Docs`, `C:\Users\me\Docs`, true},
		{`C:\Users\me\Docs`, `c:\users\ME\docs`, true},
		{`C:\Users\me\Docs`, `C:\Users\me\Docs\`, true},
		{`C:\Users\me\Docs`, `C:\Users\me\Docs\.`, true},
		{`C:\Users\me\Docs`, `C:\Users\me\Photos`, false},
		{`C:\Users\me\Docs`, `D:\Users\me\Docs`, false},
	}
	for _, tt := range tests {
		a, b := SourceID(tt.a), SourceID(tt.b)
		if (a == b) != tt.same {
			t.Errorf("SourceID(%q) = %s, SourceID(%q) = %s, same = %v, want %v", tt.a, a, tt.b, b, a == b, tt.same)
		}
		if !enginePattern.MatchString(a) {
			t.Errorf("SourceID(%q) = %q does not fit the engine's source id rule", tt.a, a)
		}
	}
}

func TestAFreshInstallIndexesNothingAndHasFileSearchOff(t *testing.T) {
	cfg := Default()
	if len(cfg.Roots) != 0 || cfg.Search.Enabled || cfg.Search.Pace != "balanced" || cfg.Hotkey != DefaultHotkey {
		t.Fatalf("unexpected defaults: %+v", cfg)
	}
	if err := cfg.Validate(); err != nil {
		t.Fatal(err)
	}
}

func TestTheRetiredSidecarTableLoadsAndIsNotWrittenBack(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.toml")
	writeFile(t, path, "hotkey = 'Ctrl+Alt+K'\n\n[sidecar]\ndir = 'C:\\\\dev\\\\AGI\\\\sidecar'\n")
	got, err := Load(path)
	if err != nil {
		t.Fatalf("a file with the retired [sidecar] table must still load: %v", err)
	}
	if got.Hotkey != "Ctrl+Alt+K" || got.Sidecar != nil {
		t.Fatalf("got %+v", got)
	}
	if err := got.Save(path); err != nil {
		t.Fatal(err)
	}
	saved, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(saved), "sidecar") {
		t.Fatalf("the retired table was written back:\n%s", saved)
	}
}

func TestLoadReadsTheSearchSettings(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.toml")
	writeFile(t, path, "[search]\nenabled = true\npace = 'gentle'\n")
	got, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if !got.Search.Enabled || got.Search.Pace != "gentle" {
		t.Fatalf("search = %+v", got.Search)
	}
	// A file from before the indexing choices keeps their defaults.
	if got.Search.PDFPages != "auto" || got.Search.Photos != "content" || got.Search.Videos != "name" {
		t.Fatalf("indexing choices = %+v, want the defaults", got.Search)
	}
}

func TestLoadReadsTheIndexingChoices(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.toml")
	writeFile(t, path, "[search]\npace = 'fast'\npdf_pages = 'text'\nphotos = 'name'\nvideos = 'content'\n")
	got, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if got.Search.PDFPages != "text" || got.Search.Photos != "name" || got.Search.Videos != "content" {
		t.Fatalf("search = %+v", got.Search)
	}
}

func TestExcludedUnderGivesPathsBelowTheRootInTheEnginesForm(t *testing.T) {
	exclude := []string{`C:\Docs\Vendor\Skills`, `c:\docs\other`, `C:\Docs`, `C:\Documents\x`, `D:\Docs\Vendor`}
	got := ExcludedUnder(`C:\Docs`, exclude)
	if want := "Vendor/Skills|other"; strings.Join(got, "|") != want {
		t.Fatalf("ExcludedUnder = %v, want %s", got, want)
	}
}

func TestLoadReadsExcludedFolders(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.toml")
	writeFile(t, path, "roots = ['C:\\Docs']\nexclude = ['C:\\Docs\\vendor']\n")
	got, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Exclude) != 1 || got.Exclude[0] != `C:\Docs\vendor` {
		t.Fatalf("exclude = %v", got.Exclude)
	}
}

func TestLoadReadsTheBrowserAndQuicklinks(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.toml")
	writeFile(t, path, "browser = 'zen'\nsearch_url = 'https://duckduckgo.com/?q={q}'\n\n[[quicklink]]\ntrigger = 'gh'\ntitle = 'GitHub'\nurl = 'https://github.com/search?q={q}'\n")
	got, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if got.Browser != "zen" || got.SearchURL != "https://duckduckgo.com/?q={q}" || len(got.Quicklinks) != 1 || got.Quicklinks[0].Trigger != "gh" {
		t.Fatalf("got %+v", got)
	}
}

func TestTheClipboardHistoryIsOffByDefaultAndItsSettingsRoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.toml")
	fresh, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if fresh.Clipboard.Enabled || len(fresh.Clipboard.Ignore) != 0 {
		t.Fatalf("a fresh install records what is copied: %+v", fresh.Clipboard)
	}
	fresh.Clipboard = Clipboard{Enabled: true, Ignore: []string{"MyVault.exe", "Notepad"}}
	if err := fresh.Save(path); err != nil {
		t.Fatal(err)
	}
	again, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if !again.Clipboard.Enabled || strings.Join(again.Clipboard.Ignore, "|") != "MyVault.exe|Notepad" {
		t.Fatalf("after a round trip: %+v", again.Clipboard)
	}
	cloned := again.Clone()
	cloned.Clipboard.Ignore[0] = "changed"
	if again.Clipboard.Ignore[0] != "MyVault.exe" {
		t.Fatal("Clone shares the ignore list")
	}
}

func TestSnippetsRoundTripWithManyLinesAndNoneByDefault(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.toml")
	fresh, err := Load(path)
	if err != nil || len(fresh.Snippets) != 0 {
		t.Fatalf("a fresh install has snippets: %v, %v", fresh.Snippets, err)
	}
	fresh.Snippets = []Snippet{
		{Name: "Home address", Keyword: "addr", Text: "12 Example Street\nSpringfield\n\t\"quoted\""},
		{Name: "Signature", Text: "Best,\nA"},
	}
	if err := fresh.Save(path); err != nil {
		t.Fatal(err)
	}
	again, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(again.Snippets) != 2 || again.Snippets[0] != fresh.Snippets[0] || again.Snippets[1] != fresh.Snippets[1] {
		t.Fatalf("after a round trip: %+v", again.Snippets)
	}
	cloned := again.Clone()
	cloned.Snippets[0].Name = "changed"
	if again.Snippets[0].Name != "Home address" {
		t.Fatal("Clone shares the snippets")
	}
}
