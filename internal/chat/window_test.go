package chat

import (
	"strings"
	"testing"
	"time"
)

func turns(tokens ...int) []Message {
	var out []Message
	for i, n := range tokens {
		role := "user"
		if i%2 == 1 {
			role = "assistant"
		}
		out = append(out, Message{Role: role, Text: role, Tokens: n})
	}
	return out
}

func TestFitKeepsEverythingThatFits(t *testing.T) {
	start, used, err := Fit(turns(10, 20, 30, 40), 50, 1000)
	if err != nil || start != 0 || used != 150 {
		t.Fatalf("start %d used %d err %v", start, used, err)
	}
}

func TestFitDropsTheOldestWholeTurnsUntilItFits(t *testing.T) {
	// four turns of 100 tokens each, 20 fixed: 420 in all
	messages := turns(50, 50, 50, 50, 50, 50, 50, 50)
	for name, tc := range map[string]struct{ room, wantStart, wantUsed int }{
		"one turn too many":  {room: 350, wantStart: 2, wantUsed: 320},
		"two turns too many": {room: 250, wantStart: 4, wantUsed: 220},
		"only the last":      {room: 120, wantStart: 6, wantUsed: 120},
	} {
		start, used, err := Fit(messages, 20, tc.room)
		if err != nil || start != tc.wantStart || used != tc.wantUsed {
			t.Errorf("%s: start %d used %d err %v; want start %d used %d", name, start, used, err, tc.wantStart, tc.wantUsed)
		}
		if messages[start].Role != "user" {
			t.Errorf("%s: the window starts on a %s message; a turn was split", name, messages[start].Role)
		}
	}
}

func TestFitNeverDropsTheNewestQuestionAndRefusesOneThatCannotFit(t *testing.T) {
	_, _, err := Fit(turns(50, 50, 900), 20, 500)
	if err != ErrTooLong {
		t.Fatalf("err = %v, want ErrTooLong", err)
	}
	if _, _, err := Fit(turns(900), 20, 500); err != ErrTooLong {
		t.Fatalf("a single oversize question: err = %v", err)
	}
}

func TestFitSkipsOverAnUnansweredQuestion(t *testing.T) {
	// a failed answer leaves two questions in a row; the window still starts on a question
	messages := []Message{
		{Role: "user", Tokens: 100}, {Role: "user", Tokens: 100}, {Role: "assistant", Tokens: 100}, {Role: "user", Tokens: 100},
	}
	start, used, err := Fit(messages, 0, 350)
	if err != nil || start != 1 || used != 300 {
		t.Fatalf("start %d used %d err %v", start, used, err)
	}
}

func TestReserveIsBiggerWithThinkingAndNeverMoreThanHalf(t *testing.T) {
	for name, tc := range map[string]struct {
		thinking bool
		context  int
		want     int
	}{
		"plain 8k":    {false, 8192, 2048},
		"thinking 8k": {true, 8192, 4096},
		"thinking 4k": {true, 4096, 2048},
		"tiny":        {false, 2048, 1024},
	} {
		if got := Reserve(tc.thinking, tc.context); got != tc.want {
			t.Errorf("%s: Reserve = %d, want %d", name, got, tc.want)
		}
	}
}

func TestWireMessagesPutTheSystemPromptFirstAndSkipEmptyAnswers(t *testing.T) {
	got := WireMessages("SYS", []Message{
		{Role: "user", Text: "q1"}, {Role: "assistant", Text: "", Thinking: "hmm", Stopped: true},
		{Role: "user", Text: "q2"}, {Role: "assistant", Text: "a2", Thinking: "secret reasoning"},
	})
	want := []Wire{{"system", "SYS"}, {"user", "q1"}, {"user", "q2"}, {"assistant", "a2"}}
	if len(got) != len(want) {
		t.Fatalf("got %+v", got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("message %d = %+v, want %+v", i, got[i], want[i])
		}
		if strings.Contains(got[i].Content, "secret") {
			t.Error("reasoning was sent back to the model")
		}
	}
}

func TestSystemPromptStatesTheDate(t *testing.T) {
	got := SystemPrompt(time.Date(2026, 10, 28, 9, 0, 0, 0, time.UTC))
	if !strings.Contains(got, "Wednesday, 28 October 2026") {
		t.Fatalf("system prompt lacks the date: %q", got)
	}
}
