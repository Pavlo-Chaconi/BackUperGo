//go:build windows

package tray

import "fyne.io/systray"

func setIcon(icon []byte) {
	systray.SetIcon(icon)
}
