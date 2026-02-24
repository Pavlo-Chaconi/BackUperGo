//go:build linux

package tray

import "fyne.io/systray"

func setIcon(icon []byte) {
	systray.SetIcon(icon)
}
