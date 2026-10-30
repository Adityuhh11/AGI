package config

// DefaultHotkey is rare enough not to collide with common apps (Alt+Space, Win+Space and
// Ctrl+Alt+Space were all taken on the development machine).
const DefaultHotkey = "Alt+Shift+Space"

// Default is the configuration of a fresh install: no folders are indexed and file search is off
// until the user turns it on.
func Default() Config {
	return Config{
		Hotkey: DefaultHotkey,
		Search: Search{
			Pace: "balanced", PDFPages: PDFPageModes[0], Photos: PhotoModes[0], Videos: VideoModes[0], Audio: AudioModes[0], GroupResults: true, PauseOnBattery: true,
		},
		UI:    UI{SelectionEffect: true, Theme: Themes[0]},
		Chat:  Chat{Load: ChatLoads[0], UnloadAfterMinutes: 10, ContextTokens: 8192, KeepHistory: true, HistoryDays: 90, OnTop: true},
		Timer: Timer{FocusMinutes: 25, ShortBreakMinutes: 5, LongBreakMinutes: 15, Rounds: 4, PresetMinutes: []int{5, 10, 25, 45}},
	}
}
