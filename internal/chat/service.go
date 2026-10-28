package chat

import (
	"context"
	"errors"
	"strings"
	"sync"
	"time"

	"omni/internal/config"
	"omni/internal/llm"
)

// Events the page listens to (Wails events, the names are the page's).
const (
	EventDelta  = "chat:delta"  // Delta: more of the answer
	EventStatus = "chat:status" // Status: what the model is doing before and during the answer
	EventDone   = "chat:done"   // Done: the answer is finished, stopped or failed
	EventModel  = "chat:model"  // Info: the model's state or a setting changed
	EventAsk    = "chat:ask"    // nothing: a question is waiting for the window (TakePending)
	EventReset  = "chat:reset"  // View: the open conversation was replaced (history cleared from Settings)
)

// Delta is a piece of an answer.
type Delta struct {
	Text     string `json:"text,omitempty"`
	Thinking string `json:"thinking,omitempty"`
}

// Status says what the model is doing: loading, reading, thinking or answering.
type Status struct {
	Phase string `json:"phase"`
	Text  string `json:"text"`
}

// Done ends an answer. Error is advice for the person when it failed; Restore is the text of a question that was not sent, for the
// box to take back.
type Done struct {
	View    View   `json:"view"`
	Error   string `json:"error,omitempty"`
	Restore string `json:"restore,omitempty"`
}

// View is the conversation as the window shows it.
type View struct {
	ID       string    `json:"id"`
	Title    string    `json:"title"`
	Messages []Message `json:"messages"`
	// MemoryStart is the first message the model is shown; the ones before it have left its memory.
	MemoryStart int `json:"memory_start"`
	// Used and Room are tokens of the model's context in use and in all.
	Used int `json:"used"`
	Room int `json:"room"`
	// Unsaved says this chat is not being kept, by the person's choice or the setting.
	Unsaved bool `json:"unsaved"`
	Busy    bool `json:"busy"`
}

// Info is the model and the settings the window's header shows.
type Info struct {
	Enabled     bool   `json:"enabled"`
	Installed   bool   `json:"installed"`
	Thinking    bool   `json:"thinking"`
	KeepHistory bool   `json:"keep_history"`
	State       string `json:"state"`
	Message     string `json:"message"`
	Device      string `json:"device"`
	UnloadsAt   int64  `json:"unloads_at"` // milliseconds since 1970; 0 when it does not unload
}

// Model is the language model as the service uses it; *llm.Server satisfies it.
type Model interface {
	Ensure(ctx context.Context) (llm.Endpoint, error)
	// Hold keeps the model loaded until the returned function is called.
	Hold() func()
	// UseProcessor moves the model off the graphics card.
	UseProcessor()
	Status() llm.Status
}

// Options are the parts of the program the service needs; main supplies them.
type Options struct {
	// Store keeps conversations; nil keeps none.
	Store *Store
	Model Model
	// Settings returns the current [chat] settings; SetThinking saves the thinking switch; Installed says whether the model
	// and its server are on disk.
	Settings    func() config.Chat
	SetThinking func(on bool) error
	Installed   func() bool
	// Emit sends an event to the chat window; SetOnTop keeps it above other windows; OpenSettings opens Settings on a page.
	Emit         func(event string, data any)
	SetOnTop     func(on bool)
	OpenSettings func(section string)
	Now          func() time.Time
	Logf         func(format string, args ...any)
}

// Service is bound to the chat window; its exported methods become TypeScript functions. One conversation is open at a time,
// and one answer is generated at a time.
type Service struct {
	o Options

	mu        sync.Mutex
	conv      Conversation
	unsaved   bool // the person asked for this chat not to be kept
	busy      bool
	cancel    context.CancelFunc
	finished  chan struct{} // closed when the answer being generated ends
	pending   string
	systemTok int // the system prompt's tokens as last counted
}

// NewService returns the service.
func NewService(o Options) *Service {
	if o.Now == nil {
		o.Now = time.Now
	}
	if o.Logf == nil {
		o.Logf = func(string, ...any) {}
	}
	if o.Emit == nil {
		o.Emit = func(string, any) {}
	}
	return &Service{o: o, systemTok: 80}
}

// Housekeeping deletes the saved conversations that are older than the setting allows. main runs it once at start.
func (s *Service) Housekeeping() {
	if s.o.Store == nil {
		return
	}
	if err := s.o.Store.Expire(s.o.Settings().HistoryDays); err != nil {
		s.o.Logf("chat: delete old conversations: %v", err)
	}
}

// Info returns the model's state and the settings the header shows.
func (s *Service) Info() Info {
	cfg := s.o.Settings()
	st := s.o.Model.Status()
	info := Info{
		Enabled: cfg.Enabled, Installed: s.o.Installed(), Thinking: cfg.Thinking, KeepHistory: cfg.KeepHistory,
		State: string(st.State), Message: st.Message, Device: st.Device,
	}
	if !st.UnloadsAt.IsZero() {
		info.UnloadsAt = st.UnloadsAt.UnixMilli()
	}
	return info
}

// Refresh tells the window that the model or a setting changed.
func (s *Service) Refresh() { s.o.Emit(EventModel, s.Info()) }

// ModelChanged is the llm.Server's OnChange.
func (s *Service) ModelChanged(llm.Status) { s.Refresh() }

// Warm starts loading the model in the background, so it is ready by the time a question is typed. It does nothing when Omni
// AI is off.
func (s *Service) Warm() {
	if !s.o.Settings().Enabled || !s.o.Installed() {
		return
	}
	go func() {
		if _, err := s.o.Model.Ensure(context.Background()); err != nil {
			s.o.Logf("chat: warm up: %v", err)
		}
	}()
}

// SetThinking turns the model's reasoning before an answer on or off, from the next question on.
func (s *Service) SetThinking(on bool) error {
	if err := s.o.SetThinking(on); err != nil {
		return err
	}
	s.Refresh()
	return nil
}

// SetOnTop keeps the chat window above other windows (on) or lets other windows cover it (off).
func (s *Service) SetOnTop(on bool) {
	if s.o.SetOnTop != nil {
		s.o.SetOnTop(on)
	}
}

// OpenSettings opens Settings on the Omni AI page.
func (s *Service) OpenSettings() {
	if s.o.OpenSettings != nil {
		s.o.OpenSettings("omni-ai")
	}
}

// Ask is a question from outside the window (the bar): it waits for the window to take it (TakePending), and tells an open
// window that it is there.
func (s *Service) Ask(text string) {
	s.mu.Lock()
	s.pending = strings.TrimSpace(text)
	s.mu.Unlock()
	s.o.Emit(EventAsk, nil)
}

// TakePending returns the question that came from outside the window, if any, and forgets it.
func (s *Service) TakePending() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	text := s.pending
	s.pending = ""
	return text
}

// Current returns the conversation that is open.
func (s *Service) Current() View {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.viewLocked()
}

// New starts an empty conversation. An answer under way is stopped first.
func (s *Service) New() View {
	s.stopAndWait()
	s.mu.Lock()
	defer s.mu.Unlock()
	s.conv, s.unsaved = Conversation{}, false
	return s.viewLocked()
}

// Open opens a saved conversation. An answer under way is stopped first.
func (s *Service) Open(id string) (View, error) {
	if s.o.Store == nil {
		return View{}, errors.New("conversations are not kept: the chats folder could not be opened (see the log)")
	}
	conv, err := s.o.Store.Get(id)
	if err != nil {
		return View{}, err
	}
	s.stopAndWait()
	s.mu.Lock()
	defer s.mu.Unlock()
	s.conv, s.unsaved = conv, false
	return s.viewLocked(), nil
}

// List returns the saved conversations that contain every word of query, newest first.
func (s *Service) List(query string) ([]Summary, error) {
	if s.o.Store == nil {
		return nil, nil
	}
	return s.o.Store.Search(query, 0)
}

// Delete removes a saved conversation; if it is the open one, the window goes back to an empty one.
func (s *Service) Delete(id string) (View, error) {
	if s.o.Store != nil {
		if err := s.o.Store.Delete(id); err != nil {
			return View{}, err
		}
	}
	s.mu.Lock()
	open := s.conv.ID == id
	s.mu.Unlock()
	if open {
		return s.New(), nil
	}
	return s.Current(), nil
}

// reset starts an empty conversation and tells the window, for a change that did not come from the window.
func (s *Service) reset() View {
	view := s.New()
	s.o.Emit(EventReset, view)
	return view
}

// ClearHistory deletes every saved conversation and leaves an empty one open.
func (s *Service) ClearHistory() (View, error) {
	if s.o.Store != nil {
		if err := s.o.Store.Clear(); err != nil {
			return View{}, err
		}
	}
	return s.reset(), nil
}

// SetUnsaved says whether this chat is kept. Turning it on deletes the file the chat already has; turning it off saves it.
func (s *Service) SetUnsaved(on bool) (View, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.unsaved = on
	if s.o.Store != nil && s.conv.ID != "" && on {
		if err := s.o.Store.Delete(s.conv.ID); err != nil {
			return s.viewLocked(), err
		}
		s.conv.ID = ""
	}
	s.persistLocked()
	return s.viewLocked(), nil
}

// persistLocked saves the open conversation if conversations are kept and this one is not marked unsaved.
func (s *Service) persistLocked() {
	if s.o.Store == nil || s.unsaved || !s.o.Settings().KeepHistory || len(s.conv.Messages) == 0 {
		return
	}
	if err := s.o.Store.Save(&s.conv); err != nil {
		s.o.Logf("chat: save the conversation: %v", err)
	}
}

// viewLocked is the conversation as the window shows it, with where the model's memory begins.
func (s *Service) viewLocked() View {
	cfg := s.o.Settings()
	start, used, err := Fit(s.conv.Messages, s.systemTok, cfg.ContextTokens-Reserve(cfg.Thinking, cfg.ContextTokens))
	if err != nil { // the newest question alone does not fit; it is shown as the model's whole memory
		start = lastUser(s.conv.Messages)
	}
	return View{
		ID: s.conv.ID, Title: s.conv.Title, Messages: append([]Message(nil), s.conv.Messages...), MemoryStart: start,
		Used: used, Room: cfg.ContextTokens, Busy: s.busy,
		Unsaved: s.unsaved || !cfg.KeepHistory || s.o.Store == nil,
	}
}

func lastUser(messages []Message) int {
	for i := len(messages) - 1; i >= 0; i-- {
		if messages[i].Role == "user" {
			return i
		}
	}
	return 0
}

// Stop ends the answer being generated; what has arrived is kept.
func (s *Service) Stop() {
	s.mu.Lock()
	cancel := s.cancel
	s.mu.Unlock()
	if cancel != nil {
		cancel()
	}
}

// stopAndWait stops the answer and waits until its goroutine is done with the conversation.
func (s *Service) stopAndWait() {
	s.Stop()
	s.mu.Lock()
	finished := s.finished
	s.mu.Unlock()
	if finished != nil {
		<-finished
	}
}
