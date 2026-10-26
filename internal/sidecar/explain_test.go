package sidecar

import (
	"errors"
	"fmt"
	"strings"
	"testing"
)

func TestExplainTurnsEngineErrorsIntoAdvice(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want string
	}{
		{"a stale engine", &APIError{Status: 422, Detail: `[{"type":"extra_forbidden"}]`}, "another version"},
		{"a route the engine lacks", fmt.Errorf("survey: %w", &APIError{Status: 404, Detail: "Not Found"}), "another version"},
		{"the graphics card", errors.New(`llama-server answered 500: "decode() failed: vk::Queue::submit: ErrorDeviceLost"`), "graphics card"},
		{"still loading", &APIError{Status: 503, Detail: "loading"}, "still starting"},
		{"a dead engine", errors.New("POST /sources: read tcp: wsarecv: An existing connection was forcibly closed"), "stopped answering"},
		{"a folder that went", errors.New(`[WinError 3] The system cannot find the path specified: 'C:\x'`), "moved or deleted"},
		{"anything else", errors.New("boom"), "details are in the log"},
	}
	for _, tt := range tests {
		if got := Explain(tt.err); !strings.Contains(got, tt.want) {
			t.Errorf("%s: Explain = %q, want it to contain %q", tt.name, got, tt.want)
		}
	}
}
