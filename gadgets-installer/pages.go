package main

import (
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"time"
	"unsafe"
)

const gridCols = 2

// ---------- рисование страниц ----------
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

	steps := []string{"Приветствие", "Выбор гаджетов", "Установка", "Готово"}
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
	p.text("гаджеты на русском v"+version+" · бесплатно", 0, H-S(38), S(SB), S(18), "small", cDim, DT_CENTER|DT_SINGLELINE)

	// --- контент ---
	x := S(SB + 44)
	cw := W - x - S(44)
	y := S(38)
	by := H - S(24) - S(40)
	bw, bh := S(160), S(40)

	switch a.page {
	case 0:
		y += p.text("Гаджеты Aspire 12.5 — на русском", x, y, cw, S(60), "title", cText, DT_WORDBREAK) + S(4)
		y += p.text("от мастерской A&A — мастерская форм", x, y, cw, S(24), "h", cAccent, DT_SINGLELINE) + S(20)
		y += p.text("Гаджеты — это дополнения к Aspire из меню «Гаджеты»: «косточки» в углах, пазы, фаски, замочная скважина, спирали, поворотная ось, конструкторы коробок и другие. Здесь 24 гаджета — встроенные в Aspire и из библиотеки Vectric — с переведёнными окнами, подсказками, сообщениями и схемами. Логика скриптов не менялась.",
			x, y, cw, S(200), "body", cText, DT_WORDBREAK) + S(22)
		bxh := S(118)
		p.rrect(x, y, cw, bxh, S(14), cPanel, int64(cLine))
		p.rect(x, y+S(14), S(4), bxh-S(28), cAccent)
		p.text("Это бесплатно", x+S(22), y+S(14), cw-S(40), S(24), "h", cAccent, DT_SINGLELINE)
		p.text("Набор распространяется бесплатно и не продаётся. Если вы за него заплатили — вас обманули. Оригинальные гаджеты сохраняются — всё можно вернуть одной кнопкой.",
			x+S(22), y+S(42), cw-S(44), S(80), "body", cText, DT_WORDBREAK)
		y += bxh + S(20)
		if len(a.dirs) == 0 {
			y += p.text("⚠  Aspire 12.5 не найден ни на одном диске. Если он установлен в необычную папку — укажите её кнопкой ниже.", x, y, cw, S(50), "body", cAccent, DT_WORDBREAK) + S(6)
		} else {
			for _, d := range a.dirs {
				st := "✓  Найден Aspire: " + d
				c := cOk
				if !a.isTarget(d) {
					st = "—  " + d + " — другая версия Aspire, её не трогаем"
					c = cMuted
				}
				y += p.text(st, x, y, cw, S(40), "small", c, DT_WORDBREAK) + S(4)
			}
			if n := a.installedCount(); n > 0 {
				y += p.text("Русские гаджеты уже стоят: "+itoa(n)+" из "+itoa(len(gadgets))+" — можно обновить, добавить или удалить.", x, y, cw, S(40), "small", cMuted, DT_WORDBREAK) + S(4)
			}
		}
		y += S(8)
		if len(a.dirs) == 0 {
			a.button(p, "browse", "Указать папку Aspire…", x, y, S(260), S(36), true, true)
		} else {
			bc := cAccent
			if a.hover == "browse" {
				bc = cText
			}
			p.text("Aspire стоит в другом месте?  Указать папку вручную →", x, y, cw, S(24), "h", bc, DT_SINGLELINE)
			a.addHit("browse", x, y, S(480), S(24))
		}
		if a.installedCount() > 0 {
			c := cMuted
			if a.hover == "uninstall" {
				c = cAccent
			}
			p.text("Удалить русские гаджеты", x, by, S(240), bh, "body", c, DT_LEFT|DT_VCENTER|DT_SINGLELINE)
			a.addHit("uninstall", x, by, S(200), bh)
		}
		a.button(p, "cancel", "Отмена", W-S(44)-bw-S(12)-S(120), by, S(120), bh, false, true)
		a.button(p, "next", "Далее  →", W-S(44)-bw, by, bw, bh, true, len(a.targets()) > 0)

	case 1:
		y += p.text("Какие гаджеты ставить", x, y, cw, S(44), "title", cText, DT_SINGLELINE) + S(2)
		p.text("Наведите на название — ниже появится описание.", x, y, cw-S(200), S(22), "body", cMuted, DT_SINGLELINE)
		// выбрать все / снять все
		lx := x + cw - S(200)
		c1, c2 := cAccent, cAccent
		if a.hover == "all" {
			c1 = cText
		}
		if a.hover == "none" {
			c2 = cText
		}
		p.text("Выбрать все", lx, y, S(100), S(22), "body", c1, DT_RIGHT|DT_SINGLELINE)
		a.addHit("all", lx, y, S(100), S(22))
		p.text("·", lx+S(100), y, S(12), S(22), "body", cDim, DT_CENTER|DT_SINGLELINE)
		p.text("Снять все", lx+S(112), y, S(88), S(22), "body", c2, DT_RIGHT|DT_SINGLELINE)
		a.addHit("none", lx+S(112), y, S(88), S(22))
		y += S(22) + S(12)
		// сетка в две колонки
		rows := (len(gadgets) + gridCols - 1) / gridCols
		colW := cw / gridCols
		rowH := S(27)
		gy := y
		for i, g := range gadgets {
			col, row := i/rows, i%rows
			gx := x + int32(col)*colW
			yy := gy + int32(row)*rowH
			id := "opt:" + g.id
			a.checkbox(p, id, gx, yy+S(3), a.opt[g.id], false)
			tc := cText
			if a.hover == id {
				tc = cAccentH
			}
			p.text(g.title, gx+S(30), yy, colW-S(36), rowH, "body", tc, DT_LEFT|DT_VCENTER|DT_SINGLELINE|DT_END_ELLIPSIS)
			if a.inst[g.id] {
				p.circle(gx+colW-S(14), yy+rowH/2, S(3), cOk, -1)
			}
			a.addHit(id, gx, yy, colW-S(6), rowH)
		}
		y = gy + int32(rows)*rowH + S(12)
		// описание наведённого гаджета
		ph := by - S(14) - y
		p.rrect(x, y, cw, ph, S(14), cPanel, int64(cLine))
		p.rect(x, y+S(14), S(4), ph-S(28), cAccent)
		if g := a.hoverGadget(); g != nil {
			t := g.title
			switch {
			case g.kind == kindBuilt && a.inst[g.id]:
				t += "  ·  встроенный, уже на русском"
			case g.kind == kindBuilt:
				t += "  ·  встроенный в Aspire"
			case a.inst[g.id]:
				t += "  ·  уже установлен"
			default:
				t += "  ·  библиотека Vectric"
			}
			p.text(t, x+S(22), y+S(12), cw-S(40), S(24), "h", cText, DT_SINGLELINE|DT_END_ELLIPSIS)
			p.text(g.desc, x+S(22), y+S(38), cw-S(44), ph-S(46), "body", cMuted, DT_WORDBREAK)
		} else {
			n := 0
			for _, g := range gadgets {
				if a.opt[g.id] {
					n++
				}
			}
			p.text("Выбрано: "+itoa(n)+" из "+itoa(len(gadgets)), x+S(22), y+S(12), cw-S(40), S(24), "h", cText, DT_SINGLELINE)
			p.text("Встроенные гаджеты заменяются на русские (оригиналы сохраняются в папку _RU_backup_original), гаджеты из библиотеки Vectric добавляются в общую папку гаджетов. Снятая галочка у установленного гаджета вернёт ему оригинал. Зелёная точка — уже стоит.",
				x+S(22), y+S(38), cw-S(44), ph-S(46), "small", cMuted, DT_WORDBREAK)
		}
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
			fun = "Аккуратно возвращаем оригинальные гаджеты…"
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
			y += p.text("Готово!", x, y, cw, S(44), "title", cText, DT_SINGLELINE) + S(8)
			msg := "Гаджеты установлены. Запускайте Aspire — они в меню «Гаджеты» (Gadgets), уже на русском. Названия пунктов меню Aspire берёт из имён папок, поэтому они остались английскими."
			if a.mode == "restore" {
				msg = "Оригинальные гаджеты возвращены, русские версии убраны. Набор можно поставить заново в любой момент."
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
				p.text("Запустить Aspire", x+S(34), y, cw, S(24), "body", cText, DT_SINGLELINE)
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
			p.text("Русификатор Aspire, модели для ЧПУ и полезное — на aa-forma.ru", x+S(22), y+S(40), cw-S(40), S(22), "small", lc, DT_SINGLELINE|DT_END_ELLIPSIS)
			a.addHit("site2", x+S(22), y+S(40), cw-S(40), S(22))
		} else {
			y += p.text("Не получилось", x, y, cw, S(44), "title", cAccent, DT_SINGLELINE) + S(8)
			y += p.text(a.errText, x, y, cw, S(120), "body", cText, DT_WORDBREAK) + S(16)
			p.text("Ничего страшного: оригинальные гаджеты сохранены. Закройте Aspire и попробуйте ещё раз, или запустите «Удалить русские гаджеты».", x, y, cw, S(80), "small", cMuted, DT_WORDBREAK)
		}
		a.mu.Unlock()
		a.button(p, "finish", "Готово", W-S(44)-bw, by, bw, bh, true, true)
	}
}

func (a *App) hoverGadget() *gadget {
	if !strings.HasPrefix(a.hover, "opt:") {
		return nil
	}
	id := a.hover[4:]
	for i := range gadgets {
		if gadgets[i].id == id {
			return &gadgets[i]
		}
	}
	return nil
}

// ---------- сценарии ----------
func (a *App) selected(kind string) []gadget {
	var out []gadget
	for _, g := range gadgets {
		if a.opt[g.id] && (kind == "" || g.kind == kind) {
			out = append(out, g)
		}
	}
	return out
}

func (a *App) buildTasks() []task {
	dirs := a.targets()
	each := func(gs []gadget, f func(gadget, string) error) func() error {
		return func() error {
			for _, d := range dirs {
				for _, g := range gs {
					if err := f(g, d); err != nil {
						return err
					}
				}
			}
			return nil
		}
	}
	ensureDirs := func() error {
		for _, d := range dirs {
			if err := os.MkdirAll(userGadgetsDir(d), 0755); err != nil {
				return err
			}
			logf("Aspire: %s; встроенные гаджеты: %s; гаджеты пользователя: %s", d, builtinGadgetsDir(d), userGadgetsDir(d))
		}
		return nil
	}
	if a.mode == "restore" {
		var all []gadget
		all = append(all, gadgets...)
		return []task{
			{"Проверяем копии оригинальных гаджетов", 2500 * time.Millisecond, ensureDirs},
			{"Возвращаем оригинальные встроенные гаджеты", 4 * time.Second, each(all, func(g gadget, d string) error {
				if g.kind == kindBuilt {
					g.remove(d)
				}
				return nil
			})},
			{"Убираем русские гаджеты из библиотеки Vectric", 4 * time.Second, each(all, func(g gadget, d string) error {
				if g.kind != kindBuilt {
					g.remove(d)
				}
				return nil
			})},
			{"Наводим порядок", 2 * time.Second, func() error {
				for _, d := range dirs {
					cleanupBackups(d)
				}
				return nil
			}},
		}
	}
	built := a.selected(kindBuilt)
	lib := a.selected(kindUser)
	var off []gadget
	for _, g := range gadgets {
		if !a.opt[g.id] && a.inst[g.id] {
			off = append(off, g)
		}
	}
	t := []task{
		{"Проверяем Aspire и папки гаджетов", 3 * time.Second, ensureDirs},
	}
	if len(built) > 0 {
		t = append(t, task{"Сохраняем оригиналы встроенных гаджетов", 4 * time.Second, each(built, func(g gadget, d string) error { return g.ensureBackup(d) })})
		t = append(t, task{"Ставим встроенные гаджеты на русском: " + itoa(len(built)), 6 * time.Second, each(built, func(g gadget, d string) error { return g.install(d) })})
	}
	if len(lib) > 0 {
		t = append(t, task{"Ставим гаджеты из библиотеки Vectric: " + itoa(len(lib)), 6 * time.Second, each(lib, func(g gadget, d string) error { return g.install(d) })})
	}
	if len(off) > 0 {
		t = append(t, task{"Убираем снятые гаджеты, возвращаем оригиналы: " + itoa(len(off)), 3 * time.Second, each(off, func(g gadget, d string) error { g.remove(d); return nil })})
	}
	t = append(t, task{"Возвращаем права на папку настроек Aspire", 3 * time.Second, func() error {
		for _, d := range dirs {
			fixDataPerms(d)
		}
		return nil
	}})
	t = append(t, task{"Наводим порядок и проверяем результат", 3 * time.Second, func() error {
		for _, d := range dirs {
			cleanupBackups(d)
		}
		return nil
	}})
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
				logf("ОШИБКА: %s", a.errText)
				a.running = false
				a.finished = true
				a.page = 3
				a.mu.Unlock()
				a.refresh()
				saveLog()
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
		a.refresh()
		saveLog()
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
	case id == "browse":
		d := browseForAspire(a.hwnd)
		if d == "" {
			return
		}
		if !looksLikeAspire(d) {
			msgBox(a.hwnd, "В этой папке нет Aspire:\n"+d+"\n\nНужна папка, в которой лежат папки x64 и Language (обычно называется «Aspire 12.5»).", "Это не папка Aspire", MB_OK|MB_ICONWARNING)
			return
		}
		if a.manual == nil {
			a.manual = map[string]bool{}
		}
		a.manual[d] = true
		if a.addDir(d) {
			a.refresh()
		}
	case id == "next":
		a.page = 1
	case id == "back":
		a.page = 0
	case id == "all" || id == "none":
		for _, g := range gadgets {
			a.opt[g.id] = id == "all"
		}
	case strings.HasPrefix(id, "opt:"):
		k := id[4:]
		a.opt[k] = !a.opt[k]
	case id == "launch":
		a.launch = !a.launch
	case id == "install":
		if len(a.selected("")) == 0 && a.installedCount() == 0 {
			msgBox(a.hwnd, "Не выбран ни один гаджет. Отметьте, что ставить.", "Ничего не выбрано", MB_OK|MB_ICONWARNING)
			return
		}
		if !a.ensureClosed() {
			return
		}
		if len(a.targets()) == 0 {
			msgBox(a.hwnd, "Не найден Aspire 12.5. Набор сделан под Aspire 12.5 — другие версии он не трогает.", "Aspire 12.5 не найден", MB_OK|MB_ICONWARNING)
			return
		}
		a.mode = "install"
		a.start()
	case id == "uninstall":
		if msgBox(a.hwnd, "Вернуть оригинальные гаджеты и убрать русские версии?\n\nПрограмма, лицензия и ваши проекты не затрагиваются.", "Удалить русские гаджеты", MB_YESNO|MB_ICONQUESTION) != IDYES {
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

// isTarget — Aspire 12.5 или папка, которую пользователь указал сам
func (a *App) isTarget(d string) bool {
	return is125(d) || a.manual[d]
}

func (a *App) targets() []string {
	var out []string
	for _, d := range a.dirs {
		if a.isTarget(d) {
			out = append(out, d)
		}
	}
	return out
}

func (a *App) installedCount() int {
	n := 0
	for _, g := range gadgets {
		if a.inst[g.id] {
			n++
		}
	}
	return n
}

// refresh — что уже стоит; при первом запуске галочки: всё, что стоит, плюс всё, если не стоит ничего
func (a *App) refresh() {
	a.mu.Lock()
	defer a.mu.Unlock()
	first := a.inst == nil
	a.inst = map[string]bool{}
	for _, d := range a.targets() {
		for _, g := range gadgets {
			if g.installed(d) {
				a.inst[g.id] = true
			}
		}
	}
	if first {
		any := len(a.inst) > 0
		for _, g := range gadgets {
			a.opt[g.id] = !any || a.inst[g.id]
		}
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

func saveLog() {
	logMu.Lock()
	defer logMu.Unlock()
	if len(logBuf) == 0 {
		return
	}
	dir := filepath.Join(os.Getenv("LOCALAPPDATA"), "aa-forma")
	os.MkdirAll(dir, 0755)
	os.WriteFile(filepath.Join(dir, "aspire-ru-gadgets.log"), []byte(time.Now().Format("2006-01-02 15:04:05")+" · v"+version+"\r\n"+strings.Join(logBuf, "\r\n")+"\r\n"), 0644)
}

func main() {
	if pSetProcessDpiAwarenessCx.Find() == nil { // Windows 10 1703+; на Windows 7/8 просто пропускаем
		pSetProcessDpiAwarenessCx.Call(^uintptr(3)) // PER_MONITOR_AWARE_V2 (-4)
	}
	exe, _ := os.Executable()
	app.dirs = findAspire()
	app.refresh()
	app.mode = "install"

	hinst, _, _ := pGetModuleHandleW.Call(0)
	icon, _, _ := pLoadIconW.Call(hinst, 1)
	cur, _, _ := pLoadCursorW.Call(0, IDC_ARROW)
	cls := u16("AAFormaAspireRUGadgets")
	wc := WNDCLASSEX{Style: CS_HREDRAW | CS_VREDRAW, WndProc: syscall.NewCallback(wndProc), Instance: hinst, Icon: icon, IconSm: icon, Cursor: cur, ClassName: cls}
	wc.Size = uint32(unsafe.Sizeof(wc))
	pRegisterClassExW.Call(uintptr(unsafe.Pointer(&wc)))

	style := uintptr(WS_OVERLAPPED | WS_CAPTION | WS_SYSMENU | WS_MINIMIZEBOX)
	hwnd, _, _ := pCreateWindowExW.Call(0, uintptr(unsafe.Pointer(cls)), uintptr(unsafe.Pointer(u16("Гаджеты Aspire на русском — A&A мастерская форм"))),
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
