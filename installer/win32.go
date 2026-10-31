package main

import (
	"syscall"
	"unsafe"
)

var (
	user32   = syscall.NewLazyDLL("user32.dll")
	gdi32    = syscall.NewLazyDLL("gdi32.dll")
	kernel32 = syscall.NewLazyDLL("kernel32.dll")
	msimg32  = syscall.NewLazyDLL("msimg32.dll")
	shell32  = syscall.NewLazyDLL("shell32.dll")
	dwmapi   = syscall.NewLazyDLL("dwmapi.dll")

	pRegisterClassExW         = user32.NewProc("RegisterClassExW")
	pCreateWindowExW          = user32.NewProc("CreateWindowExW")
	pDefWindowProcW           = user32.NewProc("DefWindowProcW")
	pGetMessageW              = user32.NewProc("GetMessageW")
	pTranslateMessage         = user32.NewProc("TranslateMessage")
	pDispatchMessageW         = user32.NewProc("DispatchMessageW")
	pPostQuitMessage          = user32.NewProc("PostQuitMessage")
	pBeginPaint               = user32.NewProc("BeginPaint")
	pEndPaint                 = user32.NewProc("EndPaint")
	pInvalidateRect           = user32.NewProc("InvalidateRect")
	pGetClientRect            = user32.NewProc("GetClientRect")
	pSetTimer                 = user32.NewProc("SetTimer")
	pLoadCursorW              = user32.NewProc("LoadCursorW")
	pSetCursor                = user32.NewProc("SetCursor")
	pMessageBoxW              = user32.NewProc("MessageBoxW")
	pGetDpiForWindow          = user32.NewProc("GetDpiForWindow")
	pSetWindowPos             = user32.NewProc("SetWindowPos")
	pShowWindow               = user32.NewProc("ShowWindow")
	pUpdateWindow             = user32.NewProc("UpdateWindow")
	pGetSystemMetrics         = user32.NewProc("GetSystemMetrics")
	pFillRect                 = user32.NewProc("FillRect")
	pDrawTextW                = user32.NewProc("DrawTextW")
	pDestroyWindow            = user32.NewProc("DestroyWindow")
	pAdjustWindowRectEx       = user32.NewProc("AdjustWindowRectEx")
	pSetProcessDpiAwarenessCx = user32.NewProc("SetProcessDpiAwarenessContext")
	pLoadIconW                = user32.NewProc("LoadIconW")
	pTrackMouseEvent          = user32.NewProc("TrackMouseEvent")
	pPostMessageW             = user32.NewProc("PostMessageW")

	pCreateCompatibleDC     = gdi32.NewProc("CreateCompatibleDC")
	pCreateCompatibleBitmap = gdi32.NewProc("CreateCompatibleBitmap")
	pSelectObject           = gdi32.NewProc("SelectObject")
	pBitBlt                 = gdi32.NewProc("BitBlt")
	pDeleteObject           = gdi32.NewProc("DeleteObject")
	pDeleteDC               = gdi32.NewProc("DeleteDC")
	pCreateSolidBrush       = gdi32.NewProc("CreateSolidBrush")
	pCreateFontW            = gdi32.NewProc("CreateFontW")
	pSetTextColor           = gdi32.NewProc("SetTextColor")
	pSetBkMode              = gdi32.NewProc("SetBkMode")
	pCreatePen              = gdi32.NewProc("CreatePen")
	pRoundRect              = gdi32.NewProc("RoundRect")
	pCreateDIBSection       = gdi32.NewProc("CreateDIBSection")
	pGetStockObject         = gdi32.NewProc("GetStockObject")
	pEllipse                = gdi32.NewProc("Ellipse")
	pMoveToEx               = gdi32.NewProc("MoveToEx")
	pLineTo                 = gdi32.NewProc("LineTo")

	pGetModuleHandleW = kernel32.NewProc("GetModuleHandleW")
	pAlphaBlend       = msimg32.NewProc("AlphaBlend")
	pShellExecuteW    = shell32.NewProc("ShellExecuteW")
	pDwmSetWindowAttr = dwmapi.NewProc("DwmSetWindowAttribute")
)

const (
	WM_DESTROY      = 0x0002
	WM_SIZE         = 0x0005
	WM_PAINT        = 0x000F
	WM_CLOSE        = 0x0010
	WM_ERASEBKGND   = 0x0014
	WM_SETCURSOR    = 0x0020
	WM_KEYDOWN      = 0x0100
	WM_TIMER        = 0x0113
	WM_MOUSEMOVE    = 0x0200
	WM_LBUTTONDOWN  = 0x0201
	WM_LBUTTONUP    = 0x0202
	WM_MOUSELEAVE   = 0x02A3
	WM_DPICHANGED   = 0x02E0
	WM_APP          = 0x8000
	WS_OVERLAPPED   = 0x00000000
	WS_CAPTION      = 0x00C00000
	WS_SYSMENU      = 0x00080000
	WS_MINIMIZEBOX  = 0x00020000
	WS_VISIBLE      = 0x10000000
	CS_HREDRAW      = 0x0002
	CS_VREDRAW      = 0x0001
	IDC_ARROW       = 32512
	IDC_HAND        = 32649
	SRCCOPY         = 0x00CC0020
	DT_LEFT         = 0x0
	DT_CENTER       = 0x1
	DT_RIGHT        = 0x2
	DT_VCENTER      = 0x4
	DT_WORDBREAK    = 0x10
	DT_SINGLELINE   = 0x20
	DT_NOPREFIX     = 0x800
	DT_CALCRECT     = 0x400
	DT_END_ELLIPSIS = 0x8000
	MB_OK           = 0x0
	MB_YESNO        = 0x4
	MB_ICONWARNING  = 0x30
	MB_ICONERROR    = 0x10
	MB_ICONQUESTION = 0x20
	MB_RETRYCANCEL  = 0x5
	IDYES           = 6
	IDRETRY         = 4
	VK_RETURN       = 0x0D
	VK_ESCAPE       = 0x1B
	SWP_NOZORDER    = 0x0004
	SWP_NOACTIVATE  = 0x0010
	TRANSPARENT     = 1
	NULL_BRUSH      = 5
	NULL_PEN        = 8
	TME_LEAVE       = 0x2
)

type WNDCLASSEX struct {
	Size       uint32
	Style      uint32
	WndProc    uintptr
	ClsExtra   int32
	WndExtra   int32
	Instance   uintptr
	Icon       uintptr
	Cursor     uintptr
	Background uintptr
	MenuName   *uint16
	ClassName  *uint16
	IconSm     uintptr
}

type POINT struct{ X, Y int32 }
type RECT struct{ Left, Top, Right, Bottom int32 }
type MSG struct {
	Hwnd    uintptr
	Message uint32
	WParam  uintptr
	LParam  uintptr
	Time    uint32
	Pt      POINT
}
type PAINTSTRUCT struct {
	Hdc         uintptr
	Erase       int32
	RcPaint     RECT
	Restore     int32
	IncUpdate   int32
	RgbReserved [32]byte
}
type BITMAPINFOHEADER struct {
	Size          uint32
	Width         int32
	Height        int32
	Planes        uint16
	BitCount      uint16
	Compression   uint32
	SizeImage     uint32
	XPelsPerMeter int32
	YPelsPerMeter int32
	ClrUsed       uint32
	ClrImportant  uint32
}
type TRACKMOUSEEVENT struct {
	Size      uint32
	Flags     uint32
	Hwnd      uintptr
	HoverTime uint32
}

func u16(s string) *uint16 { p, _ := syscall.UTF16PtrFromString(s); return p }

func rgb(r, g, b byte) uint32 { return uint32(r) | uint32(g)<<8 | uint32(b)<<16 }

func msgBox(hwnd uintptr, text, title string, flags uint32) int {
	r, _, _ := pMessageBoxW.Call(hwnd, uintptr(unsafe.Pointer(u16(text))), uintptr(unsafe.Pointer(u16(title))), uintptr(flags))
	return int(r)
}

func openURL(u string) {
	pShellExecuteW.Call(0, uintptr(unsafe.Pointer(u16("open"))), uintptr(unsafe.Pointer(u16(u))), 0, 0, 1)
}
