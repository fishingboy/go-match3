//go:build windows

// Win32 API bindings via syscall — this file is the price of "no game
// engine": window class registration, message pump structures, and raw
// GDI drawing primitives, all loaded from user32/gdi32 by hand.
package main

import (
	"syscall"
	"unsafe"
)

var (
	user32   = syscall.NewLazyDLL("user32.dll")
	gdi32    = syscall.NewLazyDLL("gdi32.dll")
	kernel32 = syscall.NewLazyDLL("kernel32.dll")

	pRegisterClassExW = user32.NewProc("RegisterClassExW")
	pCreateWindowExW  = user32.NewProc("CreateWindowExW")
	pDefWindowProcW   = user32.NewProc("DefWindowProcW")
	pGetMessageW      = user32.NewProc("GetMessageW")
	pTranslateMessage = user32.NewProc("TranslateMessage")
	pDispatchMessageW = user32.NewProc("DispatchMessageW")
	pPostQuitMessage  = user32.NewProc("PostQuitMessage")
	pInvalidateRect   = user32.NewProc("InvalidateRect")
	pBeginPaint       = user32.NewProc("BeginPaint")
	pEndPaint         = user32.NewProc("EndPaint")
	pGetClientRect    = user32.NewProc("GetClientRect")
	pAdjustWindowRect = user32.NewProc("AdjustWindowRect")
	pSetTimer         = user32.NewProc("SetTimer")
	pLoadCursorW      = user32.NewProc("LoadCursorW")
	pFillRect         = user32.NewProc("FillRect")

	pGetModuleHandleW = kernel32.NewProc("GetModuleHandleW")

	pCreateSolidBrush       = gdi32.NewProc("CreateSolidBrush")
	pCreatePen              = gdi32.NewProc("CreatePen")
	pDeleteObject           = gdi32.NewProc("DeleteObject")
	pSelectObject           = gdi32.NewProc("SelectObject")
	pCreateCompatibleDC     = gdi32.NewProc("CreateCompatibleDC")
	pCreateCompatibleBitmap = gdi32.NewProc("CreateCompatibleBitmap")
	pDeleteDC               = gdi32.NewProc("DeleteDC")
	pBitBlt                 = gdi32.NewProc("BitBlt")
	pEllipse                = gdi32.NewProc("Ellipse")
	pPolygon                = gdi32.NewProc("Polygon")
	pRoundRect              = gdi32.NewProc("RoundRect")
	pSetBkMode              = gdi32.NewProc("SetBkMode")
	pSetTextColor           = gdi32.NewProc("SetTextColor")
	pTextOutW               = gdi32.NewProc("TextOutW")
	pCreateFontW            = gdi32.NewProc("CreateFontW")
)

const (
	csHRedraw = 0x0002
	csVRedraw = 0x0001

	wsCaption     = 0x00C00000
	wsSysMenu     = 0x00080000
	wsMinimizeBox = 0x00020000
	wsVisible     = 0x10000000
	cwUseDefault  = 0x80000000

	wmDestroy     = 0x0002
	wmPaint       = 0x000F
	wmEraseBkgnd  = 0x0014
	wmKeyDown     = 0x0100
	wmTimer       = 0x0113
	wmLButtonDown = 0x0201

	vkEscape = 0x1B
	vkR      = 0x52

	idcArrow    = 32512
	srcCopy     = 0x00CC0020
	bkModeClear = 1 // TRANSPARENT

	fwSemibold      = 600
	cleartypeQuality = 5
)

type point struct {
	x, y int32
}

type rect struct {
	left, top, right, bottom int32
}

type msgW struct {
	hwnd    uintptr
	message uint32
	wParam  uintptr
	lParam  uintptr
	time    uint32
	pt      point
}

type wndClassEx struct {
	cbSize        uint32
	style         uint32
	lpfnWndProc   uintptr
	cbClsExtra    int32
	cbWndExtra    int32
	hInstance     uintptr
	hIcon         uintptr
	hCursor       uintptr
	hbrBackground uintptr
	lpszMenuName  *uint16
	lpszClassName *uint16
	hIconSm       uintptr
}

type paintStruct struct {
	hdc         uintptr
	fErase      int32
	rcPaint     rect
	fRestore    int32
	fIncUpdate  int32
	rgbReserved [32]byte
}

// rgb packs a color into a GDI COLORREF (0x00BBGGRR).
func rgb(r, g, b uint32) uint32 {
	return r | g<<8 | b<<16
}

// darken halves each channel of a COLORREF, used for gem outlines.
func darken(col uint32) uint32 {
	return rgb((col&0xFF)/2, (col>>8&0xFF)/2, (col>>16&0xFF)/2)
}

// GDI objects live in the OS, not the Go heap; cache them per color
// instead of creating and destroying hundreds per frame.
var (
	brushCache = map[uint32]uintptr{}
	penCache   = map[uint32]uintptr{}
	hudFont    uintptr
)

func brush(col uint32) uintptr {
	if h, ok := brushCache[col]; ok {
		return h
	}
	h, _, _ := pCreateSolidBrush.Call(uintptr(col))
	brushCache[col] = h
	return h
}

func pen(col uint32, width int) uintptr {
	if h, ok := penCache[col]; ok {
		return h
	}
	h, _, _ := pCreatePen.Call(0 /* PS_SOLID */, uintptr(width), uintptr(col))
	penCache[col] = h
	return h
}

func font() uintptr {
	if hudFont != 0 {
		return hudFont
	}
	face, _ := syscall.UTF16PtrFromString("Segoe UI")
	height := -23 // negative selects character height in logical units
	hudFont, _, _ = pCreateFontW.Call(
		uintptr(height),
		0, 0, 0, fwSemibold, 0, 0, 0, 0, 0, 0,
		cleartypeQuality, 0, uintptr(unsafe.Pointer(face)))
	return hudFont
}

func fillRectPx(dc uintptr, x, y, w, h int, col uint32) {
	rc := rect{int32(x), int32(y), int32(x + w), int32(y + h)}
	pFillRect.Call(dc, uintptr(unsafe.Pointer(&rc)), brush(col))
}

func selectBrushPen(dc uintptr, col uint32) {
	pSelectObject.Call(dc, brush(col))
	pSelectObject.Call(dc, pen(darken(col), 2))
}

func drawEllipse(dc uintptr, x0, y0, x1, y1 int) {
	pEllipse.Call(dc, uintptr(x0), uintptr(y0), uintptr(x1), uintptr(y1))
}

func drawRoundRect(dc uintptr, x0, y0, x1, y1, r int) {
	pRoundRect.Call(dc, uintptr(x0), uintptr(y0), uintptr(x1), uintptr(y1), uintptr(r), uintptr(r))
}

func drawPolygon(dc uintptr, pts []point) {
	pPolygon.Call(dc, uintptr(unsafe.Pointer(&pts[0])), uintptr(len(pts)))
}

func drawText(dc uintptr, x, y int, col uint32, s string) {
	u, err := syscall.UTF16FromString(s)
	if err != nil {
		return
	}
	pSelectObject.Call(dc, font())
	pSetBkMode.Call(dc, bkModeClear)
	pSetTextColor.Call(dc, uintptr(col))
	pTextOutW.Call(dc, uintptr(x), uintptr(y), uintptr(unsafe.Pointer(&u[0])), uintptr(len(u)-1))
}
