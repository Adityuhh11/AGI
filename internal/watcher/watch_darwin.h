#ifndef OMNI_WATCHER_H
#define OMNI_WATCHER_H

// Starts an FSEvents stream over the folders (file-level events, the latency in seconds) and returns it, or NULL when it could not be started.
// Every event is handed to goWatchEvent with the handle, the path and the stream's flags.
void *omniWatchStart(long handle, char **paths, int count, double latency);
void omniWatchStop(void *stream);

// Implemented in Go (watch_darwin.go).
extern void goWatchEvent(long handle, char *path, unsigned int flags);

#endif
