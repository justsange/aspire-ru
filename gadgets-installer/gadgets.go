// Установщик русских гаджетов для Vectric Aspire 12.5 — A&A, мастерская форм.
// Логика: встроенные гаджеты Aspire заменяются на месте (ProgramData) с копией оригиналов,
// гаджеты из библиотеки Vectric ставятся в папку гаджетов пользователя (общие документы).
package main

import (
	"crypto/sha256"
	"embed"
	"encoding/hex"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
)

//go:embed all:gadget
var gadgetFS embed.FS

const (
	version   = "1.0"
	marker    = "_RU_aa-forma.txt"
	bakName   = "_RU_backup_original" // папка с оригиналами рядом с гаджетами
	kindBuilt = "builtin"             // ProgramData\Vectric\Aspire\V12.5\Gadgets
	kindUser  = "user"                // Public\Documents\Vectric Files\Gadgets\Aspire V12.5
)

type gadget struct {
	id    string // папка в gadget/
	title string // короткое имя для списка
	desc  string // одна строка описания
	kind  string
	file  bool // одиночный .lua в корне папки гаджетов (About_Gadgets, DXF_Batch_Processor)
}

// Порядок = порядок в списке установщика.
var gadgets = []gadget{
	{"Chamfer", "Снятие фаски", "Траектория фаски вдоль выбранных контуров обычной фрезой, без специального инструмента.", kindUser, false},
	{"Dado_Toolpath_Creator", "Пазы", "Траектории прямоугольных пазов под полки и перегородки — по выбранным прямоугольникам.", kindUser, false},
	{"Dogbone_Filleter", "«Косточки» в углах", "Находит внутренние углы и делает выборки-«косточки», чтобы прямоугольная деталь входила в паз.", kindUser, false},
	{"Keyhole_Toolpath", "Замочная скважина", "Паз «замочная скважина» для подвешивания изделия на стену.", kindBuilt, false},
	{"Fluter", "Канавки", "Рисунок из повторяющихся канавок — для декоративных панелей и филёнок.", kindUser, false},
	{"Texture_Toolbox", "Инструменты текстуры", "Дополняет векторную текстуру: меняет глубину и направление реза для живого рисунка.", kindUser, false},
	{"Halftoner", "Полутоновая обработка", "По рельефу строит траекторию из отверстий разного размера — полутоновая картинка.", kindUser, false},
	{"Turned_Profile", "Точёный профиль", "Траектория по профилю детали для обработки на поворотной оси.", kindUser, false},
	{"Wrapping", "Поворотная ось (3 гаджета)", "Три гаджета для цилиндра: раскладка канавок, спирали и округление квадратной заготовки.", kindBuilt, false},
	{"Dragknife_Toolpath", "Траектория ножа", "Резка плёнки и картона тангенциальным ножом с учётом его поворота в углах.", kindBuilt, false},
	{"Box_Creator", "Коробка (Box Creator)", "Коробка на шипах или «ласточкином хвосте» с крышкой и дном, с траекториями.", kindUser, false},
	{"Simple_Box_Creator_12.6_RU", "Конструктор коробок 12.6", "Развитый конструктор: сдвижная крышка, ручки, вдвижное дно. Авторы gremlin529 и SharkCutUp, бета.", kindUser, false},
	{"Celtic_Weave_Creator", "Кельтское плетение", "Узор кельтского переплетения по сетке с выбором профиля.", kindBuilt, false},
	{"Spiral", "Спирали", "Векторные спирали: Архимеда, логарифмическая, эвольвента, золотая.", kindUser, false},
	{"Circle_Resize", "Диаметр окружностей", "Меняет диаметр всех выбранных окружностей заданного размера.", kindUser, false},
	{"Square_Around", "Рамка вокруг объектов", "Прямоугольник вокруг выбранного с заданным отступом.", kindUser, false},
	{"Explode_Vectors", "Разбить векторы", "Разбивает контуры на отдельные отрезки и дуги.", kindBuilt, false},
	{"DXF_Batch_Processor", "Пакетный импорт DXF", "Импортирует все DXF из папки и раскладывает их в проекте без наложения.", kindBuilt, true},
	{"Regular_Grid_Importer", "Импорт регулярной сетки", "Строит рельеф из данных регулярной сетки (ASC, DEM, HGT, PTC).", kindUser, false},
	{"Shopbot_Probe_Importer", "Щуп ShopBot", "Импорт данных измерений, полученных щупом ShopBot.", kindUser, false},
	{"Job_Backup", "Резервные копии проекта", "Сохранение копии проекта в выбранную папку и «сохранить копию как».", kindUser, false},
	{"Setup_Sheet_Editor", "Редактор листа настройки", "Логотип, цвета, заголовок и разделы листа настройки проекта.", kindBuilt, false},
	{"Global_Gadget_Settings", "Общие настройки гаджетов", "Показывать ли окна гаджетов, которые это поддерживают.", kindBuilt, false},
	{"About_Gadgets", "О гаджетах", "Окно сведений о гаджетах.", kindBuilt, true},
}

// ---------- журнал ----------
var (
	logMu  sync.Mutex
	logBuf []string
)

func logln(a ...interface{}) {
	logMu.Lock()
	logBuf = append(logBuf, strings.TrimSpace(fmt.Sprint(a...)))
	logMu.Unlock()
}
func logf(f string, a ...interface{}) { logln(fmt.Sprintf(f, a...)) }

// ---------- где что лежит ----------
func builtinGadgetsDir(aspireDir string) string {
	return filepath.Join(aspireDataDir(aspireDir), "Gadgets")
}

func userGadgetsDir(aspireDir string) string {
	pub := os.Getenv("PUBLIC")
	if pub == "" {
		pub = `C:\Users\Public`
	}
	return filepath.Join(pub, "Documents", "Vectric Files", "Gadgets", "Aspire V"+aspireVer(aspireDir))
}

func (g gadget) root(aspireDir string) string {
	if g.kind == kindBuilt {
		return builtinGadgetsDir(aspireDir)
	}
	return userGadgetsDir(aspireDir)
}

// dest — что именно заменяем: папка гаджета или одиночный .lua
func (g gadget) dest(aspireDir string) string {
	if g.file {
		return filepath.Join(g.root(aspireDir), g.id+".lua")
	}
	return filepath.Join(g.root(aspireDir), g.id)
}

func (g gadget) backup(aspireDir string) string {
	if g.file {
		return filepath.Join(g.root(aspireDir), bakName, g.id+".lua")
	}
	return filepath.Join(g.root(aspireDir), bakName, g.id)
}

func (g gadget) installed(aspireDir string) bool {
	if g.file {
		b, err := os.ReadFile(g.dest(aspireDir))
		return err == nil && strings.Contains(string(b), "A&A")
	}
	_, err := os.Stat(filepath.Join(g.dest(aspireDir), marker))
	return err == nil
}

func anyInstalled(aspireDir string) bool {
	for _, g := range gadgets {
		if g.installed(aspireDir) {
			return true
		}
	}
	return false
}

func exists(p string) bool { _, err := os.Stat(p); return err == nil }

// ---------- установка ----------
// ensureBackup — оригинал (если он есть и это не наша версия) убираем в копию. Один раз:
// при переустановке копия уже лежит, и нашу версию поверх неё не пишем.
func (g gadget) ensureBackup(aspireDir string) error {
	dst := g.dest(aspireDir)
	bak := g.backup(aspireDir)
	if !exists(dst) || g.installed(aspireDir) {
		return nil
	}
	if exists(bak) {
		return nil
	}
	os.MkdirAll(filepath.Dir(bak), 0755)
	if err := os.Rename(dst, bak); err != nil {
		// другой диск/права — копируем
		if err := copyTree(dst, bak); err != nil {
			return fmt.Errorf("не удалось сохранить оригинал %s: %v", g.id, err)
		}
	}
	logf("%s: оригинал сохранён → %s", g.title, bak)
	return nil
}

func (g gadget) install(aspireDir string) error {
	if err := g.ensureBackup(aspireDir); err != nil {
		return err
	}
	dst := g.dest(aspireDir)
	os.RemoveAll(dst)
	n := 0
	if g.file {
		b, _ := gadgetFS.ReadFile("gadget/" + g.id + "/" + g.id + ".lua")
		os.MkdirAll(filepath.Dir(dst), 0755)
		if err := os.WriteFile(dst, b, 0644); err != nil {
			return err
		}
		n = 1
	} else {
		err := fs.WalkDir(gadgetFS, "gadget/"+g.id, func(p string, d fs.DirEntry, err error) error {
			if err != nil || d.IsDir() {
				return err
			}
			rel := strings.TrimPrefix(p, "gadget/"+g.id+"/")
			b, _ := gadgetFS.ReadFile(p)
			out := filepath.Join(dst, filepath.FromSlash(rel))
			os.MkdirAll(filepath.Dir(out), 0755)
			if err := os.WriteFile(out, b, 0644); err != nil {
				return err
			}
			n++
			return nil
		})
		if err != nil {
			return fmt.Errorf("%s: %v", g.id, err)
		}
	}
	logf("%s: установлен (%d файлов) → %s", g.title, n, dst)
	return nil
}

// remove — убираем нашу версию и возвращаем оригинал, если он был
func (g gadget) remove(aspireDir string) {
	dst := g.dest(aspireDir)
	bak := g.backup(aspireDir)
	if !g.installed(aspireDir) {
		return
	}
	os.RemoveAll(dst)
	if exists(bak) {
		if err := os.Rename(bak, dst); err != nil {
			copyTree(bak, dst)
			os.RemoveAll(bak)
		}
		logf("%s: возвращён оригинал", g.title)
	} else {
		logf("%s: удалён", g.title)
	}
}

func copyTree(src, dst string) error {
	st, err := os.Stat(src)
	if err != nil {
		return err
	}
	if !st.IsDir() {
		b, err := os.ReadFile(src)
		if err != nil {
			return err
		}
		os.MkdirAll(filepath.Dir(dst), 0755)
		return os.WriteFile(dst, b, 0644)
	}
	return filepath.Walk(src, func(p string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(src, p)
		out := filepath.Join(dst, rel)
		if info.IsDir() {
			return os.MkdirAll(out, 0755)
		}
		b, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		return os.WriteFile(out, b, 0644)
	})
}

// cleanupBackups — если пустые папки копий остались, убираем
func cleanupBackups(aspireDir string) {
	for _, r := range []string{builtinGadgetsDir(aspireDir), userGadgetsDir(aspireDir)} {
		os.Remove(filepath.Join(r, bakName)) // удалится только пустая
	}
}

// ---------- прочее ----------
func aspireRunning() bool {
	cmd := exec.Command("tasklist", "/FI", "IMAGENAME eq Aspire.exe", "/NH")
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
	out, err := cmd.Output()
	return err == nil && strings.Contains(strings.ToLower(string(out)), "aspire.exe")
}

func launchAspire(dir string) {
	// через проводник — чтобы Aspire не унаследовал права администратора
	lnk := filepath.Join(dir, "Language", "RUS", "Aspire.lnk")
	if !exists(lnk) {
		lnk = filepath.Join(dir, "x64", "Aspire.exe")
	}
	exec.Command("explorer.exe", lnk).Start()
}

func sha(b []byte) string { s := sha256.Sum256(b); return hex.EncodeToString(s[:]) }

// is125 — папка Aspire 12.5 (по имени)
func is125(dir string) bool {
	return strings.Contains(strings.ToLower(filepath.Base(dir)), "12.5")
}

// findAspire — Program Files на системном диске, реестр, остальные диски
func findAspire() []string {
	var roots []string
	for _, e := range []string{"ProgramW6432", "ProgramFiles"} {
		if v := os.Getenv(e); v != "" {
			roots = append(roots, v)
		}
	}
	seen := map[string]bool{}
	var out, cands []string
	for _, r := range roots {
		m, _ := filepath.Glob(filepath.Join(r, "Aspire*"))
		cands = append(cands, m...)
	}
	cands = append(cands, registryCandidates()...)
	cands = append(cands, extraCandidates()...)
	for _, d := range cands {
		k := strings.ToLower(d)
		if seen[k] {
			continue
		}
		seen[k] = true
		if looksLikeAspire(d) {
			out = append(out, d)
		}
	}
	return out
}
