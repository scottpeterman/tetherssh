package main

import (
	"fmt"
	"runtime"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"
)

// MemoryMonitor provides real-time memory usage display
type MemoryMonitor struct {
	label    *widget.Label
	stopChan chan struct{}
}

// NewMemoryMonitor creates a new memory monitor widget
func NewMemoryMonitor() *MemoryMonitor {
	mm := &MemoryMonitor{
		label:    widget.NewLabel(formatMemStats()),
		stopChan: make(chan struct{}),
	}
	mm.label.TextStyle = fyne.TextStyle{Monospace: true}
	return mm
}

// Start begins periodic memory updates
func (mm *MemoryMonitor) Start(interval time.Duration) {
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				text := formatMemStats()
				fyne.Do(func() {
					mm.label.SetText(text)
				})
			case <-mm.stopChan:
				return
			}
		}
	}()
}

// Stop stops the memory monitor
func (mm *MemoryMonitor) Stop() {
	select {
	case <-mm.stopChan:
		// Already closed
	default:
		close(mm.stopChan)
	}
}

// GetWidget returns the label widget for embedding in UI
func (mm *MemoryMonitor) GetWidget() *widget.Label {
	return mm.label
}

// formatMemStats returns a formatted memory statistics string
func formatMemStats() string {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)

	return fmt.Sprintf("Mem: %.1f MB | GC: %d | Routines: %d",
		float64(m.Alloc)/1024/1024,
		m.NumGC,
		runtime.NumGoroutine(),
	)
}

// CreateStatusBar creates a status bar with memory monitor and timestamp
func CreateStatusBar(mm *MemoryMonitor) *fyne.Container {
	timestamp := widget.NewLabel(time.Now().Format("2006/01/02 15:04:05"))
	timestamp.TextStyle = fyne.TextStyle{Monospace: true}

	// Update timestamp every second
	go func() {
		ticker := time.NewTicker(1 * time.Second)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				text := time.Now().Format("2006/01/02 15:04:05")
				fyne.Do(func() {
					timestamp.SetText(text)
				})
			case <-mm.stopChan:
				return
			}
		}
	}()

	// Status message area (for future use)
	statusMsg := widget.NewLabel("")
	statusMsg.TextStyle = fyne.TextStyle{Italic: true}

	// Layout: [status message] --- [spacer] --- [memory] --- [timestamp]
	return container.NewBorder(
		nil, nil,
		statusMsg,
		container.NewHBox(mm.GetWidget(), widget.NewSeparator(), timestamp),
	)
}