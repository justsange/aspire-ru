package main

import (
	"bytes"
	_ "embed"
	"image"
	"image/png"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"syscall"
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
	LW = 880 // логическая ширина клиентской области
	LH = 580
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
	page    int // 0 приветствие, 1 что изменится, 2 установка, 3 готово
	mode    string
	dirs    []string
	opt     map[string]bool
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
	verOK     map[string]bool
	inst      map[string]bool
	bak       map[string]bool
}

var app = &App{opt: map[string]bool{"iface": true, "help": true, "mat": true, "lnk": true, "box": boxAvailable, "fix": true, "dark": false}, launch: true}

var funLines = []string{
	"Наливаем чай…",
	"Меняем 1 700 непереведённых строк на русские…",
	"Объясняем Aspire, что Tabs — это мостики…",
	"Исправляем «Бильд» и прочие перлы старого перевода…",
	"Затачиваем фрезы…",
	"Сдуваем стружку с диалоговых окон…",
	"Переводим 246 страниц справки…",
	"Учим справку искать по-русски…",
	"Перерисовываем 2 000 надписей на скриншотах…",
	"Ставим «Дуб светлый» вместо Oak Light…",
	"Пересчитываем контрольную сумму — Aspire любит порядок…",
	"Проверяем, что ноль по Z на месте…",
	"Пылесосим стол станка…",
	"Подбираем подачу и обороты…",
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

func (a *App) paint(dc uintptr, W, H int32) {
	a.hits = a.hits[:0]
	p := painter{dc, a}
	S := func(v int) int32 { return a.sc(v) }
	p.rect(0, 0, W, H, cBg)

	// --- боковая панель ---
	p.rect(0, 0, S(SB), H, cSide)
	if a.logo != 0 {
		mdc, _, _ := pCreateCompatibleDC.Call(dc)
		old, _, _ := pSelectObject.Call(mdc, a.logo)
		lx := (S(SB) - int32(a.logoW)) / 2
		bf := uint32(0) | uint32(0)<<8 | uint32(255)<<16 | uint32(1)<<24
		pAlphaBlend.Call(dc, uintptr(lx), uintptr(S(34)), uintptr(a.logoW), uintptr(a.logoH), mdc, 0, 0, uintptr(a.logoW), uintptr(a.logoH), uintptr(bf))
		pSelectObject.Call(mdc, old)
		pDeleteDC.Call(mdc)
	}
	p.text("A&A", 0, S(150), S(SB), S(34), "brand", cText, DT_CENTER|DT_SINGLELINE)
	p.text("мастерская форм", 0, S(184), S(SB), S(22), "small", cMuted, DT_CENTER|DT_SINGLELINE)
	p.rect((S(SB)-S(36))/2, S(214), S(36), S(3), cAccent)

	steps := []string{"Приветствие", "Что изменится", "Установка", "Готово"}
	if a.mode == "restore" {
		steps[2] = "Удаление"
	}
	for i, s := range steps {
		y := S(258 + i*46)
		cx, cy, r := S(46), y+S(11), S(12)
		col := cDim
		switch {
		case i < a.page || (i == 3 && a.page == 3):
			p.circle(cx, cy, r, cAccent, -1)
			p.text("✓", cx-r, cy-r-S(1), 2*r, 2*r, "check", cDark, DT_CENTER|DT_VCENTER|DT_SINGLELINE)
			col = cMuted
		case i == a.page:
			p.circle(cx, cy, r, cSide, int64(cAccent))
			p.circle(cx, cy, S(5), cAccent, -1)
			col = cText
		default:
			p.circle(cx, cy, r, cSide, int64(cLine))
		}
		if i < len(steps)-1 {
			p.rect(cx-S(1), cy+r+S(3), S(2), S(46)-2*r-S(6), cLine)
		}
		f := "body"
		if i == a.page {
			f = "h"
		}
		p.text(s, S(70), y, S(SB-80), S(22), f, col, DT_LEFT|DT_VCENTER|DT_SINGLELINE)
	}
	lc := cMuted
	if a.hover == "site" {
		lc = cAccent
	}
	p.text("aa-forma.ru", 0, H-S(64), S(SB), S(22), "body", lc, DT_CENTER|DT_SINGLELINE)
	a.addHit("site", S(70), H-S(64), S(SB-140), S(22))
	p.text("русификатор v"+version+" · бесплатно", 0, H-S(38), S(SB), S(18), "small", cDim, DT_CENTER|DT_SINGLELINE)

	// --- контент ---
	x := S(SB + 44)
	cw := W - x - S(44)
	y := S(38)
	by := H - S(24) - S(40)
	bw, bh := S(160), S(40)

	switch a.page {
	case 0:
		y += p.text("Русификатор Vectric Aspire 12.5", x, y, cw, S(60), "title", cText, DT_WORDBREAK) + S(4)
		y += p.text("от мастерской A&A — мастерская форм", x, y, cw, S(24), "h", cAccent, DT_SINGLELINE) + S(20)
		y += p.text("Русская версия Aspire переведена не до конца: часть меню и окон осталась на английском, справка — целиком на английском, а в переведённом хватает ошибок. Этот установщик доводит перевод до ума. Оригинальные файлы сохраняются — всё можно вернуть одной кнопкой.",
			x, y, cw, S(200), "body", cText, DT_WORDBREAK) + S(22)
		// бесплатно
		bxh := S(118)
		p.rrect(x, y, cw, bxh, S(14), cPanel, int64(cLine))
		p.rect(x, y+S(14), S(4), bxh-S(28), cAccent)
		p.text("Это бесплатно", x+S(22), y+S(14), cw-S(40), S(24), "h", cAccent, DT_SINGLELINE)
		p.text("Русификатор распространяется бесплатно и не продаётся. Если вы за него заплатили — вас обманули. Делитесь с друзьями-фрезеровщиками на здоровье.",
			x+S(22), y+S(42), cw-S(44), S(80), "body", cText, DT_WORDBREAK)
		y += bxh + S(20)
		// что нашли
		if len(a.dirs) == 0 {
			p.text("⚠  Aspire не найден в Program Files. Установите Aspire 12.5 и запустите установщик снова.", x, y, cw, S(50), "body", cAccent, DT_WORDBREAK)
		} else {
			for _, d := range a.dirs {
				st := "✓  Найден Aspire: " + d
				c := cOk
				if !is125(d) {
					st = "—  " + d + " — другая версия Aspire, её не трогаем"
					c = cMuted
					if a.inst[d] || a.bak[d] {
						st = "⚠  " + d + " — другая версия, в неё прошлый русификатор поставил свои файлы. При установке вернём ей родные"
						c = cAccent
					}
				} else if !a.verOK[d] {
					st = "⚠  " + d + " — сборка отличается от 12.5.0.4, перевод может лечь неровно"
					c = cAccent
				}
				y += p.text(st, x, y, cw, S(40), "small", c, DT_WORDBREAK) + S(4)
				if a.inst[d] {
					y += p.text("Русификатор уже установлен — можно переустановить или удалить.", x, y, cw, S(40), "small", cMuted, DT_WORDBREAK) + S(4)
				}
			}
		}
		anyBak := false
		for _, d := range a.dirs {
			if a.bak[d] {
				anyBak = true
			}
		}
		if anyBak {
			c := cMuted
			if a.hover == "uninstall" {
				c = cAccent
			}
			p.text("Удалить русификацию", x, by, S(220), bh, "body", c, DT_LEFT|DT_VCENTER|DT_SINGLELINE)
			a.addHit("uninstall", x, by, S(170), bh)
		}
		a.button(p, "cancel", "Отмена", W-S(44)-bw-S(12)-S(120), by, S(120), bh, false, true)
		a.button(p, "next", "Далее  →", W-S(44)-bw, by, bw, bh, true, len(a.dirs) > 0)

	case 1:
		y += p.text("Что изменится", x, y, cw, S(44), "title", cText, DT_SINGLELINE) + S(2)
		y += p.text("Всё касается только русской версии Aspire. Отметьте, что ставить:", x, y, cw, S(40), "body", cMuted, DT_WORDBREAK) + S(18)
		items := []struct{ id, title, desc string }{
			{"iface", "Меню, окна и сообщения", "Допереводим около 1 700 строк и исправляем около 1 400 ошибочных — вроде «Подача» вместо «Холостой ход»."},
			{"help", "Справка по F1 — на русском", "246 страниц, поиск по справке и переведённые скриншоты окон."},
			{"mat", "Материалы и база фрез", "Текстуры («Дуб светлый», «Латунь») и база фрез («Концевые», «МДФ»). Ваши фрезы не трогаем."},
			{"lnk", "Ярлык на рабочем столе", "«Aspire (RU)» — запуск сразу на русском языке."},
		}
		if boxAvailable {
			items = append(items, struct{ id, title, desc string }{"box", "Гаджет «Конструктор коробок»", "Simple Box Creator 12.6 со сдвижной крышкой. Авторы: gremlin529 и SharkCutUp (GitHub)."})
		}
		if len(a.brokenDirs()) > 0 {
			items = append(items, struct{ id, title, desc string }{"fix", "Починить другие версии Aspire", "Прошлый русификатор по ошибке поставил свои файлы в " + strings.Join(a.brokenDirs(), ", ") + " — вернём им родные файлы."})
		}
		if darkAvailable {
			items = append(items, struct{ id, title, desc string }{"dark", "Тёмная тема", "Тёмное оформление, перерисованные под него иконки и тёмный фон 3D-вида."})
		}
		for _, it := range items {
			on := a.opt[it.id]
			locked := it.id == "iface"
			a.checkbox(p, it.id, x, y+S(2), on, locked)
			tt := it.title
			if locked {
				tt += "  (основное, ставится всегда)"
			}
			p.text(tt, x+S(34), y, cw-S(34), S(24), "h", cText, DT_SINGLELINE)
			dh := p.text(it.desc, x+S(34), y+S(25), cw-S(34), S(80), "small", cMuted, DT_WORDBREAK)
			if !locked {
				a.addHit("opt:"+it.id, x, y, cw, S(25)+dh)
			}
			y += S(25) + dh + S(12)
		}
		y += S(4)
		p.line(x, y, x+cw, y, cLine)
		y += S(12)
		p.text("Перед установкой сохраняем оригиналы — всё возвращается кнопкой «Удалить русификацию». Программа, лицензия и проекты не затрагиваются.",
			x, y, cw, S(80), "small", cMuted, DT_WORDBREAK)
		a.button(p, "back", "←  Назад", W-S(44)-bw-S(12)-S(120), by, S(120), bh, false, true)
		a.button(p, "install", "Установить", W-S(44)-bw, by, bw, bh, true, true)

	case 2:
		a.mu.Lock()
		title := "Устанавливаем…"
		if a.mode == "restore" {
			title = "Возвращаем как было…"
		}
		y += p.text(title, x, y, cw, S(44), "title", cText, DT_SINGLELINE) + S(30)
		el := time.Since(a.runStart).Seconds()
		fun := ""
		if a.mode == "restore" {
			fun = "Аккуратно возвращаем заводские файлы…"
		} else if len(a.funOrder) > 0 {
			fun = funLines[a.funOrder[int(el/2.8)%len(a.funOrder)]]
		}
		p.text(fun, x, y, cw, S(60), "big", cText, DT_WORDBREAK)
		y += S(70)
		prog := a.progress()
		p.rrect(x, y, cw, S(12), S(12), cLine, -1)
		fw := int32(float64(cw) * prog)
		if fw > S(12) {
			p.rrect(x, y, fw, S(12), S(12), cAccent, -1)
		}
		p.text(itoa(int(prog*100))+"%", x, y-S(26), cw, S(20), "small", cMuted, DT_RIGHT|DT_SINGLELINE)
		y += S(26)
		if a.cur < len(a.tasks) {
			p.text("Шаг "+itoa(a.cur+1)+" из "+itoa(len(a.tasks))+" · "+a.tasks[a.cur].title, x, y, cw, S(20), "small", cMuted, DT_SINGLELINE|DT_END_ELLIPSIS)
		}
		y += S(32)
		// весь список виден сразу: впереди — тусклые пункты, текущий — с оранжевой точкой,
		// выполненные — яркие с зелёной галочкой
		for i, t := range a.tasks {
			mark, mc, tc := "○", cDim, cDim
			if i < len(a.doneTitle) {
				mark, mc, tc = "✓", cOk, cText
			} else if i == a.cur && a.running {
				mark, mc, tc = "●", cAccent, cMuted
			}
			p.text(mark, x, y, S(24), S(22), "check", mc, DT_LEFT|DT_SINGLELINE)
			p.text(t.title, x+S(26), y, cw-S(26), S(22), "body", tc, DT_SINGLELINE|DT_END_ELLIPSIS)
			y += S(25)
		}
		a.mu.Unlock()
		a.button(p, "wait", "Подождите…", W-S(44)-bw, by, bw, bh, true, false)

	case 3:
		a.mu.Lock()
		ok := a.errText == ""
		if ok {
			t := "Готово!"
			y += p.text(t, x, y, cw, S(44), "title", cText, DT_SINGLELINE) + S(8)
			msg := "Русификация установлена. Запускайте Aspire ярлыком «Aspire (RU)» на рабочем столе — меню, окна, справка по F1 и материалы теперь на русском."
			if a.mode == "restore" {
				msg = "Оригинальные файлы возвращены — Aspire снова в заводском виде. Русификатор можно поставить заново в любой момент."
			}
			y += p.text(msg, x, y, cw, S(90), "body", cText, DT_WORDBREAK) + S(18)
			for _, t := range a.doneTitle {
				p.text("✓", x, y, S(24), S(22), "check", cOk, DT_LEFT|DT_SINGLELINE)
				p.text(t, x+S(26), y, cw-S(26), S(22), "body", cMuted, DT_SINGLELINE|DT_END_ELLIPSIS)
				y += S(26)
			}
			y += S(14)
			if a.mode == "install" {
				a.checkbox(p, "launch", x, y+S(1), a.launch, false)
				p.text("Запустить Aspire на русском", x+S(34), y, cw, S(24), "body", cText, DT_SINGLELINE)
				a.addHit("launch", x, y, S(300), S(24))
				y += S(42)
			}
			bxh := S(76)
			p.rrect(x, y, cw, bxh, S(14), cPanel, int64(cLine))
			p.rect(x, y+S(14), S(4), bxh-S(28), cAccent)
			p.text("Спасибо, что пользуетесь!", x+S(22), y+S(12), cw-S(40), S(24), "h", cText, DT_SINGLELINE)
			lc := cAccent
			if a.hover == "site2" {
				lc = cAccentH
			}
			p.text("Модели для ЧПУ и полезное для фрезеровщиков — на aa-forma.ru", x+S(22), y+S(40), cw-S(40), S(22), "small", lc, DT_SINGLELINE|DT_END_ELLIPSIS)
			a.addHit("site2", x+S(22), y+S(40), cw-S(40), S(22))
		} else {
			y += p.text("Не получилось", x, y, cw, S(44), "title", cAccent, DT_SINGLELINE) + S(8)
			y += p.text(a.errText, x, y, cw, S(120), "body", cText, DT_WORDBREAK) + S(16)
			p.text("Ничего страшного: оригинальные файлы сохранены. Закройте Aspire и попробуйте ещё раз, или запустите «Удалить русификацию».", x, y, cw, S(80), "small", cMuted, DT_WORDBREAK)
		}
		a.mu.Unlock()
		a.button(p, "finish", "Готово", W-S(44)-bw, by, bw, bh, true, true)
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

// ---------- сценарии ----------
func (a *App) buildTasks() []task {
	each := func(f func(string) error) func() error {
		return func() error {
			for _, d := range a.workDirs() {
				if err := f(d); err != nil {
					return err
				}
			}
			return nil
		}
	}
	if a.mode == "restore" {
		rt := []task{
			{"Проверяем копию оригинальных файлов", 2500 * time.Millisecond, func() error { return nil }},
			{"Возвращаем заводской перевод интерфейса", 4 * time.Second, each(doRestore)},
			{"Возвращаем английские названия материалов", 3 * time.Second, each(func(d string) error { renameTextures(d, false); return nil })},
			{"Возвращаем английскую базу фрез", 2 * time.Second, each(func(d string) error { translateToolDB(d, false); return nil })},
			{"Убираем русскую справку", 3 * time.Second, each(func(d string) error { removeHelp(d); return nil })},
			{"Убираем ярлык «Aspire (RU)»", 2 * time.Second, func() error { os.Remove(shortcutPath()); return nil }},
			{"Убираем гаджет «Конструктор коробок»", 2 * time.Second, each(func(d string) error { removeGadget(d); return nil })},
		}
		if darkAvailable {
			rt = append(rt, task{"Возвращаем светлую тему, если включали тёмную", 2 * time.Second, each(func(d string) error { removeDarkTheme(d); return nil })})
		}
		return rt
	}
	t := []task{
		{"Проверяем Aspire и его версию", 3 * time.Second, func() error { return nil }},
	}
	broken := a.brokenDirs()
	if a.opt["fix"] && len(broken) > 0 {
		t = append(t, task{"Чиним другие версии Aspire после прошлого русификатора", 4 * time.Second, func() error {
			for _, d := range broken {
				if err := repairOtherVersion(d); err != nil {
					return err
				}
			}
			return nil
		}})
	}
	t = append(t, []task{
		{"Делаем копию оригинальных файлов", 4 * time.Second, each(doBackup)},
		{"Ставим перевод меню, окон и сообщений", 9 * time.Second, each(doInterface)},
	}...)
	t = append(t, task{"Возвращаем права на базу фрез и настройки Aspire", 3 * time.Second, each(fixDataPerms)})
	if a.opt["mat"] {
		t = append(t, task{"Переводим библиотеку материалов", 5 * time.Second, each(func(d string) error { renameTextures(d, true); return nil })})
		t = append(t, task{"Переводим базу фрез: «Концевые», «Сферические», «МДФ»", 4 * time.Second, each(func(d string) error { translateToolDB(d, true); return nil })})
	}
	if a.opt["help"] {
		t = append(t, task{"Ставим русскую справку (F1) и поиск", 9 * time.Second, each(installHelp)})
	}
	if a.opt["dark"] {
		t = append(t, task{"Включаем тёмную тему и гасим свет в 3D-виде", 4 * time.Second, each(installDarkTheme)})
	} else {
		dark := false
		for _, d := range a.dirs {
			if _, err := os.Stat(themeBackupPath(d)); err == nil {
				dark = true
			}
		}
		if dark {
			t = append(t, task{"Возвращаем светлую тему", 2 * time.Second, each(func(d string) error { removeDarkTheme(d); return nil })})
		}
	}
	if a.opt["box"] {
		t = append(t, task{"Ставим гаджет «Конструктор коробок» на русском", 3 * time.Second, each(installGadget)})
	} else {
		box := false
		for _, d := range a.dirs {
			if gadgetInstalled(d) {
				box = true
			}
		}
		if box {
			t = append(t, task{"Убираем гаджет «Конструктор коробок»", 2 * time.Second, each(func(d string) error { removeGadget(d); return nil })})
		}
	}
	if a.opt["lnk"] && len(a.targets()) > 0 {
		t = append(t, task{"Создаём ярлык «Aspire (RU)» на рабочем столе", 3 * time.Second, func() error { makeShortcut(a.targets()[0]); return nil }})
	}
	t = append(t, task{"Наводим порядок и проверяем результат", 3 * time.Second, func() error { return nil }})
	return t
}

func (a *App) start() {
	a.mu.Lock()
	a.tasks = a.buildTasks()
	a.cur = 0
	a.doneTitle = nil
	a.errText = ""
	a.running = true
	a.finished = false
	a.runStart = time.Now()
	a.curStart = time.Now()
	// порядок шуток: первый — чай, остальные перемешаны
	a.funOrder = []int{0}
	seed := int(time.Now().UnixNano() % 1000)
	rest := []int{}
	for i := 1; i < len(funLines); i++ {
		rest = append(rest, i)
	}
	for i := len(rest) - 1; i > 0; i-- {
		j := (seed*7 + i*13) % (i + 1)
		rest[i], rest[j] = rest[j], rest[i]
	}
	a.funOrder = append(a.funOrder, rest...)
	a.page = 2
	a.mu.Unlock()
	go func() {
		for i := range a.tasks {
			a.mu.Lock()
			a.cur = i
			a.curStart = time.Now()
			t := a.tasks[i]
			a.mu.Unlock()
			err := t.fn()
			if rem := t.min - time.Since(a.curStart); rem > 0 {
				time.Sleep(rem)
			}
			a.mu.Lock()
			if err != nil {
				a.errText = t.title + ": " + err.Error()
				a.running = false
				a.finished = true
				a.page = 3
				a.mu.Unlock()
				pInvalidateRect.Call(a.hwnd, 0, 0)
				return
			}
			a.doneTitle = append(a.doneTitle, t.title)
			a.mu.Unlock()
		}
		time.Sleep(600 * time.Millisecond)
		a.mu.Lock()
		a.cur = len(a.tasks)
		a.running = false
		a.finished = true
		a.page = 3
		a.mu.Unlock()
		pInvalidateRect.Call(a.hwnd, 0, 0)
	}()
}

// ---------- действия ----------
func (a *App) click(id string) {
	switch {
	case id == "site" || id == "site2":
		openURL("https://aa-forma.ru")
	case id == "cancel":
		pDestroyWindow.Call(a.hwnd)
	case id == "next":
		a.page = 1
	case id == "back":
		a.page = 0
	case strings.HasPrefix(id, "opt:"):
		k := id[4:]
		a.opt[k] = !a.opt[k]
	case id == "launch":
		a.launch = !a.launch
	case id == "install":
		if !a.ensureClosed() {
			return
		}
		if len(a.targets()) == 0 {
			msgBox(a.hwnd, "Не найден Aspire 12.5. Русификатор сделан только под Aspire 12.5 — другие версии он не трогает.", "Aspire 12.5 не найден", MB_OK|MB_ICONWARNING)
			return
		}
		for _, d := range a.targets() {
			if !a.verOK[d] {
				if msgBox(a.hwnd, "Эта сборка Aspire отличается от 12.5.0.4, под которую сделан перевод:\n"+d+"\n\nОкна могут отображаться неправильно, а программа — падать. Всё равно установить?", "Другая версия Aspire", MB_YESNO|MB_ICONWARNING) != IDYES {
					return
				}
			}
		}
		a.mode = "install"
		a.start()
	case id == "uninstall":
		if msgBox(a.hwnd, "Вернуть заводские файлы Aspire и убрать русификацию?\n\nПрограмма, лицензия и ваши проекты не затрагиваются.", "Удалить русификацию", MB_YESNO|MB_ICONQUESTION) != IDYES {
			return
		}
		if !a.ensureClosed() {
			return
		}
		a.mode = "restore"
		a.start()
	case id == "finish":
		if a.mode == "install" && a.errText == "" && a.launch && len(a.targets()) > 0 {
			launchAspire(a.targets()[0])
		}
		pDestroyWindow.Call(a.hwnd)
	}
	pInvalidateRect.Call(a.hwnd, 0, 0)
}

// brokenDirs — другие версии Aspire, в которых лежит наша копия оригиналов:
// туда прошлый русификатор (до 2.6.3) поставил файлы от 12.5, и эти версии не запускаются.
func (a *App) brokenDirs() []string {
	var out []string
	for _, d := range a.dirs {
		if !is125(d) && a.bak[d] {
			out = append(out, d)
		}
	}
	return out
}

// targets — куда ставим: только Aspire 12.5.
func (a *App) targets() []string {
	var out []string
	for _, d := range a.dirs {
		if is125(d) {
			out = append(out, d)
		}
	}
	return out
}

// workDirs — с чем работают задачи: при установке это 12.5, при удалении — все папки,
// где есть наша копия оригиналов (в том числе другие версии, куда русификатор 2.1–2.6
// мог попасть по ошибке).
func (a *App) workDirs() []string {
	if a.mode != "restore" {
		return a.targets()
	}
	var out []string
	for _, d := range a.dirs {
		if a.bak[d] {
			out = append(out, d)
		}
	}
	return out
}

func (a *App) refresh() {
	a.verOK, a.inst, a.bak = map[string]bool{}, map[string]bool{}, map[string]bool{}
	for _, d := range a.dirs {
		a.verOK[d] = versionOK(d)
		a.inst[d] = isInstalled(d)
		a.bak[d] = hasBackup(d)
	}
}

func (a *App) ensureClosed() bool {
	for aspireRunning() {
		if msgBox(a.hwnd, "Aspire сейчас открыт. Сохраните проект, закройте программу и нажмите «Повтор».", "Закройте Aspire", MB_RETRYCANCEL|MB_ICONWARNING) != IDRETRY {
			return false
		}
	}
	return true
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

func main() {
	if pSetProcessDpiAwarenessCx.Find() == nil { // Windows 10 1703+; на Windows 7/8 просто пропускаем
		pSetProcessDpiAwarenessCx.Call(^uintptr(3)) // PER_MONITOR_AWARE_V2 (-4)
	}
	exe, _ := os.Executable()
	app.dirs = findAspire()
	// тёмная тема уже стоит — галочка включена, снятие галочки при переустановке её выключит
	for _, d := range app.dirs {
		if _, err := os.Stat(themeBackupPath(d)); err == nil {
			app.opt["dark"] = true
		}
	}
	app.refresh()
	app.mode = "install"

	hinst, _, _ := pGetModuleHandleW.Call(0)
	icon, _, _ := pLoadIconW.Call(hinst, 1)
	cur, _, _ := pLoadCursorW.Call(0, IDC_ARROW)
	cls := u16("AAFormaAspireRU")
	wc := WNDCLASSEX{Style: CS_HREDRAW | CS_VREDRAW, WndProc: syscall.NewCallback(wndProc), Instance: hinst, Icon: icon, IconSm: icon, Cursor: cur, ClassName: cls}
	wc.Size = uint32(unsafe.Sizeof(wc))
	pRegisterClassExW.Call(uintptr(unsafe.Pointer(&wc)))

	style := uintptr(WS_OVERLAPPED | WS_CAPTION | WS_SYSMENU | WS_MINIMIZEBOX)
	hwnd, _, _ := pCreateWindowExW.Call(0, uintptr(unsafe.Pointer(cls)), uintptr(unsafe.Pointer(u16("Русификатор Aspire — A&A мастерская форм"))),
		style, 100, 100, 400, 300, 0, 0, hinst, 0)
	app.hwnd = hwnd
	var d uintptr
	if pGetDpiForWindow.Find() == nil { // Windows 10 1607+
		d, _, _ = pGetDpiForWindow.Call(hwnd)
	}
	if d == 0 {
		d = 96
	}
	app.dpi = int(d)
	app.makeResources()
	// тёмный заголовок окна (Windows 10/11)
	if pDwmSetWindowAttr.Find() == nil {
		dark := int32(1)
		pDwmSetWindowAttr.Call(hwnd, 20, uintptr(unsafe.Pointer(&dark)), 4)
		capc := cSide
		pDwmSetWindowAttr.Call(hwnd, 35, uintptr(unsafe.Pointer(&capc)), 4)
	}

	r := RECT{0, 0, app.sc(LW), app.sc(LH)}
	pAdjustWindowRectEx.Call(uintptr(unsafe.Pointer(&r)), style, 0, 0)
	w, h := r.Right-r.Left, r.Bottom-r.Top
	sw, _, _ := pGetSystemMetrics.Call(0)
	sh, _, _ := pGetSystemMetrics.Call(1)
	pSetWindowPos.Call(hwnd, 0, uintptr((int32(sw)-w)/2), uintptr((int32(sh)-h)/2), uintptr(w), uintptr(h), SWP_NOZORDER)
	pShowWindow.Call(hwnd, 1)
	pUpdateWindow.Call(hwnd)
	pSetTimer.Call(hwnd, 1, 50, 0)

	b := strings.ToLower(filepath.Base(exe))
	if strings.Contains(b, "restore") || strings.Contains(b, "uninstall") || strings.Contains(b, "откат") {
		pPostMessageW.Call(hwnd, WM_APP, 0, 0)
	}

	var m MSG
	for {
		r, _, _ := pGetMessageW.Call(uintptr(unsafe.Pointer(&m)), 0, 0, 0)
		if int32(r) <= 0 {
			break
		}
		if m.Message == WM_APP {
			app.click("uninstall")
			continue
		}
		pTranslateMessage.Call(uintptr(unsafe.Pointer(&m)))
		pDispatchMessageW.Call(uintptr(unsafe.Pointer(&m)))
	}
}
