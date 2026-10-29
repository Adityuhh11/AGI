package watcher

/*
#cgo CFLAGS: -x objective-c -fobjc-arc
#cgo LDFLAGS: -framework CoreServices -framework Foundation
#include <stdlib.h>
#include "watch_darwin.h"
*/
import "C"

import (
	"context"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"time"
	"unsafe"
)

// FSEvents watches whole trees through one stream, the Mac's counterpart of ReadDirectoryChangesW (watch_windows.go). Notes on how it differs:
// its paths are real paths (a folder reached through a link is reported under the link's target), so each root is matched by its resolved path and
// reported under the name it was given; and it can say "too many events, look at everything" (a flag), which pokes the whole root, as an overflow does there.

// The flags of an event that say anything under the folder may have changed: the events were merged, or some were lost.
const mustScan = 0x1 | 0x2 | 0x4 // MustScanSubDirs, UserDropped, KernelDropped

// latencySeconds is how long FSEvents gathers events before it reports them; the debouncer then waits for quiet.
const latencySeconds = 0.5

type watched struct {
	real, name string // the resolved path events carry, and the root as it was given
}

// stream is one call of Watch: where its events go.
type stream struct {
	roots []watched
	fire  *debouncer
}

var (
	streams    sync.Map // handle -> *stream
	lastHandle atomic.Int64
)

//export goWatchEvent
func goWatchEvent(handle C.long, path *C.char, flags C.uint) {
	if s, ok := streams.Load(int64(handle)); ok {
		s.(*stream).event(C.GoString(path), uint32(flags))
	}
}

// event decides whether a change at path is one that matters, and pokes the root it is under.
func (s *stream) event(path string, flags uint32) {
	for _, root := range s.roots {
		rel, ok := strings.CutPrefix(path, root.real)
		if !ok || (rel != "" && rel[0] != filepath.Separator) {
			continue // another folder that merely starts with the same letters
		}
		if flags&mustScan != 0 || relevant(rel) {
			s.fire.poke(root.name)
		}
		return
	}
}

// Watch calls changed(root) for each folder in roots whenever something relevant under it has changed and then been quiet for quiet (or after
// longest, if it never goes quiet). It blocks until ctx ends. A folder that cannot be watched is logged and left out; the others still work.
func Watch(ctx context.Context, roots []string, quiet, longest time.Duration, changed func(root string)) {
	fire := newDebouncer(quiet, longest, changed)
	defer fire.stop()
	s := &stream{fire: fire}
	var paths []string
	for _, root := range roots {
		real, err := filepath.EvalSymlinks(root)
		if err == nil {
			_, err = os.Stat(real)
		}
		if err != nil {
			log.Printf("watcher: cannot watch %s: %v", root, err)
			continue
		}
		s.roots = append(s.roots, watched{real: real, name: root})
		paths = append(paths, real)
	}
	if len(paths) == 0 {
		<-ctx.Done()
		return
	}
	handle := lastHandle.Add(1)
	streams.Store(handle, s)
	defer streams.Delete(handle)

	cPaths := make([]*C.char, len(paths))
	for i, p := range paths {
		cPaths[i] = C.CString(p)
	}
	ref := C.omniWatchStart(C.long(handle), (**C.char)(unsafe.Pointer(&cPaths[0])), C.int(len(cPaths)), C.double(latencySeconds))
	for _, p := range cPaths {
		C.free(unsafe.Pointer(p)) // FSEvents keeps its own copy
	}
	if ref == nil {
		log.Printf("watcher: the system would not watch %d folders", len(paths))
		<-ctx.Done()
		return
	}
	<-ctx.Done()
	C.omniWatchStop(ref)
}
