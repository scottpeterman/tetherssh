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

	// Initialize settings first
	settings := GetSettings()

	myApp := app.NewWithID("com.github.scottpeterman.tetherssh")
	myApp.SetIcon(resourceLogoPng)


	// Apply theme from settings
	myApp.Settings().SetTheme(NewNativeTheme(settings.Get().DarkTheme))

	myWindow := myApp.NewWindow(fmt.Sprintf("TetherSSH - %s", runtime.GOOS))
	myWindow.SetIcon(resourceLogoPng)

	// Apply saved window size from settings
	s := settings.Get()
	if s.RememberWindowSize && s.WindowWidth > 0 && s.WindowHeight > 0 {
		myWindow.Resize(fyne.NewSize(float32(s.WindowWidth), float32(s.WindowHeight)))
	} else {
		myWindow.Resize(fyne.NewSize(1200, 800))
	}

	// Create session manager
	sessionManager := NewSessionManager(myWindow)
	myWindow.SetContent(sessionManager.GetContainer())

	// Set up settings save callback for live theme updates
	settings.SetOnSave(func(newSettings *AppSettings) {
		myApp.Settings().SetTheme(NewNativeTheme(newSettings.DarkTheme))
		log.Printf("Settings updated - theme applied")
	})

	// Close interceptor with window size saving
	myWindow.SetCloseIntercept(func() {
		sm := sessionManager

		// Save window size if enabled
		if settings.Get().RememberWindowSize {
			size := myWindow.Canvas().Size()
			settings.Get().WindowWidth = int(size.Width)
			settings.Get().WindowHeight = int(size.Height)
			settings.Save()
		}

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