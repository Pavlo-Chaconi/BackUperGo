//go:build windows

package tray

import "syscall"

var (
	user32         = syscall.NewLazyDLL("user32.dll")
	kernel32       = syscall.NewLazyDLL("kernel32.dll")
	procShowWindow = user32.NewProc("ShowWindow")
	procGetConsole = kernel32.NewProc("GetConsoleWindow")
)

const (
	swHide = 0
	swShow = 5
)

func HideConsole() {
	if err := procGetConsole.Find(); err != nil {
		return
	}
	if err := procShowWindow.Find(); err != nil {
		return
	}
	hwnd, _, _ := procGetConsole.Call()
	if hwnd == 0 {
		return
	}
	procShowWindow.Call(hwnd, swHide)
}

func ShowConsole() {
	if err := procGetConsole.Find(); err != nil {
		return
	}
	if err := procShowWindow.Find(); err != nil {
		return
	}
	hwnd, _, _ := procGetConsole.Call()
	if hwnd == 0 {
		return
	}
	procShowWindow.Call(hwnd, swShow)
}
