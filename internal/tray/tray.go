package tray

import "fyne.io/systray"

// Run starts the system tray and blocks until it exits.
func Run(onOpen func(), onExit func()) {
	systray.Run(func() {
		mOpen := systray.AddMenuItem("Open", "Show console")
		mExit := systray.AddMenuItem("Exit", "Exit application")

		go func() {
			for {
				select {
				case <-mOpen.ClickedCh:
					if onOpen != nil {
						onOpen()
					}
					systray.Quit()
					return
				case <-mExit.ClickedCh:
					if onExit != nil {
						onExit()
					}
					systray.Quit()
					return
				}
			}
		}()
	}, func() {})
}
