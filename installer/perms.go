package main

// Права на пользовательские данные Aspire в ProgramData (база фрез, текстуры, гаджеты, настройки).
// Если Aspire ставили или копировали от администратора, папка принадлежит администраторам,
// обычный пользователь не может писать в tools.vtdb, и Aspire пишет «База инструментов открыта
// только для чтения». Установщик работает от администратора, поэтому возвращает группе
// «Пользователи» право изменять эту папку и снимает атрибут «только чтение».

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
)

func aspireDataDir(aspireDir string) string {
	pd := os.Getenv("ProgramData")
	if pd == "" {
		pd = `C:\ProgramData`
	}
	ver := strings.TrimSpace(strings.TrimPrefix(filepath.Base(aspireDir), "Aspire"))
	return filepath.Join(pd, "Vectric", "Aspire", "V"+ver)
}

func hidden(name string, args ...string) *exec.Cmd {
	c := exec.Command(name, args...)
	c.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
	return c
}

func fixDataPerms(aspireDir string) error {
	dir := aspireDataDir(aspireDir)
	if _, err := os.Stat(dir); err != nil {
		logf("Папка данных Aspire не найдена (%s) — права не трогаем", dir)
		return nil
	}
	// S-1-5-32-545 = BUILTIN\Users на любом языке Windows; (OI)(CI)M = изменение, наследуется файлами и папками
	if out, err := hidden("icacls", dir, "/grant", "*S-1-5-32-545:(OI)(CI)M", "/T", "/C", "/Q").CombinedOutput(); err != nil {
		logf("Права на %s: icacls не отработал (%v): %s", dir, err, strings.TrimSpace(string(out)))
	} else {
		logf("Группе «Пользователи» возвращено право изменять %s", dir)
	}
	// снимаем «только чтение» с базы фрез и остального
	hidden("attrib", "-R", filepath.Join(dir, "*"), "/S", "/D").Run()
	return nil
}
