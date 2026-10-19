// Package sidecar talks to the Python memory engine: a typed HTTP client for its API and a
// supervisor that starts, watches and stops the process.
package sidecar

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// Client calls the engine's API. Every request carries the bearer token.
type Client struct {
	base  string
	token string
	http  *http.Client
}

// NewClient returns a client for the engine at baseURL (for example "http://127.0.0.1:7861").
// A nil httpClient gets a 30-second timeout; searches are fast and indexing is polled, not awaited.
func NewClient(baseURL, token string, httpClient *http.Client) *Client {
	if httpClient == nil {
		httpClient = &http.Client{Timeout: 30 * time.Second}
	}
	return &Client{base: strings.TrimRight(baseURL, "/"), token: token, http: httpClient}
}

// APIError is an error answer from the engine.
type APIError struct {
	Status int    // HTTP status
	Detail string // the engine's explanation
}

func (e *APIError) Error() string {
	return fmt.Sprintf("memory engine answered %d: %s", e.Status, e.Detail)
}

// NotReady reports whether err means the model is still loading (or failed to load).
func NotReady(err error) bool {
	var apiErr *APIError
	return errors.As(err, &apiErr) && apiErr.Status == http.StatusServiceUnavailable
}

// BusyError means another index job is running (or, with an empty JobID, that the source is
// already indexed from a different folder and must be deleted first).
type BusyError struct {
	JobID   string
	Message string
}

func (e *BusyError) Error() string { return e.Message }

// Health asks whether the engine is up and its model loaded.
func (c *Client) Health(ctx context.Context) (Health, error) {
	var out Health
	err := c.do(ctx, http.MethodGet, "/health", nil, &out)
	return out, err
}

// Sources lists what is indexed.
func (c *Client) Sources(ctx context.Context) ([]Source, error) {
	var out []Source
	err := c.do(ctx, http.MethodGet, "/sources", nil, &out)
	return out, err
}

// AddSource starts indexing root as source id and returns the job to follow. exclude lists folders
// below root to leave out, as '/'-separated relative paths. If another job is running it returns a
// *BusyError carrying that job's id.
func (c *Client) AddSource(ctx context.Context, root, id string, exclude []string) (string, error) {
	var out struct {
		JobID string `json:"job_id"`
	}
	body := map[string]any{"path": root, "source_id": id}
	if len(exclude) > 0 {
		body["exclude"] = exclude
	}
	if err := c.do(ctx, http.MethodPost, "/sources", body, &out); err != nil {
		return "", err
	}
	return out.JobID, nil
}

// Job returns the state of an index job.
func (c *Client) Job(ctx context.Context, id string) (Job, error) {
	var out Job
	err := c.do(ctx, http.MethodGet, "/jobs/"+url.PathEscape(id), nil, &out)
	return out, err
}

// DeleteSource forgets a source and returns how many chunks it held. Deleting a source that
// does not exist is fine and returns 0.
func (c *Client) DeleteSource(ctx context.Context, id string) (int, error) {
	var out struct {
		RemovedChunks int `json:"removed_chunks"`
	}
	err := c.do(ctx, http.MethodDelete, "/sources/"+url.PathEscape(id), nil, &out)
	return out.RemovedChunks, err
}

// SetDuty sets the share of the time indexing may keep the GPU busy (0.05 to 1).
func (c *Client) SetDuty(ctx context.Context, duty float64) error {
	return c.do(ctx, http.MethodPut, "/pace", map[string]any{"duty": duty}, nil)
}

// Pause stops indexing at the next batch; Resume lets it go on. Searching is not affected.
func (c *Client) Pause(ctx context.Context) error { return c.do(ctx, http.MethodPost, "/pause", nil, nil) }

// Resume lets a paused index job go on.
func (c *Client) Resume(ctx context.Context) error {
	return c.do(ctx, http.MethodPost, "/resume", nil, nil)
}

// SearchFiles runs a file-level search.
func (c *Client) SearchFiles(ctx context.Context, req SearchFilesRequest) (SearchFilesResponse, error) {
	var out SearchFilesResponse
	err := c.do(ctx, http.MethodPost, "/search/files", req, &out)
	return out, err
}

func (c *Client) do(ctx context.Context, method, path string, body, out any) error {
	var reader io.Reader
	if body != nil {
		encoded, err := json.Marshal(body)
		if err != nil {
			return fmt.Errorf("encode request: %w", err)
		}
		reader = bytes.NewReader(encoded)
	}
	req, err := http.NewRequestWithContext(ctx, method, c.base+path, reader)
	if err != nil {
		return fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+c.token)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("%s %s: %w", method, path, err)
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(io.LimitReader(resp.Body, 16<<20))
	if err != nil {
		return fmt.Errorf("read %s %s: %w", method, path, err)
	}
	if resp.StatusCode >= 300 {
		return errorFor(resp.StatusCode, data)
	}
	if out == nil {
		return nil
	}
	if err := json.Unmarshal(data, out); err != nil {
		return fmt.Errorf("decode %s %s: %w", method, path, err)
	}
	return nil
}

// errorFor turns an error answer into an error. The engine sends {"detail": ...} where detail is
// a string, an object (409 from POST /sources) or a list of validation problems (422).
func errorFor(status int, data []byte) error {
	var envelope struct {
		Detail json.RawMessage `json:"detail"`
	}
	detail := strings.TrimSpace(string(data))
	if json.Unmarshal(data, &envelope) == nil && len(envelope.Detail) > 0 {
		detail = string(envelope.Detail)
		var text string
		var busy struct {
			Message string  `json:"message"`
			JobID   *string `json:"job_id"`
		}
		switch {
		case json.Unmarshal(envelope.Detail, &text) == nil:
			detail = text
		case json.Unmarshal(envelope.Detail, &busy) == nil && busy.Message != "":
			if status == http.StatusConflict {
				job := ""
				if busy.JobID != nil {
					job = *busy.JobID
				}
				return &BusyError{JobID: job, Message: busy.Message}
			}
			detail = busy.Message
		}
	}
	return &APIError{Status: status, Detail: detail}
}
