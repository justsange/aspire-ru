//go:build !box

package main

// Сборка без гаджета «Конструктор коробок» (он идёт отдельной веткой, пока не дотестирован).
const boxAvailable = false

func installGadget(string) error  { return nil }
func removeGadget(string)         {}
func gadgetInstalled(string) bool { return false }
