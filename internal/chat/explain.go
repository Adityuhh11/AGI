package chat

import (
	"errors"
	"net/http"
	"strings"
)

// Explain says in a sentence what went wrong with the model and what to do, for the window. The raw error goes to the log.
func Explain(err error) string {
	text := err.Error()
	var status *StatusError
	switch {
	case errors.Is(err, ErrTooLong):
		return ErrTooLong.Error()
	case strings.Contains(text, "not installed"):
		return "Omni AI is not installed. Turn it on in Settings."
	case outOfMemory(err):
		return "The graphics card ran out of memory or stopped. Close games or other programs that use it and ask again; Omni runs the model on the processor if it keeps happening."
	case errors.As(err, &status) && status.Status == http.StatusBadRequest && strings.Contains(strings.ToLower(status.Message), "context"):
		return "This conversation no longer fits in the model's memory. Start a new chat, or raise the context size in Settings."
	case connectionLost(err):
		return "The model stopped answering. Ask again and Omni starts it again."
	case strings.Contains(text, "not ready after"), strings.Contains(text, "stopped while starting"), strings.Contains(text, "start the model server"):
		return "The model did not start. The details are in the log (Settings, About)."
	}
	return "Something went wrong with the model. The details are in the log (Settings, About)."
}

// outOfMemory reports an error that says the graphics card ran out of memory or was lost.
func outOfMemory(err error) bool {
	text := err.Error()
	return strings.Contains(text, "ErrorOutOfDeviceMemory") || strings.Contains(text, "ErrorDeviceLost") ||
		strings.Contains(strings.ToLower(text), "out of device memory")
}

// connectionLost reports an error that says the server went away.
func connectionLost(err error) bool {
	text := err.Error()
	return strings.Contains(text, "connection refused") || strings.Contains(text, "forcibly closed") ||
		strings.Contains(text, "EOF") || strings.Contains(text, "connectex")
}
