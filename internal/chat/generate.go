package chat

import (
	"context"
	"errors"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"omni/internal/llm"
)

// deltaEvery is how often the pieces of an answer are sent to the page: one batch per frame or two, not one event per token.
const deltaEvery = 40 * time.Millisecond

// Send adds a question to the conversation and starts the answer in the background. The answer arrives as events; the view it
// returns already holds the question.
func (s *Service) Send(text string) (View, error) {
	text = strings.TrimSpace(text)
	if text == "" {
		return s.Current(), nil
	}
	return s.begin(&Message{Role: "user", Text: text})
}

// AskNew starts a new conversation with a question: what the bar's "Ask" row hands to the window.
func (s *Service) AskNew(text string) (View, error) {
	s.New()
	return s.Send(text)
}

// Regenerate answers the last question again; it is also how a failed answer is tried again.
func (s *Service) Regenerate() (View, error) {
	s.mu.Lock()
	if n := len(s.conv.Messages); n == 0 {
		s.mu.Unlock()
		return View{}, errors.New("there is nothing to answer")
	}
	s.mu.Unlock()
	return s.begin(nil)
}

// begin starts an answer. With a question it is added first; without, the answers after the last question are dropped and
// that question is answered again.
func (s *Service) begin(question *Message) (View, error) {
	cfg := s.o.Settings()
	if !cfg.Enabled || !s.o.Installed() {
		return View{}, errors.New("Omni AI is not turned on. Turn it on in Settings.")
	}
	s.mu.Lock()
	if s.busy {
		s.mu.Unlock()
		return View{}, errors.New("wait for the answer, or stop it first")
	}
	if question != nil {
		s.conv.Messages = append(s.conv.Messages, *question)
	} else {
		last := lastUser(s.conv.Messages)
		if len(s.conv.Messages) == 0 || s.conv.Messages[last].Role != "user" {
			s.mu.Unlock()
			return View{}, errors.New("there is no question to answer")
		}
		s.conv.Messages = s.conv.Messages[:last+1]
	}
	ctx, cancel := context.WithCancel(context.Background())
	s.busy, s.cancel, s.finished = true, cancel, make(chan struct{})
	finished := s.finished
	if s.conv.Title == "" {
		s.conv.Title = TitleOf(s.conv.Messages)
	}
	s.persistLocked()
	view := s.viewLocked()
	s.mu.Unlock()
	go func() {
		defer close(finished)
		s.generate(ctx, cfg.Thinking, cfg.WebSearch && s.o.Web != nil)
		cancel()
	}()
	return view, nil
}

// reply collects an answer as it streams, and sends it to the page in batches.
type reply struct {
	mu           sync.Mutex
	text, think  strings.Builder
	unsentText   strings.Builder
	unsentThink  strings.Builder
	thinkStart   time.Time
	thinkEnd     time.Time
	firstContent bool
	endpoint     llm.Endpoint // the server that answered, to count the answer afterwards
	haveEndpoint bool
	sources      []Source // the pages the answer drew on, when it looked at the web
	gap          bool     // text came before a tool call, so a break goes before the next text
}

func (r *reply) addText(piece string, status func(Status)) {
	r.mu.Lock()
	if r.gap { // text came before a tool call: a break goes between it and what follows
		piece, r.gap = "\n\n"+piece, false
	}
	first := !r.firstContent
	r.firstContent = true
	if first && !r.thinkStart.IsZero() {
		r.thinkEnd = time.Now()
	}
	r.text.WriteString(piece)
	r.unsentText.WriteString(piece)
	r.mu.Unlock()
	if first {
		status(Status{Phase: "answering"})
	}
}

func (r *reply) addThinking(piece string, status func(Status)) {
	r.mu.Lock()
	first := r.thinkStart.IsZero()
	if first {
		r.thinkStart = time.Now()
	}
	r.think.WriteString(piece)
	r.unsentThink.WriteString(piece)
	r.mu.Unlock()
	if first {
		status(Status{Phase: "thinking", Text: "Thinking…"})
	}
}

// take returns what has not been sent yet.
func (r *reply) take() Delta {
	r.mu.Lock()
	defer r.mu.Unlock()
	d := Delta{Text: r.unsentText.String(), Thinking: r.unsentThink.String()}
	r.unsentText.Reset()
	r.unsentThink.Reset()
	return d
}

func (r *reply) anything() bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.text.Len() > 0 || r.think.Len() > 0
}

// generate asks the model, streams the answer to the page and records it. It always ends with a Done event.
func (s *Service) generate(ctx context.Context, thinking, web bool) {
	release := s.o.Model.Hold()
	status := func(st Status) { s.o.Emit(EventStatus, st) }

	var r reply
	stopFlush, flushed := make(chan struct{}), make(chan struct{})
	go func() { // the answer goes to the page in batches, not one event per token
		defer close(flushed)
		tick := time.NewTicker(deltaEvery)
		defer tick.Stop()
		for {
			select {
			case <-tick.C:
				s.flush(&r)
			case <-stopFlush:
				s.flush(&r)
				return
			}
		}
	}()
	failed := s.answer(ctx, thinking, web, &r, status)
	close(stopFlush)
	<-flushed

	tokens := 0
	if r.anything() {
		tokens = s.tokensOf(&r)
	}

	restore := ""
	s.mu.Lock()
	if failed != nil && errors.Is(failed, ErrTooLong) {
		// the question was never sent: take it out of the conversation and give its text back
		last := len(s.conv.Messages) - 1
		restore = s.conv.Messages[last].Text
		s.conv.Messages = s.conv.Messages[:last]
		if len(s.conv.Messages) == 0 {
			s.conv.Title = ""
		}
	}
	if r.anything() {
		text := r.text.String()
		msg := Message{Role: "assistant", Text: text, Thinking: r.think.String(), Stopped: ctx.Err() != nil, Sources: r.sources}
		if !r.thinkStart.IsZero() {
			end := r.thinkEnd
			if end.IsZero() {
				end = time.Now()
			}
			msg.ThinkingMS = end.Sub(r.thinkStart).Milliseconds()
		}
		msg.Tokens = tokens
		s.conv.Messages = append(s.conv.Messages, msg)
	}
	s.persistLocked()
	s.busy, s.cancel = false, nil
	view := s.viewLocked()
	s.mu.Unlock()

	release() // the idle clock counts from the end of the answer, so the hold ends before the page is told
	done := Done{View: view, Restore: restore}
	if failed != nil {
		done.Error = Explain(failed)
		s.o.Logf("chat: %v", failed)
	}
	s.o.Emit(EventDone, done)
}

// flush sends what has arrived since the last batch.
func (s *Service) flush(r *reply) {
	if d := r.take(); d != (Delta{}) {
		s.o.Emit(EventDelta, d)
	}
}

// tokensOf is what an answer takes of the context. The server counts it exactly when it can be asked; if not, three
// characters to a token is close enough for a message that is only counted once.
func (s *Service) tokensOf(r *reply) int {
	text := r.text.String()
	if r.haveEndpoint {
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		if n, err := Tokens(ctx, r.endpoint, text); err == nil {
			return n + messageOverhead
		}
	}
	return utf8.RuneCountInString(text)/3 + messageOverhead
}

// answer does the work of one answer and returns why it failed, if it did. A stop is not a failure. With web search on, the model
// may ask for tools first: each round runs them and gives the results back, up to maxToolRounds, and the request after that has no
// tools so the model has to answer.
func (s *Service) answer(ctx context.Context, thinking, web bool, r *reply, status func(Status)) error {
	if s.o.Model.Status().State != llm.Ready {
		status(Status{Phase: "loading", Text: "Starting the model…"})
	}
	ep, err := s.o.Model.Ensure(ctx)
	if err != nil {
		if ctx.Err() != nil {
			return nil
		}
		return err
	}
	r.endpoint, r.haveEndpoint = ep, true
	status(Status{Phase: "reading", Text: "Reading…"})

	req, spare, err := s.prepare(ctx, ep, thinking, web)
	if err != nil {
		if ctx.Err() != nil {
			return nil
		}
		return err
	}
	var look *lookup
	if web && spare*charsPerToken >= minToolChars {
		s.mu.Lock()
		allowed := addressesIn(s.conv.Messages)
		s.mu.Unlock()
		look = &lookup{web: s.o.Web, chars: spare * charsPerToken, status: status, pages: map[string]bool{}, allowed: allowed, logf: s.o.Logf}
		req.Tools = webTools()
	} else if web { // no room for results: the model is not told it has tools it was not given
		req.Messages[0].Content = SystemPrompt(s.o.Now(), false)
	}
	for round := 1; ; round++ {
		calls, err := s.complete(ctx, &ep, req, r, status)
		if err != nil || ctx.Err() != nil || len(calls) == 0 || look == nil || round > maxToolRounds {
			if look != nil {
				r.sources = look.sources()
			}
			return err
		}
		calls = calls[:min(len(calls), maxCallsPerRound)]
		req.Messages = append(req.Messages, assistantCalls(calls))
		for _, c := range calls {
			req.Messages = append(req.Messages, Wire{Role: "tool", ToolCallID: c.ID, Content: look.run(ctx, c)})
		}
		r.separate()
		status(Status{Phase: "reading", Text: "Reading the results…"})
		if round >= maxToolRounds || look.chars < minToolChars {
			// Out of rounds or of room: the next request must answer. Without these words a model that wanted another search writes
			// its call out as plain text, and that would be the answer the person reads.
			req.Tools = nil
			req.Messages = append(req.Messages, Wire{Role: "user", Content: "Answer now with what you have found. If it does not answer the question, say so. Do not search again."})
		}
	}
}

// complete sends the request and streams the reply into r. A model that dropped or ran out of graphics memory before it said
// anything is started again, once. It returns the tools the model asked for.
func (s *Service) complete(ctx context.Context, ep *llm.Endpoint, req Request, r *reply, status func(Status)) ([]Call, error) {
	for attempt := 0; ; attempt++ {
		before := r.size()
		_, calls, err := Complete(ctx, *ep, req, func(piece string) { r.addText(piece, status) }, func(piece string) { r.addThinking(piece, status) })
		if err == nil || ctx.Err() != nil {
			return calls, nil
		}
		if attempt > 0 || r.size() > before {
			return nil, err
		}
		switch {
		case outOfMemory(err):
			s.o.Model.UseProcessor()
			status(Status{Phase: "loading", Text: "The graphics card ran out of memory; starting the model on the processor…"})
		case connectionLost(err):
			status(Status{Phase: "loading", Text: "Starting the model again…"})
		default:
			return nil, err
		}
		next, err := s.o.Model.Ensure(ctx)
		if err != nil {
			if ctx.Err() != nil {
				return nil, nil
			}
			return nil, err
		}
		*ep, r.endpoint = next, next
	}
}

// prepare counts what the new question and the system prompt take, decides which messages the model is shown and builds the
// request. spare is the tokens of the context still free after the messages and the room kept for the reply: what tool results
// may take.
func (s *Service) prepare(ctx context.Context, ep llm.Endpoint, thinking, web bool) (req Request, spare int, err error) {
	cfg := s.o.Settings()
	system := SystemPrompt(s.o.Now(), web)
	sysTokens, err := Tokens(ctx, ep, system)
	if err != nil {
		return Request{}, 0, err
	}
	s.mu.Lock()
	question := s.conv.Messages[len(s.conv.Messages)-1]
	s.mu.Unlock()
	questionTokens := question.Tokens
	if questionTokens == 0 { // a question that is being answered again has been counted already
		n, err := Tokens(ctx, ep, question.Text)
		if err != nil {
			return Request{}, 0, err
		}
		questionTokens = n + messageOverhead
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	s.systemTok = sysTokens + messageOverhead
	s.conv.Messages[len(s.conv.Messages)-1].Tokens = questionTokens
	reserve := Reserve(thinking, cfg.ContextTokens)
	start, used, err := Fit(s.conv.Messages, s.systemTok, cfg.ContextTokens-reserve)
	if err != nil {
		return Request{}, 0, err
	}
	// the tools' descriptions and the closing "answer now" message take room too
	return NewRequest(WireMessages(system, s.conv.Messages[start:]), reserve, thinking), cfg.ContextTokens - reserve - used - toolOverheadTokens, nil
}

// size is how much of an answer (text and reasoning) has arrived, to tell whether a failed request got anywhere.
func (r *reply) size() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.text.Len() + r.think.Len()
}

// separate keeps what the model wrote before it asked for a tool apart from what it writes after.
func (r *reply) separate() {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.gap = r.text.Len() > 0
}
