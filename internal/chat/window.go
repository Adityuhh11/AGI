package chat

import (
	"errors"
	"time"
)

// The context window is how much of a conversation the model is shown. The page keeps every message; the model is shown the
// newest whole turns that fit (docs/superpowers/specs/2026-10-28-ai-chat-design.md, section 7).

// messageOverhead is what the chat template adds around a message (the role marker and the end marker), in tokens.
const messageOverhead = 6

// Replies need room in the context as well: the model's answer is generated into it. With thinking on the reasoning comes first,
// and a thinking answer was measured at 51 to 2,011 tokens (docs/eval/2026-10-28-chat-model.md).
const (
	replyReserve         = 2048
	replyReserveThinking = 4096
)

// ErrTooLong means the newest question alone does not fit in the model's context.
var ErrTooLong = errors.New("that message is longer than the model can read at once; shorten it or raise the context size in Settings")

// Reserve is how many tokens of the context are kept free for the reply, which is also its length limit: never more than half
// of the context.
func Reserve(thinking bool, context int) int {
	reserve := replyReserve
	if thinking {
		reserve = replyReserveThinking
	}
	return min(reserve, context/2)
}

// SystemPrompt is what the model is told before the conversation. It says nothing about abilities it does not have, and it
// does not change during a conversation except by the date.
func SystemPrompt(now time.Time) string {
	return "You are Omni AI, a helpful assistant inside the Omni launcher on Windows. Be concise and direct. " +
		"Reply in the language the person writes in. If you are not sure of a fact, say so instead of guessing. " +
		"Today is " + now.Format("Monday, 2 January 2006") + "."
}

// Fit decides which messages the model is shown: it drops the oldest whole turns (a question and its answer) from the front
// until the rest, with the fixed tokens of the system prompt, fit in room. It never splits a turn and never drops the newest
// question. start is the index of the first message shown, used the tokens the shown ones take with the fixed ones.
func Fit(messages []Message, fixed, room int) (start, used int, err error) {
	used = fixed
	for _, m := range messages {
		used += m.Tokens
	}
	for used > room {
		next := -1
		for i := start + 1; i < len(messages); i++ {
			if messages[i].Role == "user" {
				next = i
				break
			}
		}
		if next < 0 {
			return start, used, ErrTooLong
		}
		for i := start; i < next; i++ {
			used -= messages[i].Tokens
		}
		start = next
	}
	return start, used, nil
}

// Wire is a message as the model's API takes it.
type Wire struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// WireMessages turns the shown messages into the request's: the system prompt first, the answers as plain text (their
// reasoning is not sent back, it would only eat the context), and an answer with no text, which a stopped reasoning leaves,
// left out.
func WireMessages(system string, shown []Message) []Wire {
	out := []Wire{{Role: "system", Content: system}}
	for _, m := range shown {
		if m.Role == "assistant" && m.Text == "" {
			continue
		}
		out = append(out, Wire{Role: m.Role, Content: m.Text})
	}
	return out
}
