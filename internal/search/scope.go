package search

import "strings"

// A scope is typed in front of a query to search one kind of file: "code: retry with backoff".
// Anything else before a colon ("note to self: buy milk") is part of the query.
type scope struct {
	label string   // what the bar shows
	kinds []string // the engine's chunk kinds
}

var (
	codeScope  = scope{"Code", []string{"code"}}
	imageScope = scope{"Images", []string{"image"}}
	pdfScope   = scope{"PDFs", []string{"pdf_text", "pdf_page"}}
	docScope   = scope{"Documents", []string{"markdown", "text", "file", "audio"}} // audio: by name, as before it had a kind
	videoScope = scope{"Videos", []string{"video", "video_frame"}}
	// audioScope is what is heard: audio files, and the soundtrack of videos beside the videos found by name.
	audioScope = scope{"Audio", []string{"audio", "audio_window", "video"}}
)

var scopes = map[string]scope{
	"code": codeScope,
	"img":  imageScope, "image": imageScope, "images": imageScope,
	"vid": videoScope, "video": videoScope, "videos": videoScope,
	"aud": audioScope, "audio": audioScope, "sound": audioScope, "sounds": audioScope,
	"pdf": pdfScope, "pdfs": pdfScope,
	"doc": docScope, "docs": docScope, "text": docScope, "notes": docScope,
}

// parseScope splits a leading "scope:" from the query. It reports false, and returns the query
// unchanged, when the query does not start with a known scope.
func parseScope(query string) (scope, string, bool) {
	name, rest, found := strings.Cut(query, ":")
	if !found {
		return scope{}, query, false
	}
	sc, known := scopes[strings.ToLower(strings.TrimSpace(name))]
	if !known {
		return scope{}, query, false
	}
	return sc, strings.TrimSpace(rest), true
}
