package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"
)

// TestRealModelAnswers starts the real llama.cpp server on the real model, asks one thing and unloads it. It runs only when
// OMNI_CHAT_DIR names an installed chat folder (the test is for the person who changes this package, not for CI).
func TestRealModelAnswers(t *testing.T) {
	dir := os.Getenv("OMNI_CHAT_DIR")
	if dir == "" {
		t.Skip("OMNI_CHAT_DIR is not set")
	}
	layout, err := Locate("")
	if err != nil || !layout.Installed() {
		t.Fatalf("no install at %s: %v", dir, err)
	}
	srv := NewServer(Options{
		Layout: func() Layout { return layout }, ContextTokens: func() int { return 8192 }, IdleMinutes: func() int { return 0 },
		Logf: t.Logf,
	})
	t.Cleanup(srv.Unload)

	began := time.Now()
	ep, err := srv.Ensure(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("ready on %s in %v", ep.Device, time.Since(began).Round(time.Millisecond))

	body, _ := json.Marshal(map[string]any{
		"messages":             []map[string]string{{"role": "user", "content": "What is the capital of France? One word."}},
		"max_tokens":           50,
		"stop":                 []string{"</think>"},
		"chat_template_kwargs": map[string]bool{"enable_thinking": false},
	})
	req, _ := http.NewRequest(http.MethodPost, ep.URL+"/v1/chat/completions", bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+ep.Key)
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	var out struct {
		Choices []struct {
			Message struct{ Content string } `json:"message"`
		} `json:"choices"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil || len(out.Choices) == 0 {
		t.Fatalf("decode: %v %+v", err, out)
	}
	if !strings.Contains(out.Choices[0].Message.Content, "Paris") {
		t.Fatalf("answer = %q", out.Choices[0].Message.Content)
	}
	srv.Unload()
	if srv.Status().State != Unloaded {
		t.Fatalf("status after Unload = %+v", srv.Status())
	}
}
