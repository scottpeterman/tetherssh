package main

import (
	"fmt"
	"log"
	"runtime"
	"time"

	  "fyne.io/fyne/v2"
  "fyne.io/fyne/v2/app"
  "fyne.io/fyne/v2/dialog"
)

func main() {
	log.Printf("Starting TetherSSH on %s", runtime.GOOS)

	myApp := app.New()
	darkMode := true
	myApp.Settings().SetTheme(NewNativeTheme(darkMode))

	myWindow := myApp.NewWindow(fmt.Sprintf("TetherSSH - %s", runtime.GOOS))
	myWindow.Resize(fyne.NewSize(1200, 800))

	// Create session manager
	sessionManager := NewSessionManager(myWindow)
	myWindow.SetContent(sessionManager.GetContainer())

	// SINGLE, CORRECT close interceptor with confirm dialog
	myWindow.SetCloseIntercept(func() {
		sm := sessionManager

		sm.tabsMutex.RLock()
		activeCount := len(sm.activeTabs)
		sm.tabsMutex.RUnlock()

		if activeCount == 0 {
			sm.DisconnectAll()
			myApp.Quit()
			return
		}

		dialog.ShowConfirm(
			"Close TetherSSH",
			fmt.Sprintf("You have %d active session(s).\n\nClose anyway?", activeCount),
			func(confirmed bool) {
				if confirmed {
					sm.DisconnectAll()
					time.AfterFunc(100*time.Millisecond, myApp.Quit)
				}
			},
			myWindow,
		)
	})

	log.Printf("Starting session manager interface...")
	myWindow.ShowAndRun()
}