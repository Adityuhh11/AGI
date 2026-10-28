package chat

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"sync"
	"testing"
	"time"

	"omni/internal/config"
	"omni/internal/llm"
)

type fakeModel struct {
	mu        sync.Mutex
	ep        llm.Endpoint
	ensureErr error
	ensures   int
	holds     int
	released  int
	processor int
	state     llm.State
}

func (m *fakeModel) Ensure(ctx context.Context) (llm.Endpoint, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.ensures++
	if m.ensureErr != nil {
		return llm.Endpoint{}, m.ensureErr
	}
	m.state = llm.Ready
	return m.ep, nil
}
func (m *fakeModel) Hold() func() {
	m.mu.Lock()
	m.holds++
	m.mu.Unlock()
	return func() { m.mu.Lock(); m.released++; m.mu.Unlock() }
}
func (m *fakeModel) UseProcessor() { m.mu.Lock(); m.processor++; m.mu.Unlock() }
func (m *fakeModel) Status() llm.Status {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.state == "" {
		return llm.Status{State: llm.Unloaded}
	}
	return llm.Status{State: m.state}
}

type events struct {
	mu    sync.Mutex
	all   []any
	names []string
	done  chan Done
}

func (e *events) emit(name string, data any) {
	e.mu.Lock()
	e.names = append(e.names, name)
	e.all = append(e.all, data)
	e.mu.Unlock()
	if d, ok := data.(Done); ok && name == EventDone {
		e.done <- d
	}
}

func (e *events) deltas() (text, thinking string) {
	e.mu.Lock()
	defer e.mu.Unlock()
	for _, d := range e.all {
		if delta, ok := d.(Delta); ok {
			text += delta.Text
			thinking += delta.Thinking
		}
	}
	return
}

func (e *events) phases() []string {
	e.mu.Lock()
	defer e.mu.Unlock()
	var out []string
	for _, d := range e.all {
		if st, ok := d.(Status); ok {
			out = append(out, st.Phase)
		}
	}
	return out
}

type rigT struct {
	svc   *Service
	llama *fakeLlama
	model *fakeModel
	ev    *events
	store *Store
	cfg   *config.Chat
}

func newRig(t *testing.T) *rigT {
	t.Helper()
	f := newFakeLlama(t)
	cfg := config.Default().Chat
	cfg.Enabled = true
	ev := &events{done: make(chan Done, 8)}
	model := &fakeModel{ep: f.endpoint()}
	store := newStore(t)
	r := &rigT{llama: f, model: model, ev: ev, store: store, cfg: &cfg}
	r.svc = NewService(Options{
		Store: store, Model: model,
		Settings:    func() config.Chat { return *r.cfg },
		SetThinking: func(on bool) error { r.cfg.Thinking = on; return nil },
		Installed:   func() bool { return true },
		Emit:        ev.emit,
		Now:         func() time.Time { return time.Date(2026, 10, 28, 9, 0, 0, 0, time.UTC) },
	})
	return r
}

func (r *rigT) await(t *testing.T) Done {
	t.Helper()
	select {
	case d := <-r.ev.done:
		return d
	case <-time.After(10 * time.Second):
		t.Fatal("the answer did not finish")
		return Done{}
	}
}

func TestSendAnswersStreamsSavesAndHoldsTheModelMeanwhile(t *testing.T) {
	r := newRig(t)
	r.llama.answer = "Paris is the capital"
	view, err := r.svc.Send("  What is the capital of France?  ")
	if err != nil || !view.Busy || len(view.Messages) != 1 || view.Messages[0].Text != "What is the capital of France?" {
		t.Fatalf("view after Send = %+v, err %v", view, err)
	}
	done := r.await(t)
	if done.Error != "" || done.View.Busy || len(done.View.Messages) != 2 {
		t.Fatalf("done = %+v", done)
	}
	answer := done.View.Messages[1]
	if answer.Role != "assistant" || answer.Text != "Paris is the capital" || answer.Stopped || answer.Tokens <= messageOverhead {
		t.Fatalf("answer = %+v", answer)
	}
	if text, _ := r.ev.deltas(); text != "Paris is the capital" {
		t.Fatalf("streamed text = %q", text)
	}
	if got := r.ev.phases(); len(got) < 3 || got[0] != "loading" || got[1] != "reading" || got[len(got)-1] != "answering" {
		t.Fatalf("phases = %v", got)
	}
	if r.model.holds != 1 || r.model.released != 1 {
		t.Fatalf("holds %d released %d; the model must be held for the answer and released after", r.model.holds, r.model.released)
	}
	saved, err := r.store.Get(done.View.ID)
	if err != nil || len(saved.Messages) != 2 || saved.Title != "What is the capital of France?" {
		t.Fatalf("saved = %+v, %v", saved, err)
	}
	req := r.llama.last()
	if req.Messages[0].Role != "system" || !strings.Contains(req.Messages[0].Content, "28 October 2026") || req.MaxTokens != 2048 {
		t.Fatalf("request = %+v", req)
	}
}

func TestSendRefusesWhenOffWhenBusyAndSendsNothingForBlankText(t *testing.T) {
	r := newRig(t)
	if view, err := r.svc.Send("   "); err != nil || len(view.Messages) != 0 {
		t.Fatalf("blank text: %+v, %v", view, err)
	}
	r.cfg.Enabled = false
	if _, err := r.svc.Send("hi"); err == nil {
		t.Fatal("a question was accepted while Omni AI is off")
	}
	r.cfg.Enabled = true
	r.llama.hold = make(chan struct{})
	if _, err := r.svc.Send("first"); err != nil {
		t.Fatal(err)
	}
	if _, err := r.svc.Send("second"); err == nil {
		t.Fatal("a second question was accepted while the first is being answered")
	}
	close(r.llama.hold)
	r.await(t)
}

func TestStopKeepsWhatHadArrivedAndMarksItStopped(t *testing.T) {
	r := newRig(t)
	r.llama.hold = make(chan struct{}) // the answer stops after its first word
	if _, err := r.svc.Send("tell me something"); err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(5 * time.Second)
	for {
		if text, _ := r.ev.deltas(); strings.Contains(text, "Hello") {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("no part of the answer arrived")
		}
		time.Sleep(10 * time.Millisecond)
	}
	r.svc.Stop()
	done := r.await(t)
	if len(done.View.Messages) != 2 || !done.View.Messages[1].Stopped || !strings.HasPrefix(done.View.Messages[1].Text, "Hello") || done.Error != "" {
		t.Fatalf("done = %+v", done)
	}
}

func TestThinkingIsSentPerRequestShownAndNeverSentBack(t *testing.T) {
	r := newRig(t)
	r.cfg.Thinking = true
	r.llama.reasons, r.llama.answer = "the user wants a number ", "forty two"
	if _, err := r.svc.Send("meaning of life?"); err != nil {
		t.Fatal(err)
	}
	done := r.await(t)
	answer := done.View.Messages[1]
	if answer.Thinking != "the user wants a number " || answer.Text != "forty two" {
		t.Fatalf("answer = %+v", answer)
	}
	if req := r.llama.last(); !req.Template["enable_thinking"] || req.MaxTokens != 4096 || len(req.Stop) != 0 {
		t.Fatalf("thinking request = %+v", req)
	}
	if _, err := r.svc.Send("and why?"); err != nil {
		t.Fatal(err)
	}
	r.await(t)
	for _, m := range r.llama.last().Messages {
		if strings.Contains(m.Content, "wants a number") {
			t.Fatal("the earlier reasoning was sent back to the model")
		}
	}
	if err := r.svc.SetThinking(false); err != nil || r.svc.Info().Thinking {
		t.Fatalf("SetThinking: %v, info %+v", err, r.svc.Info())
	}
}

func TestOldTurnsLeaveTheModelsMemoryWhenTheContextIsFull(t *testing.T) {
	r := newRig(t)
	r.cfg.ContextTokens = 4096 // room for a prompt: 4096 - 2048 reserve
	long := strings.TrimSpace(strings.Repeat("word ", 500))
	r.llama.answer = "ok"
	var last Done
	for range 6 {
		if _, err := r.svc.Send(long); err != nil {
			t.Fatal(err)
		}
		last = r.await(t)
	}
	if last.View.MemoryStart == 0 || last.View.MemoryStart%2 != 0 {
		t.Fatalf("memory starts at %d; old turns should have left, in whole turns", last.View.MemoryStart)
	}
	if last.View.Used > last.View.Room-2048 {
		t.Fatalf("used %d of %d leaves no room for the reply", last.View.Used, last.View.Room)
	}
	shown := 0
	for _, m := range r.llama.last().Messages {
		if m.Role == "user" {
			shown++
		}
	}
	if shown >= 6 || shown == 0 {
		t.Fatalf("the model was shown %d of 6 questions", shown)
	}
	if len(last.View.Messages) != 12 {
		t.Fatalf("the window must keep every message, has %d", len(last.View.Messages))
	}
}

func TestAQuestionTooLongForTheContextIsTakenBackToTheBox(t *testing.T) {
	r := newRig(t)
	r.cfg.ContextTokens = 4096
	huge := strings.Repeat("word ", 3000)
	if _, err := r.svc.Send(huge); err != nil {
		t.Fatal(err)
	}
	done := r.await(t)
	if done.Restore != strings.TrimSpace(huge) || len(done.View.Messages) != 0 || !strings.Contains(done.Error, "longer than the model can read") {
		t.Fatalf("done = restore %d chars, %d messages, error %q", len(done.Restore), len(done.View.Messages), done.Error)
	}
	if len(r.llama.requests) != 0 {
		t.Fatal("a question that cannot fit was sent anyway")
	}
}

func TestAServerThatDroppedIsStartedAgainOnceAndAnOutOfMemoryMovesToTheProcessor(t *testing.T) {
	r := newRig(t)
	r.llama.drop = 1 // the first request's connection is cut
	if _, err := r.svc.Send("hello"); err != nil {
		t.Fatal(err)
	}
	if done := r.await(t); done.Error != "" || len(done.View.Messages) != 2 {
		t.Fatalf("done = %+v", done)
	}
	if r.model.ensures != 2 || r.model.processor != 0 {
		t.Fatalf("ensures %d processor %d; want a second start and no move to the processor", r.model.ensures, r.model.processor)
	}

	r2 := newRig(t)
	r2.llama.status, r2.llama.statusMessage = http.StatusInternalServerError, "vk::Queue::submit: ErrorOutOfDeviceMemory"
	r2.llama.statusOnce = true
	if _, err := r2.svc.Send("hello"); err != nil {
		t.Fatal(err)
	}
	if done := r2.await(t); done.Error != "" || len(done.View.Messages) != 2 {
		t.Fatalf("done = %+v", done)
	}
	if r2.model.processor != 1 {
		t.Fatalf("processor = %d, want the model moved off the card once", r2.model.processor)
	}
}

func TestAFailureThatCannotBeHelpedIsExplainedAndTheQuestionStays(t *testing.T) {
	r := newRig(t)
	r.model.ensureErr = errors.New("the model server was not ready after 1m0s: boom")
	if _, err := r.svc.Send("hello"); err != nil {
		t.Fatal(err)
	}
	done := r.await(t)
	if !strings.Contains(done.Error, "did not start") || strings.Contains(done.Error, "boom") || len(done.View.Messages) != 1 {
		t.Fatalf("done = %+v", done)
	}
	// try again once the model works
	r.model.mu.Lock()
	r.model.ensureErr = nil
	r.model.mu.Unlock()
	if _, err := r.svc.Regenerate(); err != nil {
		t.Fatal(err)
	}
	if done := r.await(t); done.Error != "" || len(done.View.Messages) != 2 {
		t.Fatalf("after Regenerate: %+v", done)
	}
}

func TestRegenerateAnswersTheLastQuestionAgainInPlaceOfTheOldAnswer(t *testing.T) {
	r := newRig(t)
	r.llama.answer = "first answer"
	if _, err := r.svc.Send("q"); err != nil {
		t.Fatal(err)
	}
	r.await(t)
	r.llama.mu.Lock()
	r.llama.answer = "second answer"
	r.llama.mu.Unlock()
	if _, err := r.svc.Regenerate(); err != nil {
		t.Fatal(err)
	}
	done := r.await(t)
	if len(done.View.Messages) != 2 || done.View.Messages[1].Text != "second answer" {
		t.Fatalf("messages = %+v", done.View.Messages)
	}
}

func TestHistoryIsKeptListedOpenedDeletedAndClearedAndCanBeOff(t *testing.T) {
	r := newRig(t)
	for _, q := range []string{"alpha question", "beta question"} {
		if _, err := r.svc.AskNew(q); err != nil {
			t.Fatal(err)
		}
		r.await(t)
	}
	list, err := r.svc.List("")
	if err != nil || len(list) != 2 {
		t.Fatalf("list = %+v, %v", list, err)
	}
	if found, _ := r.svc.List("alpha"); len(found) != 1 {
		t.Fatalf("search = %+v", found)
	}
	view, err := r.svc.Open(list[1].ID)
	if err != nil || len(view.Messages) != 2 || view.ID != list[1].ID {
		t.Fatalf("Open = %+v, %v", view, err)
	}
	if view, err = r.svc.Delete(view.ID); err != nil || len(view.Messages) != 0 {
		t.Fatalf("deleting the open conversation should leave an empty one: %+v, %v", view, err)
	}
	if list, _ := r.svc.List(""); len(list) != 1 {
		t.Fatalf("after Delete: %+v", list)
	}
	if _, err := r.svc.ClearHistory(); err != nil {
		t.Fatal(err)
	}
	if list, _ := r.svc.List(""); len(list) != 0 {
		t.Fatalf("after ClearHistory: %+v", list)
	}

	r.cfg.KeepHistory = false
	if _, err := r.svc.AskNew("private"); err != nil {
		t.Fatal(err)
	}
	if done := r.await(t); !done.View.Unsaved {
		t.Fatal("a chat with history off must say it is not kept")
	}
	if list, _ := r.svc.List(""); len(list) != 0 {
		t.Fatalf("history was off but %+v was saved", list)
	}
}

func TestMarkingAChatUnsavedDeletesItsFileAndMarkingItSavedWritesItAgain(t *testing.T) {
	r := newRig(t)
	if _, err := r.svc.AskNew("remember this"); err != nil {
		t.Fatal(err)
	}
	r.await(t)
	if list, _ := r.svc.List(""); len(list) != 1 {
		t.Fatalf("list = %+v", list)
	}
	view, err := r.svc.SetUnsaved(true)
	if err != nil || !view.Unsaved {
		t.Fatalf("SetUnsaved(true) = %+v, %v", view, err)
	}
	if list, _ := r.svc.List(""); len(list) != 0 {
		t.Fatalf("the file stayed: %+v", list)
	}
	if _, err := r.svc.Send("and this"); err != nil {
		t.Fatal(err)
	}
	r.await(t)
	if list, _ := r.svc.List(""); len(list) != 0 {
		t.Fatalf("an unsaved chat was saved: %+v", list)
	}
	if view, _ = r.svc.SetUnsaved(false); view.Unsaved {
		t.Fatal("still unsaved")
	}
	if list, _ := r.svc.List(""); len(list) != 1 {
		t.Fatalf("turning it back on should save it: %+v", list)
	}
}

func TestAskWaitsForTheWindowAndIsTakenOnce(t *testing.T) {
	r := newRig(t)
	r.svc.Ask("  what is a mutex  ")
	if got := r.svc.TakePending(); got != "what is a mutex" {
		t.Fatalf("pending = %q", got)
	}
	if got := r.svc.TakePending(); got != "" {
		t.Fatalf("pending was not forgotten: %q", got)
	}
	r.ev.mu.Lock()
	defer r.ev.mu.Unlock()
	if len(r.ev.names) != 1 || r.ev.names[0] != EventAsk {
		t.Fatalf("events = %v", r.ev.names)
	}
}

func TestWarmStartsTheModelOnlyWhenOmniAIIsOn(t *testing.T) {
	r := newRig(t)
	r.cfg.Enabled = false
	r.svc.Warm()
	time.Sleep(100 * time.Millisecond)
	if r.model.ensures != 0 {
		t.Fatal("Warm started the model while Omni AI is off")
	}
	r.cfg.Enabled = true
	r.svc.Warm()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		r.model.mu.Lock()
		n := r.model.ensures
		r.model.mu.Unlock()
		if n == 1 {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("Warm did not start the model")
}

func TestNewStopsTheAnswerUnderWayAndStartsEmpty(t *testing.T) {
	r := newRig(t)
	r.llama.hold = make(chan struct{})
	if _, err := r.svc.Send("long one"); err != nil {
		t.Fatal(err)
	}
	view := r.svc.New()
	if view.Busy || len(view.Messages) != 0 {
		t.Fatalf("New left %+v", view)
	}
	r.await(t) // the stopped answer still ends with a Done event
}
