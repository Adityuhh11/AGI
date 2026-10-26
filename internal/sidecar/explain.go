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
	case errors.As(err, &api) && (api.Status == http.StatusUnprocessableEntity || api.Status == http.StatusNotFound):
		return "The search engine is from another version of AGI. Restart AGI and it will update itself."
	case strings.Contains(text, "ErrorDeviceLost") || strings.Contains(text, "ErrorOutOfDeviceMemory"):
		return "The graphics card ran out of memory or stopped. Close games or other programs that use it and scan again; AGI falls back to the processor if it keeps happening."
	case NotReady(err):
		return "The search engine is still starting. Try again in a moment."
	case strings.Contains(text, "forcibly closed"), strings.Contains(text, "connection refused"), strings.Contains(text, "EOF"), strings.Contains(text, "deadline exceeded"):
		return "The search engine stopped answering. AGI is starting it again; it will carry on by itself."
	case strings.Contains(text, "cannot find the path"), strings.Contains(text, "cannot find the file"), strings.Contains(text, "no longer exists"):
		return "A folder or file was moved or deleted while AGI was using it."
	}
	return "Something went wrong. The details are in the log (Settings, About)."
}
