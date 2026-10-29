package apps

import (
	"bytes"
	"errors"
	"fmt"
	"image"
	"image/png"
	"runtime"
	"sync"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"
)

var (
	shell32 = windows.NewLazySystemDLL("shell32.dll")
	gdi32   = windows.NewLazySystemDLL("gdi32.dll")

	procSHCreateItemFromParsingName = shell32.NewProc("SHCreateItemFromParsingName")
	procGetObjectW                  = gdi32.NewProc("GetObjectW")
	procGetDIBits                   = gdi32.NewProc("GetDIBits")
	procDeleteObject                = gdi32.NewProc("DeleteObject")
	procCreateCompatibleDC          = gdi32.NewProc("CreateCompatibleDC")
	procDeleteDC                    = gdi32.NewProc("DeleteDC")

	// IID_IShellItemImageFactory
	iidImageFactory = windows.GUID{Data1: 0xbcc18b79, Data2: 0xba16, Data3: 0x442f, Data4: [8]byte{0x80, 0xc4, 0x8a, 0x59, 0xc3, 0x0c, 0x46, 0x3b}}
)

const (
	siigbfIconOnly = 0x04 // the icon, never a thumbnail of the file's content
	vtableRelease  = 2
	vtableGetImage = 3
)

type bitmap struct {
	Type, Width, Height, WidthBytes int32
	Planes, BitsPixel               uint16
	Bits                            uintptr
}

type bitmapInfoHeader struct {
	Size          uint32
	Width, Height int32
	Planes        uint16
	BitCount      uint16
	Compression   uint32
	SizeImage     uint32
	XPelsPerMeter int32
	YPelsPerMeter int32
	ColorsUsed    uint32
	ColorsInUse   uint32
}

// iconMutex serialises extraction: each call needs its own COM apartment on a locked thread, and ten rows
// of fresh apps should not take every core.
var iconMutex sync.Mutex

// extractIcon asks the shell for the icon of a Start Menu entry, as a PNG.
func extractIcon(id string) ([]byte, error) {
	iconMutex.Lock()
	defer iconMutex.Unlock()
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	if err := windows.CoInitializeEx(0, windows.COINIT_APARTMENTTHREADED); err != nil {
		return nil, fmt.Errorf("start COM: %w", err)
	}
	defer windows.CoUninitialize()

	name, err := windows.UTF16PtrFromString(resolve(id).run) // the Start Menu entry itself: Store apps, programs and shortcuts all come out the same way
	if err != nil {
		return nil, err
	}
	var factory unsafe.Pointer // a COM object: its first word points at its table of methods
	hr, _, _ := procSHCreateItemFromParsingName.Call(uintptr(unsafe.Pointer(name)), 0,
		uintptr(unsafe.Pointer(&iidImageFactory)), uintptr(unsafe.Pointer(&factory)))
	if hr != 0 || factory == nil {
		return nil, fmt.Errorf("the shell has no item for it (0x%x)", uint32(hr))
	}
	vtable := (*[4]uintptr)(*(*unsafe.Pointer)(factory))
	object := uintptr(factory)
	defer syscall.SyscallN(vtable[vtableRelease], object)

	var hbitmap uintptr
	// A SIZE travels by value, both halves in one register.
	size := uintptr(uint32(iconSize)) | uintptr(uint32(iconSize))<<32
	hr, _, _ = syscall.SyscallN(vtable[vtableGetImage], object, size, siigbfIconOnly, uintptr(unsafe.Pointer(&hbitmap)))
	if hr != 0 || hbitmap == 0 {
		return nil, fmt.Errorf("the shell has no icon for it (0x%x)", uint32(hr))
	}
	defer procDeleteObject.Call(hbitmap)
	return encode(hbitmap)
}

// encode turns the shell's bitmap into a PNG.
func encode(hbitmap uintptr) ([]byte, error) {
	var bm bitmap
	if r, _, _ := procGetObjectW.Call(hbitmap, unsafe.Sizeof(bm), uintptr(unsafe.Pointer(&bm))); r == 0 {
		return nil, errors.New("read the icon bitmap")
	}
	w, h := int(bm.Width), int(bm.Height)
	dc, _, _ := procCreateCompatibleDC.Call(0)
	if dc == 0 {
		return nil, errors.New("create a device context")
	}
	defer procDeleteDC.Call(dc)
	header := bitmapInfoHeader{Size: uint32(unsafe.Sizeof(bitmapInfoHeader{})), Width: int32(w), Height: -int32(h), Planes: 1, BitCount: 32}
	pixels := make([]byte, w*h*4)
	if lines, _, _ := procGetDIBits.Call(dc, hbitmap, 0, uintptr(h), uintptr(unsafe.Pointer(&pixels[0])),
		uintptr(unsafe.Pointer(&header)), 0); lines == 0 {
		return nil, errors.New("copy the icon pixels")
	}
	img := image.NewNRGBA(image.Rect(0, 0, w, h))
	opaque := true // an icon without an alpha channel comes with every alpha zero
	for i := 0; i < len(pixels); i += 4 {
		if pixels[i+3] != 0 {
			opaque = false
			break
		}
	}
	for i := 0; i < len(pixels); i += 4 {
		a := pixels[i+3]
		if opaque {
			a = 255
		}
		img.Pix[i], img.Pix[i+1], img.Pix[i+2], img.Pix[i+3] = pixels[i+2], pixels[i+1], pixels[i], a // BGRA to RGBA
	}
	var out bytes.Buffer
	if err := png.Encode(&out, img); err != nil {
		return nil, err
	}
	return out.Bytes(), nil
}
