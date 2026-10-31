// Русификатор Vectric Aspire 12.5 от A&A — мастерская форм. Логика установки.
package main

import (
	"time"
	"crypto/sha256"
	"embed"
	"encoding/hex"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
)

//go:embed payload
var payload embed.FS

// Варианты Resources_RUS.dll: пункты выпадающих списков Windows читает в кодировке системы (ANSI).
// 1251 — обычная русская Windows (основной файл из payload), 65001 — включён «UTF-8 (бета)»,
// иначе (английская/европейская кодировка) — пункты списков оставляем английскими, чтобы не было «����».
//go:embed dllvar
var dllvar embed.FS

const rusDLL = "payload/RUS/x64/Resources_RUS.dll"

func systemACP() uint32 {
	r, _, _ := kernel32.NewProc("GetACP").Call()
	return uint32(r)
}

func dllVariant() (string, string) {
	switch acp := systemACP(); acp {
	case 1251:
		return rusDLL, "кириллица (1251)"
	case 65001:
		return "dllvar/utf8.dll", "UTF-8"
	default:
		return "dllvar/en.dll", fmt.Sprintf("кодовая страница %d — пункты некоторых списков останутся английскими", acp)
	}
}

func variantBytes(p string) []byte {
	if strings.HasPrefix(p, "dllvar/") {
		b, _ := dllvar.ReadFile(p)
		return b
	}
	b, _ := payload.ReadFile(p)
	return b
}

const (
	enuHash      = "7db9b0b1f001254db0ed561190bf665988d671654c8ec49c3800e2fb91cec29b" // Resources_ENU.dll из Aspire 12.5
	origRusHash  = "eaf66ac97c36005b02f7ca1558950d49fa69963e2d188e4c959bbefb0c62b9f9" // заводской Resources_RUS.dll
	backupFolder = "_RU_backup_original"
	version      = "2.6.3"
)

// ---------- журнал (выводится в окне установки) ----------
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

func sha(path string) string {
	f, err := os.Open(path)
	if err != nil {
		return ""
	}
	defer f.Close()
	h := sha256.New()
	io.Copy(h, f)
	return hex.EncodeToString(h.Sum(nil))
}

func payloadHash(p string) string {
	b := variantBytes(p)
	s := sha256.Sum256(b)
	return hex.EncodeToString(s[:])
}

func findAspire() []string {
	var roots []string
	for _, env := range []string{"ProgramFiles", "ProgramW6432", "ProgramFiles(x86)"} {
		if v := os.Getenv(env); v != "" {
			roots = append(roots, v)
		}
	}
	roots = append(roots, `C:\Program Files`)
	seen := map[string]bool{}
	var out []string
	for _, r := range roots {
		m, _ := filepath.Glob(filepath.Join(r, "Aspire*"))
		for _, d := range m {
			k := strings.ToLower(d)
			if seen[k] {
				continue
			}
			seen[k] = true
			if _, err := os.Stat(filepath.Join(d, "Language", "RUS", "x64", "Resources_RUS.dll")); err == nil {
				out = append(out, d)
			}
		}
	}
	return out
}

// is125 — папка от Aspire 12.5 (по имени «Aspire 12.5…»). Перевод сделан только под 12.5:
// другие версии (10.5, 11.x, 12.0), стоящие рядом, не трогаем вообще.
func is125(dir string) bool {
	n := strings.ToLower(filepath.Base(dir))
	return strings.Contains(n, "12.5") || versionOK(dir)
}

// repairOtherVersion — чинит другую версию Aspire (9.5, 10.5…), в которую старый русификатор
// поставил файлы от 12.5: возвращает её оригиналы из нашей копии и убирает остальное.
// Копию не удаляем, а переименовываем — на всякий случай.
func repairOtherVersion(dir string) error {
	if err := doRestore(dir); err != nil {
		return err
	}
	renameTextures(dir, false)
	translateToolDB(dir, false)
	removeHelp(dir)
	lang := filepath.Join(dir, "Language")
	os.Rename(filepath.Join(lang, backupFolder), filepath.Join(lang, backupFolder+"_restored_"+time.Now().Format("20060102_150405")))
	logf("Вернули родные файлы: %s", dir)
	return nil
}

func versionOK(dir string) bool {
	return sha(filepath.Join(dir, "Language", "ENU", "x64", "Resources_ENU.dll")) == enuHash
}

func isInstalled(dir string) bool {
	h := sha(filepath.Join(dir, "Language", "RUS", "x64", "Resources_RUS.dll"))
	for _, p := range []string{rusDLL, "dllvar/utf8.dll", "dllvar/en.dll"} {
		if h == payloadHash(p) {
			return true
		}
	}
	return false
}

func hasBackup(dir string) bool {
	_, err := os.Stat(filepath.Join(dir, "Language", backupFolder))
	return err == nil
}

func aspireRunning() bool {
	cmd := exec.Command("tasklist", "/FI", "IMAGENAME eq Aspire.exe", "/NH")
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
	out, err := cmd.Output()
	return err == nil && strings.Contains(strings.ToLower(string(out)), "aspire.exe")
}

// payload path -> path relative to <Aspire>\Language
func targets() map[string]string {
	m := map[string]string{}
	fs.WalkDir(payload, "payload", func(p string, d fs.DirEntry, err error) error {
		if err == nil && !d.IsDir() {
			m[p] = filepath.FromSlash(strings.TrimPrefix(p, "payload/"))
		}
		return nil
	})
	return m
}

func copyFile(src, dst string) error {
	b, err := os.ReadFile(src)
	if err != nil {
		return err
	}
	os.MkdirAll(filepath.Dir(dst), 0755)
	os.Chmod(dst, 0666)
	return os.WriteFile(dst, b, 0644)
}

func doBackup(dir string) error {
	lang := filepath.Join(dir, "Language")
	bak := filepath.Join(lang, backupFolder)
	if _, err := os.Stat(bak); err == nil {
		logln("Копия оригинальных файлов уже есть — оставляем её как есть")
		return nil
	}
	if isInstalled(dir) {
		logln("Русификатор уже стоит — копию не перезаписываем")
		return nil
	}
	n := 0
	for _, rel := range targets() {
		src := filepath.Join(lang, rel)
		if _, err := os.Stat(src); err == nil {
			if err := copyFile(src, filepath.Join(bak, rel)); err != nil {
				return fmt.Errorf("не удалось сделать копию %s: %v", rel, err)
			}
			n++
		}
	}
	logf("Сохранили копию %d оригинальных файлов", n)
	return nil
}

func doInterface(dir string) error {
	lang := filepath.Join(dir, "Language")
	n := 0
	vp, vname := dllVariant()
	acp := systemACP()
	for p, rel := range targets() {
		b, _ := payload.ReadFile(p)
		if p == rusDLL {
			b = variantBytes(vp)
		}
		// «Local» в базе инструментов Aspire выводит в кодировке системы — кладём строку в нужной
		if strings.HasSuffix(p, "/AppCmd.mo") {
			switch acp {
			case 1251:
				b = variantBytes("dllvar/AppCmd_1251.mo")
			case 65001:
			default:
				b = variantBytes("dllvar/AppCmd_en.mo")
			}
		}
		dst := filepath.Join(lang, rel)
		os.MkdirAll(filepath.Dir(dst), 0755)
		os.Chmod(dst, 0666)
		if err := os.WriteFile(dst, b, 0644); err != nil {
			return fmt.Errorf("не удалось записать %s: %v", rel, err)
		}
		n++
	}
	logf("Интерфейс: обновлено %d файлов перевода", n)
	logf("Кодировка системы: %s", vname)
	return nil
}

func doRestore(dir string) error {
	lang := filepath.Join(dir, "Language")
	bak := filepath.Join(lang, backupFolder)
	if _, err := os.Stat(bak); err != nil {
		return fmt.Errorf("копия оригинальных файлов не найдена (%s)", bak)
	}
	n := 0
	err := filepath.WalkDir(bak, func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		rel, _ := filepath.Rel(bak, p)
		if err := copyFile(p, filepath.Join(lang, rel)); err != nil {
			return err
		}
		n++
		return nil
	})
	if err != nil {
		return err
	}
	logf("Вернули %d оригинальных файлов", n)
	return nil
}

func shortcutPath() string {
	pub := os.Getenv("PUBLIC")
	if pub == "" {
		pub = `C:\Users\Public`
	}
	return filepath.Join(pub, "Desktop", "Aspire (RU).lnk")
}

func makeShortcut(dir string) {
	lnk := shortcutPath()
	exe := filepath.Join(dir, "x64", "Aspire.exe")
	ico := filepath.Join(dir, "Icons", "app_icon.ico")
	q := func(s string) string { return "'" + strings.ReplaceAll(s, "'", "''") + "'" }
	ps := "$s=(New-Object -ComObject WScript.Shell).CreateShortcut(" + q(lnk) + ");" +
		"$s.TargetPath=" + q(exe) + ";$s.Arguments='/lang=RUS';$s.WorkingDirectory=" + q(filepath.Dir(exe)) + ";" +
		"$s.IconLocation=" + q(ico) + ";$s.Save()"
	cmd := exec.Command("powershell", "-NoProfile", "-NonInteractive", "-ExecutionPolicy", "Bypass", "-Command", ps)
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
	if err := cmd.Run(); err == nil {
		logln("Ярлык «Aspire (RU)» появился на рабочем столе")
	} else {
		logln("Ярлык создать не получилось — запускайте через Language\\RUS\\Aspire.lnk")
	}
}

// запуск через проводник, чтобы Aspire работал с обычными правами, а не от администратора
func launchAspire(dir string) {
	if _, err := os.Stat(shortcutPath()); err == nil {
		exec.Command("explorer.exe", shortcutPath()).Start()
		return
	}
	lnk := filepath.Join(dir, "Language", "RUS", "Aspire.lnk")
	exec.Command("explorer.exe", lnk).Start()
}
