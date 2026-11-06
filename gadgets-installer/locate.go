package main

// Поиск Aspire за пределами Program Files на системном диске: по всем дискам,
// по реестру (запись установки Vectric) и вручную — через диалог выбора папки.

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"syscall"
	"unsafe"
)

// aspireVer — «12.5» из имени папки «Aspire 12.5»; если папка названа иначе (D:\Vectric\Aspire),
// считаем 12.5 — другие версии установщик всё равно не трогает.
func aspireVer(aspireDir string) string {
	b := filepath.Base(aspireDir)
	if m := regexp.MustCompile(`\d+(\.\d+)?`).FindString(b); m != "" {
		return m
	}
	return "12.5"
}

// looksLikeAspire — в папке есть сам Aspire и его языковые файлы
func looksLikeAspire(dir string) bool {
	if _, err := os.Stat(filepath.Join(dir, "x64", "Aspire.exe")); err != nil {
		return false
	}
	_, err := os.Stat(filepath.Join(dir, "Language", "RUS", "x64", "Resources_RUS.dll"))
	return err == nil
}

// driveRoots — только локальные жёсткие диски C:..Z: (сетевые и съёмные не трогаем,
// отключённый сетевой диск может «думать» десятки секунд)
func driveRoots() []string {
	var out []string
	for c := 'C'; c <= 'Z'; c++ {
		r := string(c) + `:\`
		t, _, _ := pGetDriveTypeW.Call(uintptr(unsafe.Pointer(u16(r))))
		if t == 3 { // DRIVE_FIXED
			out = append(out, r)
		}
	}
	return out
}

// extraCandidates — где ещё люди ставят Aspire: корень диска, Program Files на другом диске,
// папки Vectric / Programs / Soft и т. п.
func extraCandidates() []string {
	var out []string
	subs := []string{"", "Program Files", "Program Files (x86)", "Vectric", "Programs", "Program", "Soft", "Software", "Games", "CNC", "ЧПУ", "Программы"}
	for _, r := range driveRoots() {
		for _, s := range subs {
			m, _ := filepath.Glob(filepath.Join(r, s, "Aspire*"))
			out = append(out, m...)
		}
	}
	return out
}

// registryCandidates — InstallLocation из списка установленных программ
func registryCandidates() []string {
	var out []string
	for _, root := range []string{
		`SOFTWARE\Microsoft\Windows\CurrentVersion\Uninstall`,
		`SOFTWARE\WOW6432Node\Microsoft\Windows\CurrentVersion\Uninstall`,
	} {
		var hk syscall.Handle
		if syscall.RegOpenKeyEx(syscall.HKEY_LOCAL_MACHINE, u16(root), 0, syscall.KEY_READ, &hk) != nil {
			continue
		}
		for i := uint32(0); i < 4096; i++ {
			var name [256]uint16
			n := uint32(len(name))
			if syscall.RegEnumKeyEx(hk, i, &name[0], &n, nil, nil, nil, nil) != nil {
				break
			}
			sub := syscall.UTF16ToString(name[:n])
			var sk syscall.Handle
			if syscall.RegOpenKeyEx(hk, u16(root+`\`+sub), 0, syscall.KEY_READ, &sk) != nil {
				continue
			}
			dn := regString(sk, "DisplayName")
			if strings.Contains(strings.ToLower(dn), "aspire") {
				if loc := regString(sk, "InstallLocation"); loc != "" {
					out = append(out, strings.TrimRight(loc, `\`))
				}
			}
			syscall.RegCloseKey(sk)
		}
		syscall.RegCloseKey(hk)
	}
	return out
}

func regString(k syscall.Handle, name string) string {
	var typ, n uint32
	if syscall.RegQueryValueEx(k, u16(name), nil, &typ, nil, &n) != nil || n == 0 || n > 4096 {
		return ""
	}
	buf := make([]uint16, n/2+1)
	if syscall.RegQueryValueEx(k, u16(name), nil, &typ, (*byte)(unsafe.Pointer(&buf[0])), &n) != nil {
		return ""
	}
	return syscall.UTF16ToString(buf)
}

// browseForAspire — системный диалог «Обзор папок». Возвращает "" при отмене.
func browseForAspire(hwnd uintptr) string {
	type browseInfo struct {
		hwndOwner      uintptr
		pidlRoot       uintptr
		pszDisplayName *uint16
		lpszTitle      *uint16
		ulFlags        uint32
		lpfn           uintptr
		lParam         uintptr
		iImage         int32
	}
	disp := make([]uint16, 260)
	bi := browseInfo{
		hwndOwner:      hwnd,
		pszDisplayName: &disp[0],
		lpszTitle:      u16("Выберите папку, в которую установлен Aspire 12.5 (внутри есть папки x64 и Language)"),
		ulFlags:        0x40 | 0x10 | 0x1, // BIF_NEWDIALOGSTYLE | BIF_EDITBOX | BIF_RETURNONLYFSDIRS
	}
	pCoInitialize.Call(0) // новый стиль диалога требует COM; повторный вызов безвреден
	pidl, _, _ := pSHBrowseForFolderW.Call(uintptr(unsafe.Pointer(&bi)))
	if pidl == 0 {
		return ""
	}
	path := make([]uint16, 1024)
	ok, _, _ := pSHGetPathFromIDListW.Call(pidl, uintptr(unsafe.Pointer(&path[0])))
	pCoTaskMemFree.Call(pidl)
	if ok == 0 {
		return ""
	}
	return strings.TrimRight(syscall.UTF16ToString(path), `\`)
}

// addDir — добавить папку в список найденных (без дублей)
func (a *App) addDir(d string) bool {
	k := strings.ToLower(d)
	for _, e := range a.dirs {
		if strings.ToLower(e) == k {
			return false
		}
	}
	a.dirs = append(a.dirs, d)
	return true
}
