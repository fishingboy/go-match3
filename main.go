//go:build windows

// Command go-match3 is a match-3 game rendered without any game engine:
// a raw Win32 window, a hand-rolled message pump, GDI double buffering,
// and a fixed-step timer driving the animation state machine in app.go.
//
// The terminal version lives in cmd/cli.
package main

import (
	"math/rand"
	"runtime"
	"syscall"
	"time"
	"unsafe"
)

var (
	app      *App
	lastTick time.Time
)

func main() {
	// Win32 windows are bound to the thread that created them; the
	// message loop must stay on that same OS thread.
	runtime.LockOSThread()

	app = newApp(rand.New(rand.NewSource(time.Now().UnixNano())))
	lastTick = time.Now()

	hInst, _, _ := pGetModuleHandleW.Call(0)
	className, _ := syscall.UTF16PtrFromString("GoMatch3Window")
	title, _ := syscall.UTF16PtrFromString("Go Match-3 (no engine, raw Win32/GDI)")
	cursor, _, _ := pLoadCursorW.Call(0, idcArrow)

	wc := wndClassEx{
		cbSize:        uint32(unsafe.Sizeof(wndClassEx{})),
		style:         csHRedraw | csVRedraw,
		lpfnWndProc:   syscall.NewCallback(wndProc),
		hInstance:     hInst,
		hCursor:       cursor,
		lpszClassName: className,
	}
	if r, _, err := pRegisterClassExW.Call(uintptr(unsafe.Pointer(&wc))); r == 0 {
		panic("RegisterClassExW failed: " + err.Error())
	}

	// Fixed-size window: grow the outer rect so the *client* area is
	// exactly the board plus HUD.
	style := uintptr(wsCaption | wsSysMenu | wsMinimizeBox)
	rc := rect{0, 0, winW, winH}
	pAdjustWindowRect.Call(uintptr(unsafe.Pointer(&rc)), style, 0)

	hwnd, _, err := pCreateWindowExW.Call(0,
		uintptr(unsafe.Pointer(className)),
		uintptr(unsafe.Pointer(title)),
		style|wsVisible,
		cwUseDefault, cwUseDefault,
		uintptr(rc.right-rc.left), uintptr(rc.bottom-rc.top),
		0, 0, hInst, 0)
	if hwnd == 0 {
		panic("CreateWindowExW failed: " + err.Error())
	}

	// ~60 FPS tick; WM_TIMER drives update() + InvalidateRect.
	pSetTimer.Call(hwnd, 1, 16, 0)

	var m msgW
	for {
		r, _, _ := pGetMessageW.Call(uintptr(unsafe.Pointer(&m)), 0, 0, 0)
		if int32(r) <= 0 {
			return
		}
		pTranslateMessage.Call(uintptr(unsafe.Pointer(&m)))
		pDispatchMessageW.Call(uintptr(unsafe.Pointer(&m)))
	}
}

func wndProc(hwnd, msg, wParam, lParam uintptr) uintptr {
	switch msg {
	case wmTimer:
		now := time.Now()
		dt := now.Sub(lastTick).Seconds()
		lastTick = now
		if dt > 0.1 {
			dt = 0.1 // clamp after stalls (drag, sleep) so animations don't jump
		}
		app.update(dt)
		pInvalidateRect.Call(hwnd, 0, 0)

	case wmPaint:
		onPaint(hwnd)

	case wmEraseBkgnd:
		return 1 // we repaint every pixel; skipping the erase avoids flicker

	case wmLButtonDown:
		x := int(int16(lParam & 0xFFFF))
		y := int(int16((lParam >> 16) & 0xFFFF))
		app.click(x, y)

	case wmKeyDown:
		switch wParam {
		case vkEscape:
			pPostQuitMessage.Call(0)
		case vkR:
			app.reset()
		}

	case wmDestroy:
		pPostQuitMessage.Call(0)

	default:
		r, _, _ := pDefWindowProcW.Call(hwnd, msg, wParam, lParam)
		return r
	}
	return 0
}

// onPaint draws the frame into an off-screen bitmap and blits it in one
// go — GDI double buffering by hand.
func onPaint(hwnd uintptr) {
	var ps paintStruct
	hdc, _, _ := pBeginPaint.Call(hwnd, uintptr(unsafe.Pointer(&ps)))

	var rc rect
	pGetClientRect.Call(hwnd, uintptr(unsafe.Pointer(&rc)))
	w, h := int(rc.right), int(rc.bottom)

	memDC, _, _ := pCreateCompatibleDC.Call(hdc)
	bmp, _, _ := pCreateCompatibleBitmap.Call(hdc, uintptr(w), uintptr(h))
	oldBmp, _, _ := pSelectObject.Call(memDC, bmp)

	app.draw(memDC)
	pBitBlt.Call(hdc, 0, 0, uintptr(w), uintptr(h), memDC, 0, 0, srcCopy)

	pSelectObject.Call(memDC, oldBmp)
	pDeleteObject.Call(bmp)
	pDeleteDC.Call(memDC)
	pEndPaint.Call(hwnd, uintptr(unsafe.Pointer(&ps)))
}
