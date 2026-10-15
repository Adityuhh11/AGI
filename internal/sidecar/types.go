package sidecar

// These types mirror the JSON of the memory engine's HTTP API (docs/api.md).

// Health is the answer of GET /health.
type Health struct {
	Ready         bool    `json:"ready"`
	Model         string  `json:"model"`
	Device        *string `json:"device"`
	EmbeddedItems int     `json:"embedded_items"`
	Version       string  `json:"version"`
	// Error is why the model failed to load; when set, Ready will never become true.
	Error *string `json:"error"`
}

// Source is one indexed folder (GET /sources).
type Source struct {
	ID         string `json:"id"`
	Root       string `json:"root"`
	LastScanNs int64  `json:"last_scan_ns"`
	Files      int    `json:"files"`
	Chunks     int    `json:"chunks"`
}

// Job states reported by GET /jobs/{id}.
const (
	JobRunning = "running"
	JobDone    = "done"
	JobFailed  = "failed"
)

// Job is an index job (GET /jobs/{id}).
type Job struct {
	ID         string  `json:"id"`
	SourceID   string  `json:"source_id"`
	State      string  `json:"state"`
	FilesDone  int     `json:"files_done"`
	FilesTotal int     `json:"files_total"`
	Report     *Report `json:"report"`
	Error      *string `json:"error"`
}

// Report is what a finished index job did.
type Report struct {
	Added         int         `json:"added"`
	Updated       int         `json:"updated"`
	Unchanged     int         `json:"unchanged"`
	Removed       int         `json:"removed"`
	Denied        int         `json:"denied"`
	Ignored       int         `json:"ignored"`
	Unsupported   int         `json:"unsupported"`
	TooLarge      int         `json:"too_large"`
	ChunksWritten int         `json:"chunks_written"`
	Errors        []FileError `json:"errors"`
}

// FileError is a file the job could not read; the rest of the folder was still indexed.
type FileError struct {
	URI     string `json:"uri"`
	Message string `json:"message"`
}

// Filters narrow a search; the zero value of each field means "no restriction".
type Filters struct {
	Kinds           []string `json:"kinds,omitempty"`
	SourceID        string   `json:"source_id,omitempty"`
	URIPrefix       string   `json:"uri_prefix,omitempty"`
	ModifiedAfterNs int64    `json:"modified_after_ns,omitempty"`
}

// SearchFilesRequest is the body of POST /search/files.
type SearchFilesRequest struct {
	Query   string   `json:"query"`
	K       int      `json:"k,omitempty"`        // number of files, 1-50 (engine default 10)
	PerFile int      `json:"per_file,omitempty"` // chunks kept per file, 1-10 (engine default 3)
	Filters *Filters `json:"filters,omitempty"`
}

// Chunk is one matching piece of a file.
type Chunk struct {
	ID          string   `json:"id"`
	SourceID    string   `json:"source_id"`
	URI         string   `json:"uri"`
	Kind        string   `json:"kind"`
	Locator     string   `json:"locator"` // JSON text: {"lines":[a,b]} or {"page":n}
	Score       float64  `json:"score"`
	VectorScore *float64 `json:"vector_score"`
	KeywordRank *int     `json:"keyword_rank"`
	Snippet     string   `json:"snippet"`
}

// File is one search result: a file and its best matching chunks.
type File struct {
	SourceID string  `json:"source_id"`
	URI      string  `json:"uri"` // relative to the source's root, '/'-separated
	Kind     string  `json:"kind"`
	Score    float64 `json:"score"`
	Chunks   []Chunk `json:"chunks"`
}

// SearchFilesResponse is the answer of POST /search/files.
type SearchFilesResponse struct {
	Files []File `json:"files"`
	// Confident is false when the best result is probably noise ("no match").
	Confident bool `json:"confident"`
}
