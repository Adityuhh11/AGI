package panel

import (
	"encoding/binary"
	"unsafe"

	"golang.org/x/sys/windows"
)

// The panel is frosted glass: under its plate the page shows a blurred picture of what lies behind the
// window. The web view can only blur its own content, so the picture is taken here, just before the
// panel appears (while it is still cloaked, so it is not in the picture).

var (
	gdi32 = windows.NewLazySystemDLL("gdi32.dll")

	procGetDC                  = user32.NewProc("GetDC")
	procReleaseDC              = user32.NewProc("ReleaseDC")
	procCreateCompatibleDC     = gdi32.NewProc("CreateCompatibleDC")
	procCreateCompatibleBitmap = gdi32.NewProc("CreateCompatibleBitmap")
	procSelectObject           = gdi32.NewProc("SelectObject")
	procDeleteObject           = gdi32.NewProc("DeleteObject")
	procDeleteDC               = gdi32.NewProc("DeleteDC")
	procSetStretchBltMode      = gdi32.NewProc("SetStretchBltMode")
	procStretchBlt             = gdi32.NewProc("StretchBlt")
	procGetDIBits              = gdi32.NewProc("GetDIBits")
)

const (
	srcCopy  = 0x00CC0020
	halftone = 3
	// backdropScale shrinks the picture: it is blurred by tens of pixels anyway, and a fraction of
	// the pixels is a fraction of the time to copy, convert and decode them.
	backdropScale = 4
	bmpHeaderSize = 14 + 40
)

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

// snapshot returns the screen area r (physical pixels, virtual-screen coordinates) shrunk by
// backdropScale, as a 24-bit BMP file; nil if the screen cannot be read.
func snapshot(r Rect) []byte {
	w, h := max(r.W/backdropScale, 1), max(r.H/backdropScale, 1)
	screen, _, _ := procGetDC.Call(0)
	if screen == 0 {
		return nil
	}
	defer procReleaseDC.Call(0, screen)
	mem, _, _ := procCreateCompatibleDC.Call(screen)
	if mem == 0 {
		return nil
	}
	defer procDeleteDC.Call(mem)
	bitmap, _, _ := procCreateCompatibleBitmap.Call(screen, uintptr(w), uintptr(h))
	if bitmap == 0 {
		return nil
	}
	defer procDeleteObject.Call(bitmap)
	old, _, _ := procSelectObject.Call(mem, bitmap)
	defer procSelectObject.Call(mem, old)

	procSetStretchBltMode.Call(mem, halftone)
	ok, _, _ := procStretchBlt.Call(mem, 0, 0, uintptr(w), uintptr(h),
		screen, uintptr(int32(r.X)), uintptr(int32(r.Y)), uintptr(r.W), uintptr(r.H), srcCopy)
	if ok == 0 {
		return nil
	}

	stride := (w*3 + 3) &^ 3 // BMP rows are padded to four bytes
	header := bitmapInfoHeader{
		Size: 40, Width: int32(w), Height: -int32(h), // a negative height is a top-down image
		Planes: 1, BitCount: 24, SizeImage: uint32(stride * h),
	}
	file := make([]byte, bmpHeaderSize+stride*h)
	file[0], file[1] = 'B', 'M'
	binary.LittleEndian.PutUint32(file[2:], uint32(len(file)))
	binary.LittleEndian.PutUint32(file[10:], bmpHeaderSize)
	copy(file[14:bmpHeaderSize], (*[40]byte)(unsafe.Pointer(&header))[:])
	lines, _, _ := procGetDIBits.Call(mem, bitmap, 0, uintptr(h), uintptr(unsafe.Pointer(&file[bmpHeaderSize])),
		uintptr(unsafe.Pointer(&header)), 0)
	if lines == 0 {
		return nil
	}
	return file
}
