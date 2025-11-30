// Fixed terminal_events.go - Mouse wheel scrolling with WideCharScreen integration
// FIXES: SSH resize propagation and dimension calculation
package main

import (
	"fmt"
	"log"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/driver/desktop"
)

// KEYBOARD EVENT HANDLING
func (t *NativeTerminalWidget) TypedKey(key *fyne.KeyEvent) {
	fmt.Printf("========== TypedKey ENTRY ==========\n")
	fmt.Printf("TypedKey: Key pressed: %s\n", key.Name)
	fmt.Printf("TypedKey: writeOverride is nil: %v\n", t.writeOverride == nil)
	fmt.Printf("TypedKey: ptyManager is nil: %v\n", t.ptyManager == nil)
	if t.ptyManager != nil {
		fmt.Printf("TypedKey: ptyManager.pty is nil: %v\n", t.ptyManager.pty == nil)
	}
	fmt.Printf("TypedKey: isPTYAvailable(): %v\n", t.isPTYAvailable())

	// Check if we have any PTY interface available
	if !t.isPTYAvailable() {
		fmt.Printf("TypedKey: No PTY available, ignoring\n")
		fmt.Printf("========== TypedKey EXIT (no PTY) ==========\n")
		return
	}

	var data []byte

	// Handle keys - Page Up/Down always go to applications
	switch key.Name {
	case fyne.KeyPageUp:
		// Always send to application (vim, less, etc.)
		data = []byte("\x1b[5~")

	case fyne.KeyPageDown:
		// Always send to application (vim, less, etc.)
		data = []byte("\x1b[6~")
	case fyne.KeyBackspace:
		data = []byte("\x7f")
		// CRITICAL: Force cache invalidation for backspace
		if t.screen != nil {
			t.screen.InvalidateCache()
		}
		// Force immediate update
		t.updatePending = true
		go func() {
			time.Sleep(5 * time.Millisecond)
			fyne.Do(func() {
				t.performRedrawDirect()
			})
		}()
	case fyne.KeyReturn:
		// Exit history mode on Enter in normal mode
		if t.screen != nil && !t.screen.IsUsingAlternate() && t.screen.IsViewingHistory() {
			fmt.Printf("TypedKey: Enter pressed, exiting history mode\n")
			t.exitHistoryMode()
		}
		data = []byte("\r")

	case fyne.KeyTab:
		data = []byte("\t")

	case fyne.KeyDelete:
		data = []byte("\x1b[3~")

	case fyne.KeyUp:
		data = []byte("\x1b[A")

	case fyne.KeyDown:
		data = []byte("\x1b[B")

	case fyne.KeyLeft:
		data = []byte("\x1b[D")

	case fyne.KeyRight:
		data = []byte("\x1b[C")

	case fyne.KeyHome:
		data = []byte("\x1b[H")

	case fyne.KeyEnd:
		data = []byte("\x1b[F")

	case fyne.KeyEscape:
		data = []byte("\x1b")

	case fyne.KeyF1:
		data = []byte("\x1b[11~")
	case fyne.KeyF2:
		data = []byte("\x1b[12~")
	case fyne.KeyF3:
		data = []byte("\x1b[13~")
	case fyne.KeyF4:
		data = []byte("\x1b[14~")
	case fyne.KeyF5:
		data = []byte("\x1b[15~")
	case fyne.KeyF6:
		data = []byte("\x1b[17~")
	case fyne.KeyF7:
		data = []byte("\x1b[18~")
	case fyne.KeyF8:
		data = []byte("\x1b[19~")
	case fyne.KeyF9:
		data = []byte("\x1b[20~")
	case fyne.KeyF10:
		data = []byte("\x1b[21~")
	case fyne.KeyF11:
		data = []byte("\x1b[23~")
	case fyne.KeyF12:
		data = []byte("\x1b[24~")
	}

	if len(data) > 0 {
		fmt.Printf("TypedKey: Calling WriteToPTY with %d bytes: %v\n", len(data), data)
		err := t.WriteToPTY(data)
		if err != nil {
			fmt.Printf("TypedKey: WriteToPTY error: %v\n", err)
		} else {
			fmt.Printf("TypedKey: WriteToPTY succeeded\n")
		}
		t.updatePending = true
	} else {
		fmt.Printf("TypedKey: No data to send for key: %s\n", key.Name)
	}
	fmt.Printf("========== TypedKey EXIT ==========\n")
}

func (t *NativeTerminalWidget) TypedRune(r rune) {
	fmt.Printf("========== TypedRune ENTRY ==========\n")
	fmt.Printf("TypedRune: Character typed: %c (0x%04X)\n", r, r)
	fmt.Printf("TypedRune: writeOverride is nil: %v\n", t.writeOverride == nil)
	fmt.Printf("TypedRune: ptyManager is nil: %v\n", t.ptyManager == nil)
	if t.ptyManager != nil {
		fmt.Printf("TypedRune: ptyManager.pty is nil: %v\n", t.ptyManager.pty == nil)
	}
	fmt.Printf("TypedRune: isPTYAvailable(): %v\n", t.isPTYAvailable())

	if !t.isPTYAvailable() {
		fmt.Printf("TypedRune: No PTY available, ignoring\n")
		fmt.Printf("========== TypedRune EXIT (no PTY) ==========\n")
		return
	}

	// Exit history mode on any typing in normal mode
	if t.screen != nil && !t.screen.IsUsingAlternate() && t.screen.IsViewingHistory() {
		fmt.Printf("TypedRune: Exiting history mode on character input\n")
		t.exitHistoryMode()
		time.Sleep(10 * time.Millisecond)
	}

	var data []byte

	// Handle control characters (0x01-0x1F)
	if r >= 1 && r <= 31 {
		fmt.Printf("TypedRune: Control character detected: Ctrl+%c (0x%02X)\n", r+64, r)
		data = []byte{byte(r)}

		// Force cache invalidation for control characters that might change display
		if t.screen != nil {
			t.screen.InvalidateCache()
		}
	} else if r < 32 {
		// Other control characters
		fmt.Printf("TypedRune: Special control character: 0x%02X\n", r)
		data = []byte{byte(r)}
	} else {
		// Regular printable character
		data = []byte(string(r))
	}

	fmt.Printf("TypedRune: Calling WriteToPTY with %d bytes: %v (%q)\n", len(data), data, string(data))

	err := t.WriteToPTY(data)
	if err != nil {
		fmt.Printf("TypedRune: WriteToPTY error: %v\n", err)
	} else {
		fmt.Printf("TypedRune: WriteToPTY succeeded\n")
	}

	t.updatePending = true
	fmt.Printf("========== TypedRune EXIT ==========\n")
}

// Fix 4: Immediate redraw trigger
func (t *NativeTerminalWidget) triggerImmediateRedraw() {
	// Use a goroutine to avoid blocking the input event
	go func() {
		// Small delay to ensure the PTY processes the data
		time.Sleep(5 * time.Millisecond)

		fyne.Do(func() {
			t.performRedrawDirect()
		})
	}()
}

// Fix 5: Enhanced input debugging
func (t *NativeTerminalWidget) debugKeyEvent(key *fyne.KeyEvent) {
	fmt.Printf("=== KEY EVENT DEBUG ===\n")
	fmt.Printf("Key Name: %s\n", key.Name)
	fmt.Printf("Physical key: %s\n", key.Physical)
	fmt.Printf("Has focus: %v\n", t.hasFocus)
	fmt.Printf("Widget focused: %v\n", fyne.CurrentApp().Driver().CanvasForObject(t) != nil)
	fmt.Printf("========================\n")
}

// Fix 6: Enhanced rune debugging
func (t *NativeTerminalWidget) debugRuneEvent(r rune) {
	fmt.Printf("=== RUNE EVENT DEBUG ===\n")
	fmt.Printf("Rune: %c\n", r)
	fmt.Printf("Unicode: U+%04X\n", r)
	fmt.Printf("Decimal: %d\n", r)
	fmt.Printf("Is control: %v\n", r < 32)
	fmt.Printf("Is printable: %v\n", r >= 32)
	fmt.Printf("Has focus: %v\n", t.hasFocus)
	fmt.Printf("=========================\n")
}

// Fix 7: Enhanced focus handling
func (t *NativeTerminalWidget) FocusGained() {
	fmt.Printf("FocusGained: Terminal widget gained focus\n")
	t.hasFocus = true

	// Ensure we can receive all key events
	if canvas := fyne.CurrentApp().Driver().CanvasForObject(t); canvas != nil {
		canvas.Focus(t)
		fmt.Printf("FocusGained: Explicitly focused terminal widget\n")
	}
}

// Fix 8: Custom key capture for control sequences
func (t *NativeTerminalWidget) captureRawInput() {
	// This would need to be implemented at the canvas/window level
	// For now, we'll use the existing Fyne events but process them more carefully
}

// Fix 9: Test methods for debugging input issues
func (t *NativeTerminalWidget) TestInputHandling() {
	fmt.Printf("=== TESTING INPUT HANDLING ===\n")

	// Test basic character input
	fmt.Printf("Testing character 'a'...\n")
	t.TypedRune('a')
	time.Sleep(100 * time.Millisecond)

	// Test control character
	fmt.Printf("Testing Ctrl+C (0x03)...\n")
	t.TypedRune(0x03)
	time.Sleep(100 * time.Millisecond)

	// Test backspace
	fmt.Printf("Testing backspace...\n")
	t.TypedKey(&fyne.KeyEvent{Name: fyne.KeyBackspace})
	time.Sleep(100 * time.Millisecond)

	// Test escape sequence
	fmt.Printf("Testing arrow up...\n")
	t.TypedKey(&fyne.KeyEvent{Name: fyne.KeyUp})
	time.Sleep(100 * time.Millisecond)

	fmt.Printf("Input handling test completed\n")
	fmt.Printf("===============================\n")
}

// Fix 10: Better update processor with immediate mode
func (t *NativeTerminalWidget) enhancedUpdateProcessor() {
	ticker := time.NewTicker(16 * time.Millisecond) // 60 FPS for responsive input
	defer ticker.Stop()

	var lastUpdateTime time.Time
	updateCooldown := 8 * time.Millisecond // Faster updates for input responsiveness

	log.Printf("Enhanced update processor started for responsive input")

	for {
		select {
		case <-ticker.C:
			now := time.Now()
			if t.updatePending && now.Sub(lastUpdateTime) >= updateCooldown {
				// Prioritize immediate updates for input events
				go func() {
					fyne.Do(func() {
						t.performRedrawDirect()
					})
				}()

				t.updatePending = false
				lastUpdateTime = now
			}
		case <-t.ctx.Done():
			log.Printf("Enhanced update processor stopping")
			return
		}
	}
}

// MOUSE EVENT HANDLING - Implements desktop.Mouseable
func (t *NativeTerminalWidget) MouseDown(event *desktop.MouseEvent) {
	fmt.Printf("MouseDown: position=%v, button=%v\n", event.Position, event.Button)

	// Request focus on click
	if canvas := fyne.CurrentApp().Driver().CanvasForObject(t); canvas != nil {
		canvas.Focus(t)
	}
}

func (t *NativeTerminalWidget) MouseUp(event *desktop.MouseEvent) {
	fmt.Printf("MouseUp: position=%v\n", event.Position)
}

// SCROLL HANDLING - Implements fyne.Scrollable
func (t *NativeTerminalWidget) Scrolled(event *fyne.ScrollEvent) {
	fmt.Printf("Scrolled event: DY=%.2f\n", event.Scrolled.DY)
	t.handleScrollEvent(event)
}

// handleScrollEvent processes mouse wheel events
func (t *NativeTerminalWidget) handleScrollEvent(event *fyne.ScrollEvent) bool {
	if t.screen == nil {
		return false
	}

	now := time.Now()

	// Debounce rapid scroll events
	if now.Sub(t.lastScrollTime) < 50*time.Millisecond {
		return true
	}
	t.lastScrollTime = now

	fmt.Printf("handleScrollEvent: DY=%.2f, IsUsingAlternate=%v\n",
		event.Scrolled.DY, t.screen.IsUsingAlternate())

	// DEBUG: Log state before scroll
	t.debugScrollEvent("BEFORE", 0)

	// In alternate screen (vim), don't handle scroll
	if t.screen.IsUsingAlternate() {
		fmt.Printf("handleScrollEvent: in alternate screen, letting application handle\n")
		return false
	}

	// Normal mode: handle mouse wheel scrolling for history
	scrollLines := 3
	if absFloat32(event.Scrolled.DY) > 5 {
		scrollLines = 5 // Faster scroll for larger movements
	}

	if event.Scrolled.DY > 0.1 {
		// Scroll up (into history) - use WideCharScreen directly
		fmt.Printf("handleScrollEvent: MOUSE WHEEL UP by %d lines\n", scrollLines)

		// DEBUG: Check before scroll up
		beforePos := t.screen.GetHistoryPos()
		beforeMax := t.screen.GetHistorySize()
		fmt.Printf("Before scroll up: %d/%d\n", beforePos, beforeMax)

		t.screen.ScrollUp(scrollLines)
		t.updatePending = true

		// DEBUG: Check after scroll up and log what happened
		t.debugScrollEvent("UP", scrollLines)

		return true
	} else if event.Scrolled.DY < -0.1 {
		// Scroll down (towards current) - use WideCharScreen directly
		fmt.Printf("handleScrollEvent: MOUSE WHEEL DOWN by %d lines\n", scrollLines)

		// DEBUG: Check before scroll down
		beforePos := t.screen.GetHistoryPos()
		beforeMax := t.screen.GetHistorySize()
		fmt.Printf("Before scroll down: %d/%d\n", beforePos, beforeMax)

		t.screen.ScrollDown(scrollLines)
		t.updatePending = true

		// DEBUG: Check after scroll down and log what happened
		t.debugScrollEvent("DOWN", scrollLines)

		return true
	}

	return false
}

// Add this method to debug scroll limits
func (t *NativeTerminalWidget) debugScrollLimits() {
	historySize := t.screen.GetHistorySize()
	historyPos := t.screen.GetHistoryPos()
	maxPos := t.screen.GetMaxHistoryPos()
	isAtTop := t.screen.IsAtTopOfHistory()
	isAtBottom := t.screen.IsAtBottomOfHistory()

	log.Printf("SCROLL LIMITS DEBUG:")
	log.Printf("  - History size: %d lines", historySize)
	log.Printf("  - Current position: %d/%d", historyPos, maxPos)
	log.Printf("  - At top: %v", isAtTop)
	log.Printf("  - At bottom: %v", isAtBottom)

	if historySize == 0 {
		log.Printf("  - WARNING: NO HISTORY: Cannot scroll into history")
	} else if historySize < 100 {
		log.Printf("  - WARNING: LIMITED HISTORY: Only %d lines available", historySize)
	}

	// Check theoretical vs actual scrollable range
	allLines := t.screen.GetDisplay()
	theoreticalTotal := historySize + t.rows
	actualTotal := len(allLines)

	log.Printf("  - Theoretical total lines: %d", theoreticalTotal)
	log.Printf("  - Actual displayed lines: %d", actualTotal)

	if actualTotal < theoreticalTotal {
		missing := theoreticalTotal - actualTotal
		log.Printf("  - WARNING: MISSING CONTENT: %d lines not accessible", missing)
	}
}

// SIMPLE HISTORY MODE EXIT

func (t *NativeTerminalWidget) exitHistoryMode() {
	if t.screen != nil {
		log.Printf("Exiting history mode - returning to current output")
		t.screen.ScrollToBottom()
		t.updatePending = true
	}
}

// RESIZE HANDLING

func (t *NativeTerminalWidget) handleResize(width, height float32) {
	t.resizeMutex.Lock()
	defer t.resizeMutex.Unlock()

	if width == t.lastWidth && height == t.lastHeight {
		return
	}

	t.lastWidth = width
	t.lastHeight = height

	if t.resizeTimer != nil {
		t.resizeTimer.Stop()
	}

	t.resizeTimer = time.AfterFunc(150*time.Millisecond, func() {
		t.performResize(width, height)
	})
}

func (t *NativeTerminalWidget) performResize(width, height float32) {
	newCols, newRows := t.CalculateTerminalSize(width, height)

	t.mutex.Lock()
	currentCols, currentRows := t.cols, t.rows
	needsResize := newCols != currentCols || newRows != currentRows

	if needsResize {
		log.Printf("performResize: from %dx%d to %dx%d (widget: %.1fx%.1f)",
			currentCols, currentRows, newCols, newRows, width, height)

		// Update terminal dimensions
		t.cols = newCols
		t.rows = newRows

		// Update virtual scroll viewport size if it exists
		if hasVirtualScroll(t) {
			t.virtualScroll.visibleLines = newRows
		}

		// Resize the underlying terminal screen (gopyte)
		func() {
			defer func() {
				if r := recover(); r != nil {
					log.Printf("Error resizing screen: %v", r)
				}
			}()
			t.screen.Resize(newCols, newRows)
		}()
	}
	t.mutex.Unlock()

	if needsResize {
		// *** FIX: Call the resize callback if set (for SSH sessions) ***
		if t.onResizeCallback != nil {
			log.Printf("performResize: calling onResizeCallback for SSH")
			t.onResizeCallback(newCols, newRows)
		} else {
			// Fall back to local PTY resize
			go t.performPTYResize(newRows, newCols)
		}

		// Force immediate redraw with new dimensions
		t.updatePending = true
	}
}

func (t *NativeTerminalWidget) performPTYResize(newRows, newCols int) {
	// Use your existing ResizePTY method if available
	func() {
		defer func() {
			if r := recover(); r != nil {
				log.Printf("Error during PTY resize: %v", r)
			}
		}()

		// Try to call your ResizePTY method
		err := t.ResizePTY(newCols, newRows)
		if err != nil {
			log.Printf("Failed to resize PTY: %v", err)
		} else {
			log.Printf("PTY resized to %dx%d", newCols, newRows)
		}
	}()

	// Force viewport recalculation after PTY resize
	t.recalculateViewport()

	// Trigger multiple display updates to ensure everything syncs
	t.updatePending = true
	time.Sleep(100 * time.Millisecond)
	t.updatePending = true
}

// SIZING AND UTILITY METHODS

func (t *NativeTerminalWidget) CalculateTerminalSize(width, height float32) (int, int) {
	// Ensure we have valid character dimensions
	if t.charWidth <= 0 || t.charHeight <= 0 {
		log.Printf("CalculateTerminalSize: invalid char dimensions (%.2f x %.2f), using defaults",
			t.charWidth, t.charHeight)
		return 80, 24
	}

	// Account for padding/margins in the terminal widget
	// Fyne TextGrid typically has some internal padding
	const horizontalPadding float32 = 4.0 // Left + right padding
	const verticalPadding float32 = 2.0   // Top + bottom padding (minimal)

	usableWidth := width - horizontalPadding
	usableHeight := height - verticalPadding

	// Ensure we don't go negative
	if usableWidth < 0 {
		usableWidth = width
	}
	if usableHeight < 2 {
		usableHeight = height - 2
	}
	// Calculate columns and rows
	
	cols := int(usableWidth / t.charWidth)
	rows := int(usableHeight / t.charHeight)
    settings := GetSettings().Get()
	rows = rows - settings.RowOffset
	cols = cols - settings.ColOffset
	// Apply reasonable limits
	if cols < 10 {
		cols = 10
	} else if cols > 500 {
		cols = 500
	}

	if rows < 3 {
		rows = 3
	} else if rows > 200 {
		rows = 200
	}

	log.Printf("CalculateTerminalSize: window=%.1fx%.1f, charSize=%.2fx%.2f -> %dx%d",
		width, height, t.charWidth, t.charHeight, cols, rows)
	return cols, rows
}

func (t *NativeTerminalWidget) recalculateViewport() {
	t.mutex.Lock()
	defer t.mutex.Unlock()

	// Update virtual scroll state with current dimensions if it exists
	if hasVirtualScroll(t) {
		t.virtualScroll.visibleLines = t.rows
	}

	log.Printf("Recalculated viewport: visibleLines=%d, cols=%d", t.rows, t.cols)

	// Trigger display update
	t.updatePending = true
}

func (t *NativeTerminalWidget) forceScrollToBottom() {
	t.mutex.Lock()
	defer t.mutex.Unlock()

	if t.screen != nil {
		// Exit any history mode
		if t.screen.IsViewingHistory() {
			t.screen.ScrollToBottom()
			log.Printf("forceScrollToBottom: Exited history mode")
		}

		// Force display update
		t.updatePending = true

		// Ensure scroll container also scrolls to bottom if it exists
		if t.scroll != nil {
			go func() {
				time.Sleep(10 * time.Millisecond)
				fyne.Do(func() {
					t.scroll.ScrollToBottom()
				})
			}()
		}
	}
}

// HELPER FUNCTIONS

// isPTYAvailable checks if any PTY interface is available
func (t *NativeTerminalWidget) isPTYAvailable() bool {
	// SSH connections use writeOverride instead of ptyManager
	if t.writeOverride != nil {
		fmt.Printf("isPTYAvailable: writeOverride is set, returning true\n")
		return true
	}

	hasPTY := t.ptyManager != nil && t.ptyManager.pty != nil
	fmt.Printf("isPTYAvailable: ptyManager check = %v\n", hasPTY)
	return hasPTY
}

// hasVirtualScroll checks if virtual scroll field exists
func hasVirtualScroll(t *NativeTerminalWidget) bool {
	defer func() {
		recover() // Ignore if field doesn't exist
	}()
	return t.virtualScroll != (VirtualScrollState{})
}

// UTILITY FUNCTIONS

func absFloat32(x float32) float32 {
	if x < 0 {
		return -x
	}
	return x
}

// *** NEW: SetResizeCallback allows SSH widgets to receive resize events ***
func (t *NativeTerminalWidget) SetResizeCallback(callback func(cols, rows int)) {
	t.onResizeCallback = callback
	log.Printf("SetResizeCallback: resize callback registered")
}
