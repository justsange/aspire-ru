package main

// Перевод стандартного содержимого базы инструментов (tools.vtdb — это SQLite).
// Меняем только заводские английские названия по точному совпадению, поэтому
// собственные фрезы пользователя и режимы резания не затрагиваются, а удаление
// русификации возвращает английские названия обратной заменой.
// SQLite берём из самой Windows (winsqlite3.dll, есть в Windows 10/11).

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"unsafe"
)

var (
	winsqlite      = syscall.NewLazyDLL("winsqlite3.dll")
	pSqliteOpenV2  = winsqlite.NewProc("sqlite3_open_v2")
	pSqliteExec    = winsqlite.NewProc("sqlite3_exec")
	pSqliteClose   = winsqlite.NewProc("sqlite3_close")
	pSqliteBusy    = winsqlite.NewProc("sqlite3_busy_timeout")
	pSqliteChanges = winsqlite.NewProc("sqlite3_total_changes")
	pSqliteErrmsg  = winsqlite.NewProc("sqlite3_errmsg")
)

type tdbCol struct{ table, col string }

var tdbCols = []tdbCol{
	{"material", "name"}, {"machine", "name"},
	{"tool_tree_entry", "name"}, {"tool_tree_entry", "notes"},
	{"tool_geometry", "name_format"},
}

var tdbMap = [][2]string{
	// материалы
	{"MDF", "МДФ"}, {"Softwood", "Мягкая древесина"}, {"Hardwood", "Твёрдая древесина"}, {"Acrylic", "Акрил"},
	// станки
	{"Desktop", "Настольный"}, {"Large", "Большой"},
	// группы
	{"Imperial Tools", "Дюймовые инструменты"}, {"Metric Tools", "Метрические инструменты"},
	{"End Mills", "Концевые фрезы"}, {"Ball Nose", "Сферические фрезы"}, {"V-Bits", "V-образные фрезы"},
	{"Form Tools", "Фасонные фрезы"}, {"Engraving", "Гравировальные"}, {"Specialist", "Специальные"}, {"Drills", "Свёрла"},
	// описания групп
	{"Default sample tool set, defined using inch units.", "Стандартный набор инструментов в дюймах."},
	{"Default sample tool set, defined using metric units.", "Стандартный набор инструментов в миллиметрах."},
	{"Any tool with straight side and a flat bottom.", "Фрезы с прямой боковой стенкой и плоским торцом."},
	{"Any tool with a straight side and a radiused bottom.", "Фрезы с прямой боковой стенкой и скруглённым торцом."},
	{"Any tool with one or more angled cutting edges that comes to a sharp central point.", "Фрезы с одной или несколькими наклонными режущими кромками, сходящимися в острую вершину."},
	{"An irregular profiled tool.\r\nTo create:  Draw a single vector that defines the right hand half of the tool,\r\nfrom the lower centerpoint of the tool to the top right edge.\r\nthe profile may not double back from the furtherst right hand point toward the center.",
		"Фреза с произвольным профилем.\r\nКак создать: нарисуйте один вектор, описывающий правую половину фрезы,\r\nот нижней центральной точки до верхнего правого края.\r\nПрофиль не должен возвращаться к центру после самой правой точки."},
	{"Any tool that has a flat bottom and an angled cutting edge.", "Фрезы с плоским торцом и наклонной режущей кромкой."},
	{"Standard drill tools.", "Стандартные свёрла."},
	// шаблоны имён фрез ({Tool Type} и единицы Aspire подставляет сам, уже по-русски)
	{"{Tool Type} ({Included Angle|.0}° - {Diameter|F}{Units Short} - {Line Width}{Units Short} Line Width)",
		"{Tool Type} ({Included Angle|.0}° - {Diameter|F}{Units Short} - ширина линии {Line Width}{Units Short})"},
	{"{Tool Type} ({Side Angle|.0}°, Tip {Flat Diameter} - {Diameter|F}{Units Short})",
		"{Tool Type} ({Side Angle|.0}°, вершина {Flat Diameter} - {Diameter|F}{Units Short})"},
	{`Ogee - 1/4" Rads 1 1/4" Dia 1/2" Deep`, `S-образная (Ogee) - радиус 1/4", диаметр 1 1/4", глубина 1/2"`},
	{`Roundover - 3/8" Rad 1" Dia x 1/2" Deep`, `Радиусная - радиус 3/8", диаметр 1", глубина 1/2"`},
	{"Ogee - 6mm Rads 32mm Dia 12mm Deep", "S-образная (Ogee) - радиус 6 мм, диаметр 32 мм, глубина 12 мм"},
	{"Roundover -  9mm Rad 25mm Dia 12mm Deep", "Радиусная - радиус 9 мм, диаметр 25 мм, глубина 12 мм"},
}

func toolDBFiles(aspireDir string) []string {
	pd := os.Getenv("ProgramData")
	if pd == "" {
		pd = `C:\ProgramData`
	}
	ver := strings.TrimSpace(strings.TrimPrefix(filepath.Base(aspireDir), "Aspire"))
	var out []string
	for _, n := range []string{"tools.vtdb", "tools.vtdb.vectric"} {
		p := filepath.Join(pd, "Vectric", "Aspire", "V"+ver, "ToolDatabase", n)
		if _, err := os.Stat(p); err == nil {
			out = append(out, p)
		}
	}
	return out
}

func cstr(s string) *byte { b := append([]byte(s), 0); return &b[0] }

func gostr(p uintptr) string {
	if p == 0 {
		return ""
	}
	var b []byte
	for i := uintptr(0); ; i++ {
		c := *(*byte)(unsafe.Pointer(p + i))
		if c == 0 {
			break
		}
		b = append(b, c)
	}
	return string(b)
}

func sqlq(s string) string { return "'" + strings.ReplaceAll(s, "'", "''") + "'" }

func tdbSQL(toRu bool) string {
	var sb strings.Builder
	sb.WriteString("BEGIN;")
	for _, c := range tdbCols {
		for _, m := range tdbMap {
			from, to := m[0], m[1]
			if !toRu {
				from, to = to, from
			}
			fmt.Fprintf(&sb, "UPDATE OR IGNORE \"%s\" SET \"%s\"=%s WHERE \"%s\"=%s;", c.table, c.col, sqlq(to), c.col, sqlq(from))
		}
	}
	sb.WriteString("COMMIT;")
	return sb.String()
}

func translateToolDB(aspireDir string, toRu bool) {
	if err := pSqliteOpenV2.Find(); err != nil {
		logln("База инструментов: в системе нет winsqlite3.dll — пропускаем")
		return
	}
	for _, path := range toolDBFiles(aspireDir) {
		if toRu {
			bak := path + ".ru_backup"
			if _, err := os.Stat(bak); err != nil {
				copyFile(path, bak)
			}
		}
		os.Chmod(path, 0666)
		var db uintptr
		r, _, _ := pSqliteOpenV2.Call(uintptr(unsafe.Pointer(cstr(path))), uintptr(unsafe.Pointer(&db)), 0x2 /*READWRITE*/, 0)
		if r != 0 {
			logf("База инструментов: не открылась %s (код %d)", filepath.Base(path), r)
			if db != 0 {
				pSqliteClose.Call(db)
			}
			continue
		}
		pSqliteBusy.Call(db, 5000)
		var errp uintptr
		r, _, _ = pSqliteExec.Call(db, uintptr(unsafe.Pointer(cstr(tdbSQL(toRu)))), 0, 0, uintptr(unsafe.Pointer(&errp)))
		if r != 0 {
			m, _, _ := pSqliteErrmsg.Call(db)
			logf("База инструментов: ошибка в %s: %s", filepath.Base(path), gostr(m))
			pSqliteExec.Call(db, uintptr(unsafe.Pointer(cstr("ROLLBACK;"))), 0, 0, 0)
		} else {
			n, _, _ := pSqliteChanges.Call(db)
			if toRu {
				logf("База инструментов %s: переведено %d названий", filepath.Base(path), n)
			} else {
				logf("База инструментов %s: возвращено %d английских названий", filepath.Base(path), n)
			}
		}
		pSqliteClose.Call(db)
	}
}
