package config

import (
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"strings"
	"testing"

	"omni/internal/testpath"
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
	got, err := Path()
	if err != nil {
		t.Fatal(err)
	}
	dir, err := os.UserConfigDir()
	if err != nil {
		t.Fatal(err)
	}
	if want := filepath.Join(dir, "Omni", "config.toml"); got != want {
		t.Fatalf("Path() = %q, want %q", got, want)
	}
}

func TestLoadCreatesTheFileWithDefaultsWhenMissing(t *testing.T) {
	path := filepath.Join(t.TempDir(), "Omni", "config.toml")
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
	writeFile(t, path, "roots = ['"+testpath.P(`C:\Docs`)+"', '"+testpath.P(`D:\Photos`)+"']\n")
	got, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Roots) != 2 || got.Roots[0] != testpath.P(`C:\Docs`) || got.Roots[1] != testpath.P(`D:\Photos`) {
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
		{"wrong type", "roots = '" + testpath.P(`C:\Docs`) + "'\n", "parse"},
		{"empty hotkey", "hotkey = ''\n", "hotkey must not be empty"},
		{"relative root", "roots = ['docs']\n", "must be an absolute path"},
		{"duplicate root", "roots = ['" + testpath.P(`C:\Docs`) + "', '" + testpath.P(`c:\docs\`) + "']\n", "repeats"},
		{"relative exclude", "roots = ['" + testpath.P(`C:\Docs`) + "']\nexclude = ['vendor']\n", "must be an absolute path"},
		{"exclude outside every root", "roots = ['" + testpath.P(`C:\Docs`) + "']\nexclude = ['" + testpath.P(`D:\Other`) + "']\n", "not inside any root"},
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
		{"exclude that is a root", "roots = ['" + testpath.P(`C:\Docs`) + "']\nexclude = ['" + testpath.P(`C:\Docs`) + "']\n", "not inside any root"},
		{"unknown pdf mode", "[search]\npace = 'fast'\npdf_pages = 'some'\n", "pdf_pages"},
		{"unknown photo mode", "[search]\npace = 'fast'\nphotos = 'pixels'\n", "photos"},
		{"unknown video mode", "[search]\npace = 'fast'\nvideos = 'off'\n", "videos"},
		{"unknown audio mode", "[search]\npace = 'fast'\naudio = 'on'\n", "audio"},
		{"unknown theme", "[ui]\ntheme = 'neon'\n", "ui theme"},
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
		{testpath.P(`C:\Users\me\Docs`), testpath.P(`C:\Users\me\Docs`), true},
		{testpath.P(`C:\Users\me\Docs`), testpath.P(`c:\users\ME\docs`), true},
		{testpath.P(`C:\Users\me\Docs`), testpath.P(`C:\Users\me\Docs\`), true},
		{testpath.P(`C:\Users\me\Docs`), testpath.P(`C:\Users\me\Docs\.`), true},
		{testpath.P(`C:\Users\me\Docs`), testpath.P(`C:\Users\me\Photos`), false},
		{testpath.P(`C:\Users\me\Docs`), testpath.P(`D:\Users\me\Docs`), false},
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
	writeFile(t, path, "hotkey = 'Ctrl+Alt+K'\n\n[sidecar]\ndir = '"+testpath.P(`C:\\dev\\Omni\\sidecar`)+"'\n")
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

func TestFilesStayInTheLauncherForOlderFilesButNotForNewInstalls(t *testing.T) {
	tests := []struct {
		name, body string
		want       bool
	}{
		{"file search on, from before the key", "[search]\nenabled = true\n", true},
		{"file search off", "[search]\nenabled = false\n", false},
		{"chosen off", "[search]\nenabled = true\nin_launcher = false\n", false},
		{"chosen on", "[search]\nin_launcher = true\n", true},
	}
	for _, tt := range tests {
		path := filepath.Join(t.TempDir(), "config.toml")
		writeFile(t, path, tt.body)
		got, err := Load(path)
		if err != nil {
			t.Fatal(err)
		}
		if got.Search.InLauncher != tt.want {
			t.Errorf("%s: InLauncher = %v, want %v", tt.name, got.Search.InLauncher, tt.want)
		}
	}
	if Default().Search.InLauncher {
		t.Error("a fresh install must start as a launcher only")
	}
}

func TestHoldIndexingIsOffForOlderFilesAndWrittenOnlyWhenOn(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.toml")
	writeFile(t, path, "[search]\nenabled = true\n")
	got, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if got.Search.HoldIndexing {
		t.Fatal("a file from before the hold must keep indexing")
	}
	if err := got.Save(path); err != nil {
		t.Fatal(err)
	}
	if saved, _ := os.ReadFile(path); strings.Contains(string(saved), "hold_indexing") {
		t.Fatalf("an off hold was written:\n%s", saved)
	}
	got.Search.HoldIndexing = true
	if err := got.Save(path); err != nil {
		t.Fatal(err)
	}
	if again, err := Load(path); err != nil || !again.Search.HoldIndexing {
		t.Fatalf("the hold did not survive a save: %+v, %v", again.Search, err)
	}
}

func TestLoadReadsTheIndexingChoices(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.toml")
	writeFile(t, path, "[search]\npace = 'fast'\npdf_pages = 'text'\nphotos = 'name'\nvideos = 'content'\naudio = 'content'\n")
	got, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if got.Search.PDFPages != "text" || got.Search.Photos != "name" || got.Search.Videos != "content" || got.Search.Audio != "content" {
		t.Fatalf("search = %+v", got.Search)
	}
}

func TestExcludedUnderGivesPathsBelowTheRootInTheEnginesForm(t *testing.T) {
	exclude := []string{testpath.P(`C:\Docs\Vendor\Skills`), testpath.P(`C:\Docs\other`), testpath.P(`C:\Docs`), testpath.P(`C:\Documents\x`), testpath.P(`D:\Docs\Vendor`)}
	got := ExcludedUnder(testpath.P(`C:\Docs`), exclude)
	if want := "Vendor/Skills|other"; strings.Join(got, "|") != want {
		t.Fatalf("ExcludedUnder = %v, want %s", got, want)
	}
}

func TestLoadReadsExcludedFolders(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.toml")
	writeFile(t, path, "roots = ['"+testpath.P(`C:\Docs`)+"']\nexclude = ['"+testpath.P(`C:\Docs\vendor`)+"']\n")
	got, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Exclude) != 1 || got.Exclude[0] != testpath.P(`C:\Docs\vendor`) {
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

func TestDataDirMustBeAnAbsoluteLocalPath(t *testing.T) {
	cfg := Default()
	for dir, ok := range map[string]bool{"": true, testpath.P(`D:\Files\Omni`): true, testpath.P(`relative\Omni`): false} {
		cfg.Search.DataDir = dir
		if err := cfg.Validate(); (err == nil) != ok {
			t.Errorf("data_dir %q: err = %v, want ok=%v", dir, err, ok)
		}
	}
}

func TestNewSettingsDefaultOnAndAnOlderFileKeepsThem(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.toml")
	writeFile(t, path, `hotkey = 'Alt+Shift+Space'
[search]
pace = 'fast'
`)
	got, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if !got.UI.SelectionEffect || !got.Search.GroupResults {
		t.Errorf("the feel settings must be on when the file lacks them: %+v %+v", got.UI, got.Search)
	}
	if !reflect.DeepEqual(got.Timer, Timer{FocusMinutes: 25, ShortBreakMinutes: 5, LongBreakMinutes: 15, Rounds: 4, PresetMinutes: []int{5, 10, 25, 45}}) {
		t.Errorf("timer = %+v, want the pomodoro defaults", got.Timer)
	}
}

func TestTheNewSettingsCanBeTurnedOffAndSurviveASave(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.toml")
	cfg := Default()
	cfg.UI.SelectionEffect, cfg.Search.GroupResults, cfg.Timer.AutoNext, cfg.Timer.FocusMinutes = false, false, true, 50
	if err := cfg.Save(path); err != nil {
		t.Fatal(err)
	}
	got, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if got.UI.SelectionEffect || got.Search.GroupResults || !got.Timer.AutoNext || got.Timer.FocusMinutes != 50 {
		t.Fatalf("lost on the way: %+v %+v %+v", got.UI, got.Search, got.Timer)
	}
}

func TestTimerLengthsMustBeSensible(t *testing.T) {
	tests := []struct {
		name   string
		change func(*Timer)
	}{
		{"no focus time", func(t *Timer) { t.FocusMinutes = 0 }},
		{"a day of focus", func(t *Timer) { t.FocusMinutes = 24 * 60 }},
		{"no break", func(t *Timer) { t.ShortBreakMinutes = 0 }},
		{"a negative long break", func(t *Timer) { t.LongBreakMinutes = -5 }},
		{"no rounds", func(t *Timer) { t.Rounds = 0 }},
		{"too many rounds", func(t *Timer) { t.Rounds = 13 }},
		{"a zero-minute preset", func(t *Timer) { t.PresetMinutes = []int{5, 0} }},
		{"a preset longer than a day", func(t *Timer) { t.PresetMinutes = []int{24*60 + 1} }},
		{"nine presets", func(t *Timer) { t.PresetMinutes = []int{1, 2, 3, 4, 5, 6, 7, 8, 9} }},
	}
	for _, tt := range tests {
		cfg := Default()
		tt.change(&cfg.Timer)
		if err := cfg.Validate(); err == nil {
			t.Errorf("%s was accepted", tt.name)
		}
	}
}

func TestAConfigWithoutTheBatteryKeyWaitsForPower(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.toml")
	if err := os.WriteFile(path, []byte("[search]\nenabled = true\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	got, err := Load(path)
	if err != nil || !got.Search.PauseOnBattery {
		t.Fatalf("Load: %v, PauseOnBattery %v; want it on unless the file says off", err, got.Search.PauseOnBattery)
	}
}

func TestOmniAIIsOffByDefaultLoadsOnDemandAndKeepsHistory(t *testing.T) {
	got := Default().Chat
	if got.Enabled || got.Hotkey != "" || got.Thinking {
		t.Fatalf("chat must start off, with no hotkey and no thinking: %+v", got)
	}
	if got.Load != "when_asked" || got.UnloadAfterMinutes != 10 || got.ContextTokens != 8192 || !got.KeepHistory || got.HistoryDays != 90 || !got.OnTop {
		t.Fatalf("defaults = %+v", got)
	}
}

func TestChatSettingsRoundTripAndAreValidated(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.toml")
	cfg := Default()
	cfg.Chat = Chat{Enabled: true, Hotkey: "Ctrl+Alt+A", Load: "at_start", UnloadAfterMinutes: 0, ContextTokens: 16384, KeepHistory: false, HistoryDays: 0, Thinking: true, OnTop: false}
	if err := cfg.Save(path); err != nil {
		t.Fatal(err)
	}
	back, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if back.Chat != cfg.Chat {
		t.Fatalf("round trip = %+v, want %+v", back.Chat, cfg.Chat)
	}

	for name, change := range map[string]func(*Config){
		"load":             func(c *Config) { c.Chat.Load = "always" },
		"unload minutes":   func(c *Config) { c.Chat.UnloadAfterMinutes = 7 },
		"context":          func(c *Config) { c.Chat.ContextTokens = 5000 },
		"history days":     func(c *Config) { c.Chat.HistoryDays = -1 },
		"history too long": func(c *Config) { c.Chat.HistoryDays = 366 },
		"launcher hotkey":  func(c *Config) { c.Chat.Hotkey = "alt+shift+space" },
		"clipboard hotkey": func(c *Config) { c.Clipboard.Hotkey = "Ctrl+Alt+A"; c.Chat.Hotkey = "Ctrl+Alt+A" },
	} {
		bad := Default()
		change(&bad)
		if err := bad.Validate(); err == nil {
			t.Errorf("%s: an invalid chat setting was accepted", name)
		}
	}
}

func TestNoTwoHotkeysMayShareACombination(t *testing.T) {
	for name, change := range map[string]func(*Config){
		"notes and launcher":     func(c *Config) { c.Notes.Hotkey = "alt + shift + space" },
		"notes and clipboard":    func(c *Config) { c.Clipboard.Hotkey, c.Notes.Hotkey = "Ctrl+Alt+N", "ctrl+alt+n" },
		"notes and chat":         func(c *Config) { c.Chat.Hotkey, c.Notes.Hotkey = "Ctrl+Alt+A", "Ctrl+Alt+A" },
		"clipboard and launcher": func(c *Config) { c.Clipboard.Hotkey = c.Hotkey },
	} {
		bad := Default()
		change(&bad)
		if err := bad.Validate(); err == nil || !strings.Contains(err.Error(), "hotkey") {
			t.Errorf("%s: err = %v, want a refusal naming the hotkey", name, err)
		}
	}
	ok := Default()
	ok.Clipboard.Hotkey, ok.Chat.Hotkey, ok.Notes.Hotkey = "Ctrl+Shift+V", "Ctrl+Alt+A", "Ctrl+Alt+N"
	if err := ok.Validate(); err != nil {
		t.Errorf("four different hotkeys were refused: %v", err)
	}
}
