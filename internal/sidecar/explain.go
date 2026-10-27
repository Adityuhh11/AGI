package sidecar

import (
	"errors"
	"net/http"
	"strings"
)

// Explain says in plain words what an error from the engine means for the person and what they can do. The raw error
// belongs in the log; this is what the windows show.
func Explain(err error) string {
	var api *APIError
	text := err.Error()
	switch {
	// A route the engine does not have (FastAPI's own "Not Found") or a field it does not accept: the launcher and the engine are of
	// different versions. Any other 404 or 422 (a job the restarted engine no longer knows, a query that is too long) is not that.
	case errors.As(err, &api) && ((api.Status == http.StatusNotFound && api.Detail == "Not Found") || (api.Status == http.StatusUnprocessableEntity && strings.Contains(api.Detail, "extra_forbidden"))):
		return "The search engine is from another version of Omni. Restart Omni and it will update itself."
	case strings.Contains(text, "ErrorDeviceLost") || strings.Contains(text, "ErrorOutOfDeviceMemory"):
		return "The graphics card ran out of memory or stopped. Close games or other programs that use it and scan again; Omni falls back to the processor if it keeps happening."
	case NotReady(err):
		return "The search engine is still starting. Try again in a moment."
	case strings.Contains(text, "forcibly closed"), strings.Contains(text, "connection refused"), strings.Contains(text, "EOF"):
		return "The search engine stopped answering. Omni is starting it again; it will carry on by itself."
	case strings.Contains(text, "deadline exceeded"):
		return "That took too long and was given up. Try again; a big index can make the first search slow."
	case strings.Contains(text, "cannot find the path"), strings.Contains(text, "cannot find the file"), strings.Contains(text, "no longer exists"):
		return "A folder or file was moved or deleted while Omni was using it."
	}
	return "Something went wrong. The details are in the log (Settings, About)."
}
