package apps

/*
#cgo CFLAGS: -x objective-c -fobjc-arc
#cgo LDFLAGS: -framework Cocoa
#include <stdlib.h>
#include "icon_darwin.h"
*/
import "C"

import (
	"errors"
	"unsafe"
)

// extractIcon is the icon Finder shows for the app's bundle, as a PNG. NSWorkspace gives the icon at every size the bundle has; it is drawn at
// iconSize pixels. It is called from a background goroutine, which is fine: it draws into a bitmap, not a window.
func extractIcon(id string) ([]byte, error) {
	path := C.CString(id)
	defer C.free(unsafe.Pointer(path))
	var length C.int
	data := C.omniIconPNG(path, C.int(iconSize), &length)
	if data == nil {
		return nil, errors.New("the system has no icon for it")
	}
	defer C.free(unsafe.Pointer(data))
	return C.GoBytes(unsafe.Pointer(data), length), nil
}
