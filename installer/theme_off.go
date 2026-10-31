//go:build !dark

package main

// Сборка без тёмной темы (публичная версия)
const darkAvailable = false

func themeBackupPath(dir string) string { return "" }
func installDarkTheme(dir string) error { return nil }
func removeDarkTheme(dir string)        {}
