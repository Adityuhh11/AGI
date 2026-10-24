// Package config reads the launcher's settings file, %APPDATA%\AGI\config.toml.
//
// A missing file is created with the defaults, a file that sets only some keys keeps the
// defaults for the rest, and a key that does not exist is an error (a typo in a hotkey or a
// folder list should not be silently ignored). The one exception is the retired [sidecar]
// table, which older files still have: it is read, ignored and not written back.
package config

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/pelletier/go-toml/v2"
)

// Config is the content of config.toml.
type Config struct {
	// Hotkey opens the search bar from anywhere, e.g. "Alt+Shift+Space".
	Hotkey string `toml:"hotkey"`
	// Roots are the folders that get indexed; every file under them is searchable.
	Roots []string `toml:"roots"`
	// Exclude are folders inside a root to leave out, as absolute paths. What they held is dropped
	// from the index at the next sync.
	Exclude []string `toml:"exclude,omitempty"`
	// Browser is the browser web searches open in: a name Windows knows it by ("zen", "firefox",
	// "chrome") or the path of its program. Empty means the Windows default browser.
	Browser string `toml:"browser,omitempty"`
	// SearchURL is the address of a web search with {q} where the text goes. Without it, Firefox-based
	// browsers search with the engine set inside them and others use DuckDuckGo.
	SearchURL string `toml:"search_url,omitempty"`
	// Quicklinks turn a trigger word and some text into a web address: "gh rust lifetimes".
	Quicklinks []Quicklink `toml:"quicklink,omitempty"`
	// CheckUpdates lets the launcher ask GitHub once a day whether a newer release exists. Off unless
	// the user turns it on, because it is a request to a server the user did not name.
	CheckUpdates bool   `toml:"check_updates,omitempty"`
	Search       Search `toml:"search"`
	// Clipboard is the searchable history of what is copied (internal/clipboard).
	Clipboard Clipboard `toml:"clipboard"`
	// Snippets are texts kept to be pasted from the bar (internal/snippets).
	Snippets []Snippet `toml:"snippet,omitempty"`
	// Sidecar is the retired [sidecar] table (where the engine lived in the repository). Load reads
	// it so old files still open and then clears it, so it is never written back.
	Sidecar *retiredSidecar `toml:"sidecar,omitempty"`
}

// Quicklink is a web address with a place for what the user types after its trigger word.
type Quicklink struct {
	Trigger string `toml:"trigger"`
	Title   string `toml:"title"`
	// URL holds {q}, where the text typed after the trigger goes, escaped.
	URL string `toml:"url"`
}

// Snippet is a text kept to be pasted from the bar. {date}, {time} and {clipboard} in it are filled in when it is pasted.
type Snippet struct {
	Name string `toml:"name"`
	// Keyword, if set, is one word that shows the snippet in the bar when it is typed exactly.
	Keyword string `toml:"keyword,omitempty"`
	Text    string `toml:"text"`
}

// Clipboard holds the settings of the clipboard history.
type Clipboard struct {
	// Enabled switches the history on. It is off until the user turns it on, because it records what is copied.
	Enabled bool `toml:"enabled"`
	// Ignore are programs whose copies are never kept ("Notepad", "MyVault.exe"), beside the password
	// managers the history already leaves out.
	Ignore []string `toml:"ignore,omitempty"`
	// Hotkey, if set, opens the bar already listing what was copied ("Ctrl+Shift+V"). Empty means there is none.
	Hotkey string `toml:"hotkey,omitempty"`
}

// Search holds the settings of file search, the memory engine.
type Search struct {
	// Enabled turns file search on: the engine runs and the folders in Roots are indexed.
	Enabled bool `toml:"enabled"`
	// HoldIndexing keeps the folders from being indexed until the person presses "Start indexing": set when file search
	// is first downloaded, so the choices below are made before any picture is embedded. A file without the key (every
	// install made before it existed) indexes as it always did.
	HoldIndexing bool `toml:"hold_indexing,omitempty"`
	// Pace is how much of the time indexing may keep the GPU busy: one of Paces.
	Pace string `toml:"pace"`
	// PDFPages, Photos and Videos say what each kind of file is searched by (docs/superpowers/specs/
	// 2026-10-20-indexing-choices-design.md): one of PDFPageModes, PhotoModes and VideoModes, the first the default.
	PDFPages string `toml:"pdf_pages"`
	Photos   string `toml:"photos"`
	Videos   string `toml:"videos"`
}

// Paces are the allowed values of Search.Pace, from the most to the least demanding.
var Paces = []string{"fast", "balanced", "gentle"}

// The allowed values of the indexing choices, the default first. PDF pages: a page's picture is searched too when
// it looks visual (auto), always (all) or never (text). Photos and videos: by what they show (content) or by name.
var (
	PDFPageModes = []string{"auto", "all", "text"}
	PhotoModes   = []string{"content", "name"}
	VideoModes   = []string{"name", "content"}
)

type retiredSidecar struct {
	Dir string `toml:"dir"`
}

// Path is where the settings file lives for the current user.
func Path() (string, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", fmt.Errorf("locate the user config directory: %w", err)
	}
	return filepath.Join(dir, "AGI", "config.toml"), nil
}

// Load reads the settings file at path. If it does not exist it is created with the defaults.
func Load(path string) (Config, error) {
	cfg := Default()
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		if err := cfg.Save(path); err != nil {
			return Config{}, err
		}
		return cfg, nil
	}
	if err != nil {
		return Config{}, fmt.Errorf("read config: %w", err)
	}
	decoder := toml.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&cfg); err != nil {
		return Config{}, fmt.Errorf("parse %s: %w", path, err)
	}
	cfg.Sidecar = nil
	if err := cfg.Validate(); err != nil {
		return Config{}, fmt.Errorf("check %s: %w", path, err)
	}
	return cfg, nil
}

// Save writes the settings to path, creating the folder if needed. The file is written beside its
// final name and renamed over it, so a crash never leaves half a file.
func (c Config) Save(path string) error {
	body, err := toml.Marshal(c)
	if err != nil {
		return fmt.Errorf("encode config: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("create config folder: %w", err)
	}
	temp := path + ".tmp"
	if err := os.WriteFile(temp, body, 0o644); err != nil {
		return fmt.Errorf("write config: %w", err)
	}
	if err := os.Rename(temp, path); err != nil {
		_ = os.Remove(temp)
		return fmt.Errorf("replace config: %w", err)
	}
	return nil
}

// Clone returns a copy that shares no slices with c.
func (c Config) Clone() Config {
	c.Roots = slices.Clone(c.Roots)
	c.Exclude = slices.Clone(c.Exclude)
	c.Quicklinks = slices.Clone(c.Quicklinks)
	c.Clipboard.Ignore = slices.Clone(c.Clipboard.Ignore)
	c.Snippets = slices.Clone(c.Snippets)
	return c
}

// Validate reports the first thing wrong with the settings.
func (c Config) Validate() error {
	if c.Hotkey == "" {
		return fmt.Errorf("hotkey must not be empty")
	}
	seen := map[string]string{}
	for _, root := range c.Roots {
		if !filepath.IsAbs(root) {
			return fmt.Errorf("root %q must be an absolute path", root)
		}
		id := SourceID(root)
		if other, dup := seen[id]; dup {
			return fmt.Errorf("root %q repeats %q", root, other)
		}
		seen[id] = root
	}
	if err := c.validateWeb(); err != nil {
		return err
	}
	if err := c.validateSnippets(); err != nil {
		return err
	}
	if c.Clipboard.Hotkey != "" && strings.EqualFold(strings.ReplaceAll(c.Clipboard.Hotkey, " ", ""), strings.ReplaceAll(c.Hotkey, " ", "")) {
		return fmt.Errorf("the clipboard hotkey %q is the launcher's own hotkey", c.Clipboard.Hotkey)
	}
	for _, program := range c.Clipboard.Ignore {
		if strings.TrimSpace(program) == "" {
			return fmt.Errorf("clipboard ignore: a program name must not be empty")
		}
	}
	for _, choice := range []struct {
		name, value string
		allowed     []string
	}{
		{"search pace", c.Search.Pace, Paces},
		{"pdf_pages", c.Search.PDFPages, PDFPageModes},
		{"photos", c.Search.Photos, PhotoModes},
		{"videos", c.Search.Videos, VideoModes},
	} {
		if !slices.Contains(choice.allowed, choice.value) {
			return fmt.Errorf("%s %q must be one of %s", choice.name, choice.value, strings.Join(choice.allowed, ", "))
		}
	}
	for _, folder := range c.Exclude {
		if !filepath.IsAbs(folder) {
			return fmt.Errorf("excluded folder %q must be an absolute path", folder)
		}
		inside := false
		for _, root := range c.Roots {
			inside = inside || len(ExcludedUnder(root, []string{folder})) > 0
		}
		if !inside {
			return fmt.Errorf("excluded folder %q is not inside any root", folder)
		}
	}
	return nil
}

// validateWeb checks the browser settings: a typo in a quicklink should fail loudly, not give a dead row.
func (c Config) validateWeb() error {
	if c.SearchURL != "" {
		if err := checkTemplate(c.SearchURL); err != nil {
			return fmt.Errorf("search_url: %w", err)
		}
	}
	seen := map[string]bool{}
	for _, link := range c.Quicklinks {
		switch {
		case link.Trigger == "" || strings.ContainsAny(link.Trigger, " \t:"):
			return fmt.Errorf("quicklink trigger %q must be one word without a colon", link.Trigger)
		case link.Title == "":
			return fmt.Errorf("quicklink %q needs a title", link.Trigger)
		case seen[strings.ToLower(link.Trigger)]:
			return fmt.Errorf("quicklink trigger %q is used twice", link.Trigger)
		}
		seen[strings.ToLower(link.Trigger)] = true
		if err := checkTemplate(link.URL); err != nil {
			return fmt.Errorf("quicklink %q: %w", link.Trigger, err)
		}
	}
	return nil
}

// validateSnippets checks the snippets: a name and a text each, and keywords that are one word, used once, and
// not a quicklink's trigger (both are typed in the same bar).
func (c Config) validateSnippets() error {
	names, keywords := map[string]bool{}, map[string]bool{}
	for _, link := range c.Quicklinks {
		keywords[strings.ToLower(link.Trigger)] = true
	}
	for _, snippet := range c.Snippets {
		name := strings.ToLower(strings.TrimSpace(snippet.Name))
		switch {
		case name == "":
			return fmt.Errorf("a snippet needs a name")
		case names[name]:
			return fmt.Errorf("snippet name %q is used twice", snippet.Name)
		case snippet.Text == "":
			return fmt.Errorf("snippet %q needs a text", snippet.Name)
		}
		names[name] = true
		if snippet.Keyword == "" {
			continue
		}
		word := strings.ToLower(snippet.Keyword)
		switch {
		case strings.ContainsAny(snippet.Keyword, " \t:"):
			return fmt.Errorf("snippet %q: the keyword %q must be one word without a colon", snippet.Name, snippet.Keyword)
		case keywords[word]:
			return fmt.Errorf("snippet %q: the keyword %q is already a quicklink trigger or another snippet's keyword", snippet.Name, snippet.Keyword)
		}
		keywords[word] = true
	}
	return nil
}

// checkTemplate says what is wrong with a web address template, if anything.
func checkTemplate(template string) error {
	if !strings.HasPrefix(template, "https://") && !strings.HasPrefix(template, "http://") {
		return fmt.Errorf("%q must start with https:// or http://", template)
	}
	if strings.Count(template, "{q}") != 1 {
		return fmt.Errorf("%q must contain {q} once, where the text goes", template)
	}
	return nil
}
