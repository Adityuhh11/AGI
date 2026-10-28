// Package config reads the launcher's settings file, %APPDATA%\Omni\config.toml.
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
	// UI holds the small choices about how the bar feels.
	UI UI `toml:"ui"`
	// Timer is the pomodoro timer (internal/timer).
	Timer Timer `toml:"timer"`
	// Clipboard is the searchable history of what is copied (internal/clipboard).
	Clipboard Clipboard `toml:"clipboard"`
	// Chat is the local language model and its chat window (internal/llm, internal/chat).
	Chat Chat `toml:"chat"`
	// Snippets are texts kept to be pasted from the bar (internal/snippets).
	Snippets []Snippet `toml:"snippet,omitempty"`
	// Sidecar is the retired [sidecar] table (where the engine lived in the repository). Load reads
	// it so old files still open and then clears it, so it is never written back.
	Sidecar *retiredSidecar `toml:"sidecar,omitempty"`
}

// UI holds the choices about how the bar feels.
type UI struct {
	// SelectionEffect lifts the selected result and dims the others while the arrow keys move through the list.
	SelectionEffect bool `toml:"selection_effect"`
}

// Timer holds the lengths of a pomodoro and how it goes on.
type Timer struct {
	FocusMinutes      int `toml:"focus_minutes"`
	ShortBreakMinutes int `toml:"short_break_minutes"`
	LongBreakMinutes  int `toml:"long_break_minutes"`
	// Rounds is how many focus periods make a cycle; the last is followed by the long break.
	Rounds int `toml:"rounds"`
	// AutoNext starts the next period by itself; off, a finished period waits to be started.
	AutoNext bool `toml:"auto_next"`
	// PresetMinutes are the quick countdowns the bar offers when only "timer" is typed. Any other length is typed after it.
	PresetMinutes []int `toml:"preset_minutes"`
}

// maxPresets keeps the bar's list of quick timers short enough to read at a glance.
const maxPresets = 8

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

// Chat holds the settings of Omni AI (docs/superpowers/specs/2026-10-28-ai-chat-design.md).
type Chat struct {
	// Enabled switches Omni AI on: the model is downloaded and `ask` opens the chat. Off until the person turns it on.
	Enabled bool `toml:"enabled"`
	// Hotkey, if set, opens the chat window from anywhere ("Ctrl+Alt+A"). Empty means there is none.
	Hotkey string `toml:"hotkey,omitempty"`
	// Load is when the model is put in memory: one of ChatLoads.
	Load string `toml:"load"`
	// UnloadAfterMinutes is how long the model stays in memory after the last answer; 0 means it is never unloaded.
	UnloadAfterMinutes int `toml:"unload_after_minutes"`
	// ContextTokens is the size of the model's context window, one of ChatContexts.
	ContextTokens int `toml:"context_tokens"`
	// KeepHistory saves conversations as files; off, a conversation is forgotten when its window closes.
	KeepHistory bool `toml:"keep_history"`
	// HistoryDays deletes saved conversations not touched for this long; 0 keeps them for good.
	HistoryDays int `toml:"history_days"`
	// Thinking lets the model reason before it answers: slower, and sometimes better.
	Thinking bool `toml:"thinking"`
	// OnTop keeps the chat window above other windows.
	OnTop bool `toml:"on_top"`
}

// ChatLoads are the allowed values of Chat.Load: the model starts when it is first asked, or when Omni starts.
var ChatLoads = []string{"when_asked", "at_start"}

// ChatContexts are the allowed sizes of the model's context window, in tokens.
var ChatContexts = []int{4096, 8192, 16384, 32768}

// ChatUnloadMinutes are the allowed times before an idle model is unloaded; 0 is never.
var ChatUnloadMinutes = []int{1, 5, 10, 30, 60, 0}

// maxHistoryDays is the longest the history may be kept by days (a year); longer is "never" (0).
const maxHistoryDays = 365

// Search holds the settings of file search, the memory engine.
type Search struct {
	// Enabled turns file search on: the engine runs and the folders in Roots are indexed.
	Enabled bool `toml:"enabled"`
	// InLauncher puts files among the results of the default search. Off, the default search is only a launcher (apps,
	// the web, clipboard, snippets) and files are searched in the Files mode, reached with Tab or "file:". A file from
	// before the key existed gets it on when file search is on, so nobody loses what they had.
	InLauncher bool `toml:"in_launcher"`
	// GroupResults shows file results under a heading per kind of file, with the single best match first.
	GroupResults bool `toml:"group_results"`
	// HoldIndexing keeps the folders from being indexed until the person presses "Start indexing": set when file search
	// is first downloaded, so the choices below are made before any picture is embedded. A file without the key (every
	// install made before it existed) indexes as it always did.
	HoldIndexing bool `toml:"hold_indexing,omitempty"`
	// DataDir is the folder the search engine, its models and the index are kept under (in engine and memory inside
	// it), chosen before file search is first downloaded. Empty means %LOCALAPPDATA%\Omni.
	DataDir string `toml:"data_dir,omitempty"`
	// PauseOnBattery holds indexing while the computer runs on its battery and goes on when it is plugged in. On by
	// default: a big sync would drain a laptop.
	PauseOnBattery bool `toml:"pause_on_battery"`
	// Pace is how much of the time indexing may keep the GPU busy: one of Paces.
	Pace string `toml:"pace"`
	// PDFPages, Photos, Videos and Audio say what each kind of file is searched by (docs/superpowers/specs/
	// 2026-10-20-indexing-choices-design.md, 2026-10-28-audio-search-design.md): one of PDFPageModes, PhotoModes,
	// VideoModes and AudioModes, the first the default. Audio covers audio files and the soundtrack of videos.
	PDFPages string `toml:"pdf_pages"`
	Photos   string `toml:"photos"`
	Videos   string `toml:"videos"`
	Audio    string `toml:"audio"`
}

// Paces are the allowed values of Search.Pace, from the most to the least demanding.
var Paces = []string{"fast", "balanced", "gentle"}

// The allowed values of the indexing choices, the default first. PDF pages: a page's picture is searched too when
// it looks visual (auto), always (all) or never (text). Photos and videos: by what they show (content) or by name.
// Audio: by what is said and played (content) or by name.
var (
	PDFPageModes = []string{"auto", "all", "text"}
	PhotoModes   = []string{"content", "name"}
	VideoModes   = []string{"name", "content"}
	AudioModes   = []string{"name", "content"}
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
	return filepath.Join(dir, "Omni", "config.toml"), nil
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
	if cfg.Search.Enabled && !bytes.Contains(data, []byte("in_launcher")) {
		cfg.Search.InLauncher = true
	}
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
	if c.Search.DataDir != "" && (!filepath.IsAbs(c.Search.DataDir) || strings.HasPrefix(filepath.VolumeName(c.Search.DataDir), `\\`)) {
		return fmt.Errorf("data_dir %q must be an absolute path on a local drive", c.Search.DataDir)
	}
	if err := c.validateWeb(); err != nil {
		return err
	}
	if err := c.validateSnippets(); err != nil {
		return err
	}
	if err := c.validateChat(); err != nil {
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
		{"audio", c.Search.Audio, AudioModes},
	} {
		if !slices.Contains(choice.allowed, choice.value) {
			return fmt.Errorf("%s %q must be one of %s", choice.name, choice.value, strings.Join(choice.allowed, ", "))
		}
	}
	for _, limit := range []struct {
		name          string
		value, lo, hi int
	}{
		{"timer focus_minutes", c.Timer.FocusMinutes, 1, 180},
		{"timer short_break_minutes", c.Timer.ShortBreakMinutes, 1, 60},
		{"timer long_break_minutes", c.Timer.LongBreakMinutes, 1, 120},
		{"timer rounds", c.Timer.Rounds, 1, 12},
	} {
		if limit.value < limit.lo || limit.value > limit.hi {
			return fmt.Errorf("%s %d must be between %d and %d", limit.name, limit.value, limit.lo, limit.hi)
		}
	}
	if len(c.Timer.PresetMinutes) > maxPresets {
		return fmt.Errorf("timer preset_minutes has %d lengths, at most %d", len(c.Timer.PresetMinutes), maxPresets)
	}
	for _, minutes := range c.Timer.PresetMinutes {
		if minutes < 1 || minutes > 24*60 {
			return fmt.Errorf("timer preset_minutes %d must be between 1 and %d", minutes, 24*60)
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

// validateChat checks the Omni AI settings.
func (c Config) validateChat() error {
	chat := c.Chat
	same := func(a, b string) bool {
		return a != "" && strings.EqualFold(strings.ReplaceAll(a, " ", ""), strings.ReplaceAll(b, " ", ""))
	}
	switch {
	case same(chat.Hotkey, c.Hotkey):
		return fmt.Errorf("the chat hotkey %q is the launcher's own hotkey", chat.Hotkey)
	case same(chat.Hotkey, c.Clipboard.Hotkey):
		return fmt.Errorf("the chat hotkey %q is the clipboard's hotkey", chat.Hotkey)
	case !slices.Contains(ChatLoads, chat.Load):
		return fmt.Errorf("chat load %q must be one of %s", chat.Load, strings.Join(ChatLoads, ", "))
	case !slices.Contains(ChatUnloadMinutes, chat.UnloadAfterMinutes):
		return fmt.Errorf("chat unload_after_minutes %d must be one of %v", chat.UnloadAfterMinutes, ChatUnloadMinutes)
	case !slices.Contains(ChatContexts, chat.ContextTokens):
		return fmt.Errorf("chat context_tokens %d must be one of %v", chat.ContextTokens, ChatContexts)
	case chat.HistoryDays < 0 || chat.HistoryDays > maxHistoryDays:
		return fmt.Errorf("chat history_days %d must be between 0 and %d", chat.HistoryDays, maxHistoryDays)
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
