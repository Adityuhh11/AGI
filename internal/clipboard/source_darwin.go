package clipboard

/*
#cgo CFLAGS: -x objective-c -fobjc-arc
#cgo LDFLAGS: -framework Cocoa -framework Carbon -framework ApplicationServices
#include <stdlib.h>
#include "pasteboard_darwin.h"
*/
import "C"

import (
	"os"
	"path/filepath"
	"sync/atomic"
	"unicode/utf8"
	"unsafe"
)

// The Mac clipboard is NSPasteboard (pasteboard_darwin.m). Its change counter is the cheap thing the watcher polls, as Windows' sequence number is.

// ownCount is the change counter after this program's own last copy. A Mac does not say who copied, so a copy this program made (a paste,
// "copy path") is known by its counter, and its owner is reported as this program: the watcher then leaves it out of the history.
var ownCount atomic.Int64

// selfName is what the watcher is told this program is called (cmd/omni passes the executable's name without ".exe").
var selfName = func() string {
	exe, err := os.Executable()
	if err != nil {
		return "omni"
	}
	return filepath.Base(exe)
}()

// NoteOwnCopy tells that this program has just written to the clipboard. Call it right after the write.
func NoteOwnCopy() { ownCount.Store(int64(C.omniPasteboardCount())) }

// NewSource is the clipboard as the history sees it.
func NewSource() Source { return pasteboard{} }

type pasteboard struct{}

func (pasteboard) Sequence() uint32 { return uint32(C.omniPasteboardCount()) }

// Read returns the text on the clipboard. Who copied it is the application that has the keyboard now, which is right for a copy made by hand (the
// watcher looks every 250 ms) and wrong for one a background tool made; it feeds the list of programs whose copies are never kept (password managers),
// most of which mark what they copy as private as well, which is honoured separately.
func (pasteboard) Read() (Clip, error) {
	count := int64(C.omniPasteboardCount())
	text := C.omniPasteboardString()
	if text == nil {
		return Clip{}, nil // not text
	}
	defer C.free(unsafe.Pointer(text))
	s := C.GoString(text)
	if utf8.RuneCountInString(s) > MaxText {
		return Clip{}, ErrTooLong
	}
	clip := Clip{Text: s, Private: C.omniPasteboardPrivate() != 0}
	if count == ownCount.Load() {
		clip.Owner = selfName
		return clip, nil
	}
	if name := C.omniFrontmostName(); name != nil {
		clip.Owner = C.GoString(name)
		C.free(unsafe.Pointer(name))
	}
	return clip, nil
}
