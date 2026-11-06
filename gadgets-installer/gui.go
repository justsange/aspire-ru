package main

import (
	"bytes"
	_ "embed"
	"image"
	"image/png"
	"runtime"
	"sync"
	"time"
	"unsafe"
)

//go:embed logo.png
var logoPNG []byte

func init() { runtime.LockOSThread() }

// ---------- палитра aa-forma.ru ----------
var (
	cBg      = rgb(0x15, 0x15, 0x15)
	cSide    = rgb(0x0e, 0x0e, 0x0e)
	cPanel   = rgb(0x1e, 0x1e, 0x1d)
	cText    = rgb(0xf0, 0xf0, 0xec)
	cMuted   = rgb(0xaf, 0xaf, 0xab)
	cDim     = rgb(0x6e, 0x6e, 0x6a)
	cLine    = rgb(0x3c, 0x3c, 0x39)
	cAccent  = rgb(0xed, 0x69, 0x3f)
	cAccentH = rgb(0xf4, 0x84, 0x5f)
	cOk      = rgb(0x86, 0xc9, 0x8e)
	cDark    = rgb(0x15, 0x15, 0x15)
)

const (
	LW = 900 // логическая ширина клиентской области
	LH = 640
	SB = 260 // ширина боковой панели
)

type task struct {
	title string
	min   time.Duration
	fn    func() error
}

type hit struct {
	id string
	r  RECT
}

type App struct {
	hwnd    uintptr
	dpi     int
	page    int // 0 приветствие, 1 выбор гаджетов, 2 установка, 3 готово
	mode    string
	dirs    []string
	opt     map[string]bool // какие гаджеты ставим (по id)
	launch  bool
	hover   string
	hits    []hit
	tracked bool

	fonts map[string]uintptr
	logo  uintptr // HBITMAP (premultiplied BGRA)
	logoW int
	logoH int

	mu        sync.Mutex
	tasks     []task
	cur       int
	curStart  time.Time
	runStart  time.Time
	doneTitle []string
	running   bool
	finished  bool
	errText   string
	funOrder  []int
	manual    map[string]bool // папки, указанные пользователем вручную — ставим туда даже без «12.5» в имени
	inst      map[string]bool // гаджет (по id) уже стоит в русской версии хотя бы в одной папке Aspire
}

var app = &App{opt: map[string]bool{}, launch: true}

var funLines = []string{
	"Наливаем чай…",
	"Раскладываем гаджеты по полочкам…",
	"Переводим «Dogbone» как «косточка»…",
	"Объясняем Aspire, что Keyhole — это замочная скважина…",
	"Перерисовываем схемы с размерами…",
	"Проверяем, что ни одна строчка Lua не пострадала…",
	"Прячем оригиналы в надёжное место…",
	"Сдуваем стружку с диалоговых окон…",
	"Пересчитываем косточки в углах…",
	"Накручиваем спираль на поворотную ось…",
	"Проверяем, что ноль по Z на месте…",
	"Ещё чуть-чуть — чай как раз остыл…",
}

// ---------- шрифты и логотип ----------
func (a *App) sc(v int) int32 { return int32(v * a.dpi / 96) }

func (a *App) makeResources() {
	for _, f := range a.fonts {
		pDeleteObject.Call(f)
	}
	a.fonts = map[string]uintptr{}
	mk := func(name string, px int, weight int, face string) {
		h, _, _ := pCreateFontW.Call(uintptr(int32(-a.sc(px))), 0, 0, 0, uintptr(weight), 0, 0, 0, 1, 0, 0, 5, 0, uintptr(unsafe.Pointer(u16(face))))
		a.fonts[name] = h
	}
	mk("title", 28, 600, "Segoe UI Semibold")
	mk("big", 20, 600, "Segoe UI Semibold")
	mk("h", 16, 600, "Segoe UI Semibold")
	mk("body", 15, 400, "Segoe UI")
	mk("small", 13, 400, "Segoe UI")
	mk("brand", 26, 700, "Segoe UI")
	mk("btn", 15, 600, "Segoe UI Semibold")
	mk("check", 15, 700, "Segoe UI Symbol")
	a.buildLogo()
}

func (a *App) buildLogo() {
	if a.logo != 0 {
		pDeleteObject.Call(a.logo)
		a.logo = 0
	}
	img, err := png.Decode(bytes.NewReader(logoPNG))
	if err != nil {
		return
	}
	b := img.Bounds()
	tw := int(a.sc(118))
	th := tw * b.Dy() / b.Dx()
	a.logoW, a.logoH = tw, th
	bi := BITMAPINFOHEADER{Size: 40, Width: int32(tw), Height: -int32(th), Planes: 1, BitCount: 32}
	var bits unsafe.Pointer
	h, _, _ := pCreateDIBSection.Call(0, uintptr(unsafe.Pointer(&bi)), 0, uintptr(unsafe.Pointer(&bits)), 0, 0)
	if h == 0 {
		return
	}
	px := unsafe.Slice((*byte)(bits), tw*th*4)
	// box-filter downscale with premultiplied alpha
	sx := float64(b.Dx()) / float64(tw)
	sy := float64(b.Dy()) / float64(th)
	for y := 0; y < th; y++ {
		for x := 0; x < tw; x++ {
			x0, x1 := int(float64(x)*sx), int(float64(x+1)*sx)
			y0, y1 := int(float64(y)*sy), int(float64(y+1)*sy)
			if x1 <= x0 {
				x1 = x0 + 1
			}
			if y1 <= y0 {
				y1 = y0 + 1
			}
			var r, g, bb, al, n float64
			for yy := y0; yy < y1; yy++ {
				for xx := x0; xx < x1; xx++ {
					cr, cg, cb, ca := img.At(b.Min.X+xx, b.Min.Y+yy).RGBA()
					r += float64(cr >> 8)
					g += float64(cg >> 8)
					bb += float64(cb >> 8)
					al += float64(ca >> 8)
					n++
				}
			}
			o := (y*tw + x) * 4
			px[o] = byte(bb / n)
			px[o+1] = byte(g / n)
			px[o+2] = byte(r / n)
			px[o+3] = byte(al / n)
		}
	}
	a.logo = h
}

var _ = image.Rect

// ---------- рисование ----------
type painter struct {
	dc uintptr
	a  *App
}

func (p painter) rect(x, y, w, h int32, c uint32) {
	br, _, _ := pCreateSolidBrush.Call(uintptr(c))
	r := RECT{x, y, x + w, y + h}
	pFillRect.Call(p.dc, uintptr(unsafe.Pointer(&r)), br)
	pDeleteObject.Call(br)
}

func (p painter) rrect(x, y, w, h, rad int32, fill uint32, border int64) {
	br, _, _ := pCreateSolidBrush.Call(uintptr(fill))
	var pen uintptr
	if border >= 0 {
		pen, _, _ = pCreatePen.Call(0, uintptr(p.a.sc(1)), uintptr(uint32(border)))
	} else {
		pen, _, _ = pCreatePen.Call(0, 1, uintptr(fill))
	}
	ob, _, _ := pSelectObject.Call(p.dc, br)
	op, _, _ := pSelectObject.Call(p.dc, pen)
	pRoundRect.Call(p.dc, uintptr(x), uintptr(y), uintptr(x+w), uintptr(y+h), uintptr(rad), uintptr(rad))
	pSelectObject.Call(p.dc, ob)
	pSelectObject.Call(p.dc, op)
	pDeleteObject.Call(br)
	pDeleteObject.Call(pen)
}

func (p painter) circle(cx, cy, r int32, fill uint32, border int64) {
	p.rrect(cx-r, cy-r, 2*r, 2*r, 2*r, fill, border)
}

// text draws s; returns used height
func (p painter) text(s string, x, y, w, h int32, font string, c uint32, flags uint32) int32 {
	pSelectObject.Call(p.dc, p.a.fonts[font])
	pSetTextColor.Call(p.dc, uintptr(c))
	pSetBkMode.Call(p.dc, TRANSPARENT)
	r := RECT{x, y, x + w, y + h}
	ws := u16(s)
	if flags&DT_WORDBREAK != 0 {
		cr := r
		pDrawTextW.Call(p.dc, uintptr(unsafe.Pointer(ws)), ^uintptr(0), uintptr(unsafe.Pointer(&cr)), uintptr(flags|DT_CALCRECT|DT_NOPREFIX))
		r.Bottom = r.Top + (cr.Bottom - cr.Top)
	}
	pDrawTextW.Call(p.dc, uintptr(unsafe.Pointer(ws)), ^uintptr(0), uintptr(unsafe.Pointer(&r)), uintptr(flags|DT_NOPREFIX))
	return r.Bottom - r.Top
}

func (p painter) line(x0, y0, x1, y1 int32, c uint32) {
	pen, _, _ := pCreatePen.Call(0, uintptr(p.a.sc(1)), uintptr(c))
	op, _, _ := pSelectObject.Call(p.dc, pen)
	pMoveToEx.Call(p.dc, uintptr(x0), uintptr(y0), 0)
	pLineTo.Call(p.dc, uintptr(x1), uintptr(y1))
	pSelectObject.Call(p.dc, op)
	pDeleteObject.Call(pen)
}

func (a *App) addHit(id string, x, y, w, h int32) {
	a.hits = append(a.hits, hit{id, RECT{x, y, x + w, y + h}})
}

func (a *App) button(p painter, id, label string, x, y, w, h int32, primary bool, enabled bool) {
	hov := a.hover == id && enabled
	if primary {
		c := cAccent
		if hov {
			c = cAccentH
		}
		if !enabled {
			c = cLine
		}
		p.rrect(x, y, w, h, a.sc(10), c, -1)
		tc := cDark
		if !enabled {
			tc = cDim
		}
		p.text(label, x, y, w, h, "btn", tc, DT_CENTER|DT_VCENTER|DT_SINGLELINE)
	} else {
		bc := int64(cLine)
		if hov {
			bc = int64(cMuted)
		}
		p.rrect(x, y, w, h, a.sc(10), cBg, bc)
		p.text(label, x, y, w, h, "btn", cText, DT_CENTER|DT_VCENTER|DT_SINGLELINE)
	}
	if enabled {
		a.addHit(id, x, y, w, h)
	}
}

func (a *App) checkbox(p painter, id string, x, y int32, on, locked bool) {
	s := a.sc(20)
	if on {
		c := cAccent
		if locked {
			c = rgb(0x8a, 0x4a, 0x33)
		}
		p.rrect(x, y, s, s, a.sc(6), c, -1)
		p.text("✓", x, y-a.sc(1), s, s, "check", cDark, DT_CENTER|DT_VCENTER|DT_SINGLELINE)
	} else {
		bc := int64(cMuted)
		if a.hover == id {
			bc = int64(cAccent)
		}
		p.rrect(x, y, s, s, a.sc(6), cBg, bc)
	}
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	neg := n < 0
	if neg {
		n = -n
	}
	var b []byte
	for n > 0 {
		b = append([]byte{byte('0' + n%10)}, b...)
		n /= 10
	}
	if neg {
		b = append([]byte{'-'}, b...)
	}
	return string(b)
}

// progress: must be called with a.mu held
func (a *App) progress() float64 {
	if len(a.tasks) == 0 {
		return 0
	}
	var total, done float64
	for i, t := range a.tasks {
		w := t.min.Seconds()
		total += w
		if i < a.cur {
			done += w
		} else if i == a.cur && a.running {
			f := time.Since(a.curStart).Seconds() / w
			if f > 0.97 {
				f = 0.97
			}
			done += w * f
		}
	}
	if a.finished {
		return 1
	}
	return done / total
}
func (a *App) hitAt(x, y int32) string {
	for i := len(a.hits) - 1; i >= 0; i-- {
		r := a.hits[i].r
		if x >= r.Left && x < r.Right && y >= r.Top && y < r.Bottom {
			return a.hits[i].id
		}
	}
	return ""
}

func (a *App) primary() string {
	switch a.page {
	case 0:
		if len(a.dirs) > 0 {
			return "next"
		}
	case 1:
		return "install"
	case 3:
		return "finish"
	}
	return ""
}

func wndProc(hwnd, msg, wp, lp uintptr) uintptr {
	a := app
	switch msg {
	case WM_ERASEBKGND:
		return 1
	case WM_PAINT:
		var ps PAINTSTRUCT
		hdc, _, _ := pBeginPaint.Call(hwnd, uintptr(unsafe.Pointer(&ps)))
		var rc RECT
		pGetClientRect.Call(hwnd, uintptr(unsafe.Pointer(&rc)))
		mdc, _, _ := pCreateCompatibleDC.Call(hdc)
		bmp, _, _ := pCreateCompatibleBitmap.Call(hdc, uintptr(rc.Right), uintptr(rc.Bottom))
		old, _, _ := pSelectObject.Call(mdc, bmp)
		a.paint(mdc, rc.Right, rc.Bottom)
		pBitBlt.Call(hdc, 0, 0, uintptr(rc.Right), uintptr(rc.Bottom), mdc, 0, 0, SRCCOPY)
		pSelectObject.Call(mdc, old)
		pDeleteObject.Call(bmp)
		pDeleteDC.Call(mdc)
		pEndPaint.Call(hwnd, uintptr(unsafe.Pointer(&ps)))
		return 0
	case WM_TIMER:
		a.mu.Lock()
		run := a.page == 2
		a.mu.Unlock()
		if run {
			pInvalidateRect.Call(hwnd, 0, 0)
		}
		return 0
	case WM_MOUSEMOVE:
		x, y := int32(int16(lp&0xffff)), int32(int16((lp>>16)&0xffff))
		h := a.hitAt(x, y)
		if h != a.hover {
			a.hover = h
			pInvalidateRect.Call(hwnd, 0, 0)
		}
		if !a.tracked {
			t := TRACKMOUSEEVENT{Size: uint32(unsafe.Sizeof(TRACKMOUSEEVENT{})), Flags: TME_LEAVE, Hwnd: hwnd}
			pTrackMouseEvent.Call(uintptr(unsafe.Pointer(&t)))
			a.tracked = true
		}
		return 0
	case WM_MOUSELEAVE:
		a.tracked = false
		if a.hover != "" {
			a.hover = ""
			pInvalidateRect.Call(hwnd, 0, 0)
		}
		return 0
	case WM_SETCURSOR:
		if lp&0xffff == 1 { // HTCLIENT
			c := uintptr(IDC_ARROW)
			if a.hover != "" {
				c = IDC_HAND
			}
			h, _, _ := pLoadCursorW.Call(0, c)
			pSetCursor.Call(h)
			return 1
		}
	case WM_LBUTTONUP:
		x, y := int32(int16(lp&0xffff)), int32(int16((lp>>16)&0xffff))
		if h := a.hitAt(x, y); h != "" {
			a.click(h)
		}
		return 0
	case WM_KEYDOWN:
		if wp == VK_RETURN {
			if p := a.primary(); p != "" {
				a.click(p)
			}
		} else if wp == VK_ESCAPE && a.page != 2 {
			if a.page == 3 {
				a.click("finish")
			} else {
				pDestroyWindow.Call(hwnd)
			}
		}
		return 0
	case WM_DPICHANGED:
		a.dpi = int(wp & 0xffff)
		r := (*RECT)(unsafe.Pointer(lp))
		pSetWindowPos.Call(hwnd, 0, uintptr(r.Left), uintptr(r.Top), uintptr(r.Right-r.Left), uintptr(r.Bottom-r.Top), SWP_NOZORDER|SWP_NOACTIVATE)
		a.makeResources()
		pInvalidateRect.Call(hwnd, 0, 0)
		return 0
	case WM_CLOSE:
		a.mu.Lock()
		busy := a.page == 2
		a.mu.Unlock()
		if busy {
			return 0
		}
		pDestroyWindow.Call(hwnd)
		return 0
	case WM_DESTROY:
		pPostQuitMessage.Call(0)
		return 0
	}
	r, _, _ := pDefWindowProcW.Call(hwnd, msg, wp, lp)
	return r
}
