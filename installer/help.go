package main

import (
	"archive/zip"
	"bytes"
	_ "embed"
	"io"
	"os"
	"path/filepath"
	"strings"
)

//go:embed help.zip
var helpZip []byte

const helpMarker = "_ru_translation.txt"

func installHelp(dir string) error {
	dst := filepath.Join(dir, "Language", "RUS", "Help")
	if _, err := os.Stat(dst); err == nil {
		if _, err := os.Stat(filepath.Join(dst, helpMarker)); err != nil {
			logln("Папка RUS\\Help уже есть и не наша — справку не трогаю.")
			return nil
		}
		os.RemoveAll(dst)
	}
	zr, err := zip.NewReader(bytes.NewReader(helpZip), int64(len(helpZip)))
	if err != nil {
		return err
	}
	n := 0
	for _, f := range zr.File {
		p := filepath.Join(dst, filepath.FromSlash(f.Name))
		if !strings.HasPrefix(p, dst) {
			continue
		}
		if f.FileInfo().IsDir() {
			os.MkdirAll(p, 0755)
			continue
		}
		os.MkdirAll(filepath.Dir(p), 0755)
		rc, err := f.Open()
		if err != nil {
			return err
		}
		w, err := os.Create(p)
		if err != nil {
			rc.Close()
			return err
		}
		io.Copy(w, rc)
		w.Close()
		rc.Close()
		n++
	}
	logf("Русская справка установлена (%d файлов)\n", n-1)
	return nil
}

func removeHelp(dir string) {
	dst := filepath.Join(dir, "Language", "RUS", "Help")
	if _, err := os.Stat(filepath.Join(dst, helpMarker)); err == nil {
		os.RemoveAll(dst)
		logln("Русская справка удалена")
	}
}
