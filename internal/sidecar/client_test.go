package sidecar

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

const testToken = "t0ken-xxxxxxxxxxxxxxxxxxxxxxxxxx"

// engine answers one request with a fixed status and body and records what it was asked.
type engine struct {
	t      *testing.T
	status int
	body   string
	method string
	path   string
	auth   string
	sent   string
}

func (e *engine) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	data, _ := io.ReadAll(r.Body)
	e.method, e.path, e.auth, e.sent = r.Method, r.URL.EscapedPath(), r.Header.Get("Authorization"), string(data)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(e.status)
	_, _ = io.WriteString(w, e.body)
}

func newClient(t *testing.T, status int, body string) (*Client, *engine) {
	t.Helper()
	e := &engine{t: t, status: status, body: body}
	server := httptest.NewServer(e)
	t.Cleanup(server.Close)
	return NewClient(server.URL+"/", testToken, nil), e
}

func TestEveryRequestCarriesTheTokenAndUsesTheRightRoute(t *testing.T) {
	tests := []struct {
		name       string
		body       string
		call       func(*Client) error
		wantMethod string
		wantPath   string
	}{
		{"health", `{"ready":true,"model":"m","device":"cuda","embedded_items":3,"version":"1","error":null}`,
			func(c *Client) error { _, err := c.Health(context.Background()); return err }, "GET", "/health"},
		{"sources", `[]`,
			func(c *Client) error { _, err := c.Sources(context.Background()); return err }, "GET", "/sources"},
		{"job", `{"id":"job-1","source_id":"s","state":"running","files_done":1,"files_total":2,"report":null,"error":null}`,
			func(c *Client) error { _, err := c.Job(context.Background(), "job-1"); return err }, "GET", "/jobs/job-1"},
		{"delete", `{"removed_chunks":4}`,
			func(c *Client) error { _, err := c.DeleteSource(context.Background(), "root-abc"); return err }, "DELETE", "/sources/root-abc"},
		{"add", `{"job_id":"job-2"}`,
			func(c *Client) error {
				_, err := c.AddSource(context.Background(), `C:\x`, "root-abc", nil, IndexOptions{})
				return err
			}, "POST", "/sources"},
		{"search", `{"files":[],"confident":false}`,
			func(c *Client) error {
				_, err := c.SearchFiles(context.Background(), SearchFilesRequest{Query: "q"})
				return err
			}, "POST", "/search/files"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client, e := newClient(t, 200, tt.body)
			if err := tt.call(client); err != nil {
				t.Fatal(err)
			}
			if e.method != tt.wantMethod || e.path != tt.wantPath {
				t.Fatalf("called %s %s, want %s %s", e.method, e.path, tt.wantMethod, tt.wantPath)
			}
			if e.auth != "Bearer "+testToken {
				t.Fatalf("Authorization = %q", e.auth)
			}
		})
	}
}

func TestHealthAndJobAnswersAreDecoded(t *testing.T) {
	client, _ := newClient(t, 200, `{"ready":false,"model":"m","device":null,"embedded_items":0,"version":"0.1.0","error":"no gpu"}`)
	health, err := client.Health(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if health.Ready || health.Device != nil || health.Error == nil || *health.Error != "no gpu" {
		t.Fatalf("health = %+v", health)
	}

	client, _ = newClient(t, 200, `{"id":"job-1","source_id":"s","state":"done","files_done":3,"files_total":3,
		"report":{"added":3,"updated":0,"unchanged":0,"removed":0,"denied":1,"ignored":0,"unsupported":0,"too_large":0,
		"chunks_written":9,"errors":[{"uri":"a.png","message":"bad"}]},"error":null}`)
	job, err := client.Job(context.Background(), "job-1")
	if err != nil {
		t.Fatal(err)
	}
	if job.State != JobDone || job.Report == nil || job.Report.ChunksWritten != 9 || job.Report.Errors[0].URI != "a.png" {
		t.Fatalf("job = %+v", job)
	}
}

func TestSearchSendsOnlyWhatWasAskedAndDecodesFiles(t *testing.T) {
	client, e := newClient(t, 200, `{"files":[{"source_id":"s","uri":"a/b.js","kind":"code","score":0.8,
		"chunks":[{"id":"c1","source_id":"s","uri":"a/b.js","kind":"code","locator":"{\"lines\":[1,40]}","score":0.8,
		"vector_score":0.79,"keyword_rank":null,"snippet":"hello"}]}],"confident":true}`)
	got, err := client.SearchFiles(context.Background(), SearchFilesRequest{Query: "jwt", K: 5})
	if err != nil {
		t.Fatal(err)
	}
	var sent map[string]any
	if err := json.Unmarshal([]byte(e.sent), &sent); err != nil {
		t.Fatal(err)
	}
	if len(sent) != 2 || sent["query"] != "jwt" || sent["k"] != float64(5) {
		t.Fatalf("request body = %s, want only query and k", e.sent)
	}
	if !got.Confident || len(got.Files) != 1 {
		t.Fatalf("response = %+v", got)
	}
	chunk := got.Files[0].Chunks[0]
	if chunk.Locator != `{"lines":[1,40]}` || chunk.VectorScore == nil || *chunk.VectorScore != 0.79 || chunk.KeywordRank != nil {
		t.Fatalf("chunk = %+v", chunk)
	}

	filters := &Filters{Kinds: []string{"code"}, SourceID: "s", URIPrefix: "a/", ModifiedAfterNs: 5}
	if _, err := client.SearchFiles(context.Background(), SearchFilesRequest{Query: "x", Filters: filters}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(e.sent, `"filters":{"kinds":["code"],"source_id":"s","uri_prefix":"a/","modified_after_ns":5}`) {
		t.Fatalf("filters were not sent as documented: %s", e.sent)
	}
}

func TestAddSourceSendsThePathAndTheId(t *testing.T) {
	client, e := newClient(t, 202, `{"job_id":"job-7"}`)
	options := IndexOptions{PDFPages: "text", Photos: "name", Videos: "content"}
	job, err := client.AddSource(context.Background(), `C:\Users\me\Docs`, "root-1", []string{"vendor/skills"}, options)
	if err != nil {
		t.Fatal(err)
	}
	if job != "job-7" {
		t.Fatalf("job id = %q", job)
	}
	var sent struct {
		Path     string       `json:"path"`
		SourceID string       `json:"source_id"`
		Exclude  []string     `json:"exclude"`
		Options  IndexOptions `json:"options"`
	}
	if err := json.Unmarshal([]byte(e.sent), &sent); err != nil {
		t.Fatal(err)
	}
	if sent.Path != `C:\Users\me\Docs` || sent.SourceID != "root-1" || len(sent.Exclude) != 1 || sent.Exclude[0] != "vendor/skills" || sent.Options != options {
		t.Fatalf("body = %s", e.sent)
	}
	if !strings.Contains(e.sent, `"options":{"pdf_pages":"text","photos":"name","videos":"content"}`) {
		t.Fatalf("options were not sent as documented: %s", e.sent)
	}
}

func TestErrorAnswersBecomeUsefulErrors(t *testing.T) {
	tests := []struct {
		name   string
		status int
		body   string
		check  func(*testing.T, error)
	}{
		{"a wrong token", 401, `{"detail":"missing or wrong bearer token"}`, func(t *testing.T, err error) {
			var apiErr *APIError
			if !errors.As(err, &apiErr) || apiErr.Status != 401 || apiErr.Detail != "missing or wrong bearer token" {
				t.Fatalf("err = %v", err)
			}
		}},
		{"the model still loading", 503, `{"detail":"the model is still loading"}`, func(t *testing.T, err error) {
			if !NotReady(err) {
				t.Fatalf("NotReady(%v) = false", err)
			}
		}},
		{"another job running", 409, `{"detail":{"message":"job-3 is still running","job_id":"job-3"}}`, func(t *testing.T, err error) {
			var busy *BusyError
			if !errors.As(err, &busy) || busy.JobID != "job-3" || busy.Message != "job-3 is still running" {
				t.Fatalf("err = %v", err)
			}
		}},
		{"a source indexed from another folder", 409, `{"detail":{"message":"s is indexed from C:\\a; delete it first","job_id":null}}`, func(t *testing.T, err error) {
			var busy *BusyError
			if !errors.As(err, &busy) || busy.JobID != "" || !strings.Contains(busy.Message, "delete it first") {
				t.Fatalf("err = %v", err)
			}
		}},
		{"a source being indexed cannot be deleted", 409, `{"detail":"repo is being indexed"}`, func(t *testing.T, err error) {
			var apiErr *APIError
			if !errors.As(err, &apiErr) || apiErr.Status != 409 || apiErr.Detail != "repo is being indexed" {
				t.Fatalf("err = %v", err)
			}
		}},
		{"validation problems", 422, `{"detail":[{"loc":["body","k"],"msg":"too big"}]}`, func(t *testing.T, err error) {
			var apiErr *APIError
			if !errors.As(err, &apiErr) || apiErr.Status != 422 || !strings.Contains(apiErr.Detail, "too big") {
				t.Fatalf("err = %v", err)
			}
		}},
		{"an unknown job", 404, `{"detail":"no such job: job-99"}`, func(t *testing.T, err error) {
			var apiErr *APIError
			if !errors.As(err, &apiErr) || apiErr.Status != 404 {
				t.Fatalf("err = %v", err)
			}
		}},
		{"a host check failure with a plain body", 400, `Invalid host header`, func(t *testing.T, err error) {
			var apiErr *APIError
			if !errors.As(err, &apiErr) || apiErr.Status != 400 || apiErr.Detail != "Invalid host header" {
				t.Fatalf("err = %v", err)
			}
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client, _ := newClient(t, tt.status, tt.body)
			_, err := client.Job(context.Background(), "job-1")
			if err == nil {
				t.Fatal("expected an error")
			}
			tt.check(t, err)
		})
	}
}

func TestNotReadyIsFalseForOtherErrors(t *testing.T) {
	if NotReady(nil) || NotReady(errors.New("boom")) || NotReady(&APIError{Status: 500}) {
		t.Fatal("NotReady must only be true for a 503 answer")
	}
}

func TestMalformedAnswersAndDeadServersAreErrors(t *testing.T) {
	client, _ := newClient(t, 200, `not json`)
	if _, err := client.Health(context.Background()); err == nil || !strings.Contains(err.Error(), "decode") {
		t.Fatalf("err = %v, want a decode error", err)
	}

	dead := NewClient("http://127.0.0.1:1", testToken, &http.Client{Timeout: time.Second})
	if _, err := dead.Health(context.Background()); err == nil {
		t.Fatal("expected a connection error")
	}
}

func TestACancelledContextStopsTheRequest(t *testing.T) {
	slow := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		select {
		case <-r.Context().Done():
		case <-time.After(5 * time.Second):
		}
	}))
	defer slow.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	started := time.Now()
	_, err := NewClient(slow.URL, testToken, nil).Health(ctx)
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("err = %v, want a deadline error", err)
	}
	if time.Since(started) > 2*time.Second {
		t.Fatal("the request did not stop when its context ended")
	}
}

func TestPathsAreEscaped(t *testing.T) {
	client, e := newClient(t, 200, `{"removed_chunks":0}`)
	if _, err := client.DeleteSource(context.Background(), "a b/c"); err != nil {
		t.Fatal(err)
	}
	if e.path != "/sources/a%20b%2Fc" {
		t.Fatalf("path = %q", e.path)
	}
}
