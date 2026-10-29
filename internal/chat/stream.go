package chat

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"omni/internal/llm"
)

// Request is a chat completion as the llama.cpp server takes it (an OpenAI-style API).
type Request struct {
	Messages      []Wire          `json:"messages"`
	Stream        bool            `json:"stream"`
	StreamOptions map[string]bool `json:"stream_options"`
	MaxTokens     int             `json:"max_tokens"`
	// Stop is set when thinking is off: with it off the model sometimes writes a closing </think> it never opened and then
	// repeats its answer; stopping there keeps the first, complete answer (docs/eval/2026-10-28-chat-model.md).
	Stop []string `json:"stop,omitempty"`
	// Template switches thinking per request: {"enable_thinking": false}.
	Template map[string]bool `json:"chat_template_kwargs"`
	// Tools are what the model may call instead of answering (tools.go); empty offers none.
	Tools []Tool `json:"tools,omitempty"`
}

// NewRequest builds the request for the shown messages.
func NewRequest(messages []Wire, maxTokens int, thinking bool) Request {
	r := Request{
		Messages: messages, Stream: true, StreamOptions: map[string]bool{"include_usage": true},
		MaxTokens: maxTokens, Template: map[string]bool{"enable_thinking": thinking},
	}
	if !thinking {
		r.Stop = []string{"</think>"}
	}
	return r
}

// Usage is what the server counted for one request.
type Usage struct{ Prompt, Completion int }

// StatusError is an answer from the server that was not a success.
type StatusError struct {
	Status  int
	Message string
}

func (e *StatusError) Error() string {
	return fmt.Sprintf("the model answered %d: %s", e.Status, e.Message)
}

type chunk struct {
	Choices []struct {
		Delta struct {
			Content   string `json:"content"`
			Reasoning string `json:"reasoning_content"`
			ToolCalls []struct {
				Index    int    `json:"index"`
				ID       string `json:"id"`
				Function struct {
					Name      string `json:"name"`
					Arguments string `json:"arguments"`
				} `json:"function"`
			} `json:"tool_calls"`
		} `json:"delta"`
	} `json:"choices"`
	Usage *struct {
		Prompt     int `json:"prompt_tokens"`
		Completion int `json:"completion_tokens"`
	} `json:"usage"`
}

// Complete sends the request and calls text and thinking with the pieces of the answer as they arrive. It returns when the
// answer is finished or ctx ends; a cut answer is not an error to the caller, who keeps what arrived. When the model asked to
// call tools instead of (or after) writing, the calls are returned whole.
func Complete(ctx context.Context, ep llm.Endpoint, req Request, text, thinking func(string)) (Usage, []Call, error) {
	body, err := json.Marshal(req)
	if err != nil {
		return Usage{}, nil, err
	}
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, ep.URL+"/v1/chat/completions", bytes.NewReader(body))
	if err != nil {
		return Usage{}, nil, err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+ep.Key)
	resp, err := http.DefaultClient.Do(httpReq)
	if err != nil {
		return Usage{}, nil, fmt.Errorf("ask the model: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return Usage{}, nil, statusError(resp)
	}
	var usage Usage
	var calls callBuilder
	scanner := bufio.NewScanner(resp.Body)
	scanner.Buffer(make([]byte, 0, 64<<10), 4<<20)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		data, ok := strings.CutPrefix(line, "data:")
		if !ok {
			continue
		}
		data = strings.TrimSpace(data)
		if data == "[DONE]" {
			break
		}
		var c chunk
		if err := json.Unmarshal([]byte(data), &c); err != nil {
			continue // a line that is not a chunk (a keep-alive comment) says nothing
		}
		if c.Usage != nil {
			usage = Usage{Prompt: c.Usage.Prompt, Completion: c.Usage.Completion}
		}
		for _, choice := range c.Choices {
			if choice.Delta.Reasoning != "" {
				thinking(choice.Delta.Reasoning)
			}
			if choice.Delta.Content != "" {
				text(choice.Delta.Content)
			}
			for _, piece := range choice.Delta.ToolCalls {
				calls.add(piece.Index, piece.ID, piece.Function.Name, piece.Function.Arguments)
			}
		}
	}
	if err := scanner.Err(); err != nil {
		if ctx.Err() != nil {
			return usage, nil, nil // cut: calls that were not finished are not run
		}
		return usage, nil, fmt.Errorf("read the answer: %w", err)
	}
	return usage, calls.done(), nil
}

func statusError(resp *http.Response) error {
	raw, _ := io.ReadAll(io.LimitReader(resp.Body, 4<<10))
	var wrapped struct {
		Error struct {
			Message string `json:"message"`
		} `json:"error"`
	}
	message := strings.TrimSpace(string(raw))
	if json.Unmarshal(raw, &wrapped) == nil && wrapped.Error.Message != "" {
		message = wrapped.Error.Message
	}
	return &StatusError{Status: resp.StatusCode, Message: message}
}

// Tokens asks the server how many tokens text is, exactly.
func Tokens(ctx context.Context, ep llm.Endpoint, text string) (int, error) {
	body, _ := json.Marshal(map[string]string{"content": text})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, ep.URL+"/tokenize", bytes.NewReader(body))
	if err != nil {
		return 0, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+ep.Key)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return 0, fmt.Errorf("count tokens: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return 0, statusError(resp)
	}
	var out struct {
		Tokens []json.RawMessage `json:"tokens"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return 0, fmt.Errorf("count tokens: %w", err)
	}
	return len(out.Tokens), nil
}
