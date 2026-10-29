package settingsui

import (
	"strings"

	"omni/internal/chat"
	"omni/internal/config"
	"omni/internal/llm"
	"omni/internal/settings"
)

// The Omni AI page: turning the local model on, when it is in memory, and what is kept of the conversations.

// ChatSetup is the installer of the model as the page uses it; *llm.Setup satisfies it.
type ChatSetup interface {
	State() llm.InstallState
	Start() error
	Cancel()
	Remove() error
	Layout() llm.Layout
}

// ChatModel is the running model as the page uses it; *llm.Server satisfies it.
type ChatModel interface {
	Status() llm.Status
	Unload()
	// Rearm applies a changed idle time to a model that is waiting to be unloaded.
	Rearm()
}

// ChatHistory is the saved conversations as the page uses it; *chat.Store satisfies it.
type ChatHistory interface {
	Dir() string
	Search(words string, limit int) ([]chat.Summary, error)
}

// OmniAIOptions are the parts of the program the page needs; main supplies them.
type OmniAIOptions struct {
	Store   *settings.Store
	Setup   ChatSetup
	Model   ChatModel
	History ChatHistory // nil when the chats folder could not be opened
	// ClearHistory deletes every saved conversation and empties the open one.
	ClearHistory func() error
	// RebindHotkey moves the chat hotkey (empty turns it off) and fails if the combination cannot be used; HotkeyError says why
	// it could not be bound at start.
	RebindHotkey func(accelerator string) error
	HotkeyError  func() string
	OpenFolder   func(path string) error
}

// OmniAI is bound to the Omni AI page of the settings window. Methods that change something return the reason they were
// refused, or "" when it worked.
type OmniAI struct{ o OmniAIOptions }

// NewOmniAI returns the page's service.
func NewOmniAI(o OmniAIOptions) *OmniAI { return &OmniAI{o: o} }

// OmniAIView is the page: what is installed, how an install is going, where the model is, and the choices.
type OmniAIView struct {
	Enabled     bool
	Installed   bool
	Installing  bool
	NeedsUpdate bool
	// Step, Steps and StepName say where an install is; Done and Total are bytes of the download in progress.
	Step, Steps int
	StepName    string
	Done, Total int64
	Error       string // why the last install failed
	// Hosts, DownloadMB and LicenseURL are what the page tells a person before they agree to the download.
	Hosts      []string
	DownloadMB int
	LicenseURL string

	Load               string
	UnloadAfterMinutes int
	ContextTokens      int
	Thinking           bool
	OnTop              bool
	KeepHistory        bool
	HistoryDays        int
	Hotkey             string
	HotkeyError        string

	// ModelState is unloaded, loading, ready or failed; UnloadsAt is when an idle model goes, in milliseconds since 1970 (0 if it
	// does not).
	ModelState, ModelMessage, Device string
	UnloadsAt                        int64

	Conversations int
	HistoryFolder string
	// The values each choice offers, from the settings file's own lists.
	Loads                                         []string
	ContextChoices, UnloadChoices, HistoryChoices []int
}

// historyDayChoices are the times the page offers for keeping conversations; 0 is for good.
var historyDayChoices = []int{7, 30, 90, 365, 0}

// View returns the page; the window asks again every second or two.
func (p *OmniAI) View() OmniAIView {
	cfg := p.o.Store.Get().Chat
	st := p.o.Setup.State()
	model := p.o.Model.Status()
	v := OmniAIView{
		Enabled: cfg.Enabled, Installed: st.Installed, Installing: st.Installing, NeedsUpdate: st.NeedsUpdate,
		Step: st.Progress.Step, Steps: st.Progress.Steps, StepName: st.Progress.Name, Done: st.Progress.Done, Total: st.Progress.Total,
		Error: st.Error, Hosts: llm.Hosts, DownloadMB: int(llm.DownloadBytes() >> 20), LicenseURL: llm.LicenseURL,
		Load: cfg.Load, UnloadAfterMinutes: cfg.UnloadAfterMinutes, ContextTokens: cfg.ContextTokens, Thinking: cfg.Thinking, OnTop: cfg.OnTop,
		KeepHistory: cfg.KeepHistory, HistoryDays: cfg.HistoryDays, Hotkey: cfg.Hotkey, HotkeyError: p.o.HotkeyError(),
		ModelState: string(model.State), ModelMessage: model.Message, Device: model.Device,
		Loads: config.ChatLoads, ContextChoices: config.ChatContexts, UnloadChoices: config.ChatUnloadMinutes, HistoryChoices: historyDayChoices,
	}
	if !model.UnloadsAt.IsZero() {
		v.UnloadsAt = model.UnloadsAt.UnixMilli()
	}
	if p.o.History != nil {
		v.HistoryFolder = p.o.History.Dir()
		if list, err := p.o.History.Search("", 0); err == nil {
			v.Conversations = len(list)
		}
	}
	return v
}

// TurnOn switches Omni AI on. If the model is not downloaded yet (or is out of date) the download starts in the background, and
// Omni AI is switched on when it finishes (main does that).
func (p *OmniAI) TurnOn() string {
	st := p.o.Setup.State()
	if st.Installed && !st.NeedsUpdate {
		return reason(p.o.Store.Update(func(c *config.Config) { c.Chat.Enabled = true }))
	}
	return p.install()
}

// Update downloads what a newer Omni asks for in place of what is installed.
func (p *OmniAI) Update() string { return p.install() }

func (p *OmniAI) install() string {
	if p.o.Setup.State().Installing {
		return "Omni AI is already being installed"
	}
	p.o.Model.Unload() // its files are replaced
	if msg := reason(p.o.Store.Update(func(c *config.Config) { c.Chat.Enabled = false })); msg != "" {
		return msg
	}
	return reason(p.o.Setup.Start())
}

// CancelInstall stops a running download. What was downloaded is kept, so the next one starts where it ended.
func (p *OmniAI) CancelInstall() { p.o.Setup.Cancel() }

// TurnOff switches Omni AI off and frees the model's memory; its files stay, so turning it on again is instant.
func (p *OmniAI) TurnOff() string {
	msg := reason(p.o.Store.Update(func(c *config.Config) { c.Chat.Enabled = false }))
	p.o.Model.Unload()
	return msg
}

// Remove switches Omni AI off and deletes the model and its server (about a gigabyte). Conversations are kept.
func (p *OmniAI) Remove() string {
	if msg := p.TurnOff(); msg != "" {
		return msg
	}
	return reason(p.o.Setup.Remove())
}

// Unload frees the model's memory now.
func (p *OmniAI) Unload() { p.o.Model.Unload() }

// SetBehaviour sets when the model is loaded, how long it stays after the last answer, and how much it can read at once. A
// new context size takes effect when the model next starts, so a loaded model is unloaded.
func (p *OmniAI) SetBehaviour(load string, unloadAfterMinutes, contextTokens int) string {
	old := p.o.Store.Get().Chat
	msg := reason(p.o.Store.Update(func(c *config.Config) {
		c.Chat.Load, c.Chat.UnloadAfterMinutes, c.Chat.ContextTokens = load, unloadAfterMinutes, contextTokens
	}))
	if msg == "" && old.ContextTokens != contextTokens {
		p.o.Model.Unload()
	}
	return msg
}

// SetThinking lets the model reason before it answers, or not.
func (p *OmniAI) SetThinking(on bool) string {
	return reason(p.o.Store.Update(func(c *config.Config) { c.Chat.Thinking = on }))
}

// SetOnTop keeps the chat window above other windows, or lets them cover it.
func (p *OmniAI) SetOnTop(on bool) string {
	return reason(p.o.Store.Update(func(c *config.Config) { c.Chat.OnTop = on }))
}

// SetHistory sets whether conversations are saved and for how many days (0 is for good).
func (p *OmniAI) SetHistory(keep bool, days int) string {
	return reason(p.o.Store.Update(func(c *config.Config) { c.Chat.KeepHistory, c.Chat.HistoryDays = keep, days }))
}

// SetHotkey moves the chat hotkey. One that cannot be bound is refused and the working one stays.
func (p *OmniAI) SetHotkey(accelerator string) string {
	accelerator = strings.TrimSpace(accelerator)
	old := p.o.Store.Get().Chat.Hotkey
	if accelerator == old {
		return ""
	}
	// The settings reject a combination that is another hotkey's before it is registered, so a refused one is never bound.
	probe := p.o.Store.Get()
	probe.Chat.Hotkey = accelerator
	if err := probe.Validate(); err != nil {
		return reason(err)
	}
	if err := p.o.RebindHotkey(accelerator); err != nil {
		return err.Error()
	}
	err := p.o.Store.Update(func(c *config.Config) { c.Chat.Hotkey = accelerator })
	if err != nil {
		_ = p.o.RebindHotkey(old) // the page shows the refusal; the key that worked keeps working
	}
	return reason(err)
}

// ClearHistory deletes every saved conversation.
func (p *OmniAI) ClearHistory() string {
	if err := p.o.ClearHistory(); err != nil {
		return err.Error()
	}
	return ""
}

// OpenHistoryFolder shows the folder the conversations are saved in.
func (p *OmniAI) OpenHistoryFolder() string {
	if p.o.History == nil {
		return "the chats folder could not be opened (see the log)"
	}
	if err := p.o.OpenFolder(p.o.History.Dir()); err != nil {
		return err.Error()
	}
	return ""
}
