// Enhanced terminal_display.go with comprehensive debugging
package main

import (
	"image/color"
	"log"
	"strings"

	"tetherssh/internal/gopyte"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/widget"
)

type VirtualScrollState struct {
	totalLines    int     // Total content lines available
	visibleLines  int     // Lines visible in viewport
	scrollOffset  int     // Current scroll position (line number)
	maxScroll     int     // Maximum scroll position
	contentHeight float32 // Height of all content if rendered
}

// DEBUG: Add detailed buffer state logging
func (t *NativeTerminalWidget) logBufferState(context string) {
	t.mutex.RLock()
	defer t.mutex.RUnlock()

	allLines := t.screen.GetDisplay()
	historySize := t.screen.GetHistorySize()
	isAlternate := t.screen.IsUsingAlternate()
	isViewingHistory := t.screen.IsViewingHistory()

	// Get additional state from WideCharScreen
	historyPos := t.screen.GetHistoryPos()
	maxHistoryPos := t.screen.GetMaxHistoryPos()
	totalContentLines := historySize + t.rows

	log.Printf("=== BUFFER STATE DEBUG [%s] ===", context)
	log.Printf("Terminal dimensions: %dx%d (cols x rows)", t.cols, t.rows)
	log.Printf("Display lines returned: %d", len(allLines))
	log.Printf("History size: %d lines", historySize)
	log.Printf("History position: %d/%d", historyPos, maxHistoryPos)
	log.Printf("Total content lines: %d", totalContentLines)
	log.Printf("Using alternate screen: %v", isAlternate)
	log.Printf("Viewing history: %v", isViewingHistory)

	// Show actual available scrollable area
	if !isAlternate {
		maxScrollableLines := historySize + t.rows
		actualViewableLines := len(allLines)
		scrollableRatio := float64(actualViewableLines) / float64(maxScrollableLines) * 100

		log.Printf("SCROLL ANALYSIS:")
		log.Printf("  - Max scrollable content: %d lines", maxScrollableLines)
		log.Printf("  - Actually viewable: %d lines (%.1f%%)", actualViewableLines, scrollableRatio)
		log.Printf("  - Potentially missing: %d lines", maxScrollableLines-actualViewableLines)

		if actualViewableLines < maxScrollableLines {
			log.Printf("  - WARNING: LIMITED VIEWPORT DETECTED!")
		}
	}

	// Debug viewport calculation in WideCharScreen
	t.screen.DebugViewportState()

	log.Printf("================================")
}

// Main redraw function with enhanced debugging
func (t *NativeTerminalWidget) performRedrawDirect() {
	// Log buffer state before processing
	t.logBufferState("BEFORE_REDRAW")

	t.mutex.RLock()

	// Get display data
	allLines := t.screen.GetDisplay()
	allAttrs := t.screen.GetAttributes()
	isUsingAlternate := t.screen.IsUsingAlternate()

	t.mutex.RUnlock()

	log.Printf("performRedrawDirect: Got %d lines, alternate=%v", len(allLines), isUsingAlternate)

	// Route to appropriate renderer
	if isUsingAlternate {
		t.renderAlternateScreen(allLines, allAttrs)
	} else {
		shouldAutoScroll := !t.IsInHistoryMode()
		t.renderNormalMode(allLines, allAttrs, shouldAutoScroll)
	}

	// Log buffer state after processing
	t.logBufferState("AFTER_REDRAW")
}

// Alternate screen renderer (vim, htop, less)
func (t *NativeTerminalWidget) renderAlternateScreen(allLines []string, allAttrs [][]gopyte.Attributes) {
	log.Printf("ALTERNATE: Rendering full screen mode with %d lines", len(allLines))

	// Size TextGrid to exact screen dimensions
	screenSize := fyne.NewSize(
		float32(t.cols)*t.charWidth,
		float32(t.rows)*t.charHeight,
	)

	t.textGrid.Resize(screenSize)

	// Prepare display lines
	displayLines := make([]string, t.rows)
	for i := 0; i < t.rows; i++ {
		if i < len(allLines) {
			displayLines[i] = allLines[i]
		} else {
			displayLines[i] = ""
		}
	}

	// Pad lines to exact column width
	for i := range displayLines {
		runes := []rune(displayLines[i])
		if len(runes) < t.cols {
			padding := strings.Repeat(" ", t.cols-len(runes))
			displayLines[i] = displayLines[i] + padding
		} else if len(runes) > t.cols {
			displayLines[i] = string(runes[:t.cols])
		}
	}

	// Place cursor
	cursorX, cursorY := t.screen.GetCursor()
	if cursorY >= 0 && cursorY < len(displayLines) && cursorX >= 0 && cursorX < t.cols {
		t.placeCursorInLine(&displayLines[cursorY], cursorX)
		log.Printf("ALTERNATE: Cursor at (%d,%d)", cursorX, cursorY)
	}

	// Set content
	fullText := strings.Join(displayLines, "\n")
	t.textGrid.SetText(fullText)

	// Apply colors
	if len(allAttrs) > 0 {
		t.applyColors(displayLines, allAttrs)
	} else {
		t.textGrid.Refresh()
	}

	log.Printf("ALTERNATE: Rendered %d lines", len(displayLines))
}

// Enhanced normal mode renderer with detailed viewport debugging
func (t *NativeTerminalWidget) renderNormalMode(allLines []string, allAttrs [][]gopyte.Attributes, shouldAutoScroll bool) {
	log.Printf("NORMAL: Rendering %d lines, autoScroll=%v", len(allLines), shouldAutoScroll)

	if len(allLines) == 0 {
		t.textGrid.SetText("")
		return
	}

	// Calculate viewport with detailed logging
	viewport := t.calculateVirtualViewport(allLines)
	t.logViewportCalculation(viewport, len(allLines))

	// Size TextGrid - POTENTIAL ISSUE: This might be too restrictive
	viewportSize := fyne.NewSize(
		float32(t.cols)*t.charWidth,
		float32(viewport.visibleLines)*t.charHeight,
	)
	t.textGrid.Resize(viewportSize)

	log.Printf("NORMAL: TextGrid resized to %.1fx%.1f (for %d visible lines)",
		viewportSize.Width, viewportSize.Height, viewport.visibleLines)

	// Extract visible content
	visibleLines := t.extractVisibleContent(allLines, viewport)

	// Place cursor if visible
	cursorX, cursorY := t.screen.GetCursor()
	adjustedCursorY := t.adjustCursorForViewport(cursorX, cursorY, viewport, len(allLines))

	if adjustedCursorY >= 0 && adjustedCursorY < len(visibleLines) && cursorX >= 0 && cursorX < t.cols && !t.IsInHistoryMode() {
		t.placeCursorInLine(&visibleLines[adjustedCursorY], cursorX)
		log.Printf("NORMAL: Cursor at (%d,%d) in viewport", cursorX, adjustedCursorY)
	}

	// Set visible content
	fullText := strings.Join(visibleLines, "\n")
	t.textGrid.SetText(fullText)

	// Apply colors to visible content
	if len(allAttrs) > 0 {
		visibleAttrs := t.extractVisibleAttributes(allAttrs, viewport)
		t.applyColors(visibleLines, visibleAttrs)
	} else {
		t.textGrid.Refresh()
	}

	log.Printf("NORMAL: Rendered viewport lines %d-%d of %d total",
		viewport.scrollOffset, viewport.scrollOffset+viewport.visibleLines-1, len(allLines))
}

// Enhanced viewport calculation with detailed debugging
func (t *NativeTerminalWidget) calculateVirtualViewport(allLines []string) VirtualScrollState {
	totalLines := len(allLines)
	visibleLines := t.rows

	// DEBUG: Check if t.rows is limiting us
	log.Printf("VIEWPORT CALC: t.rows=%d, but do we have container space for more?", t.rows)

	if visibleLines <= 0 {
		visibleLines = 24
		log.Printf("VIEWPORT CALC: t.rows was %d, defaulting to %d", t.rows, visibleLines)
	}

	// Check if we're artificially limiting the viewport
	historySize := t.screen.GetHistorySize()
	theoreticalMax := historySize + t.rows

	log.Printf("VIEWPORT CALC ANALYSIS:")
	log.Printf("  - totalLines returned by GetDisplay(): %d", totalLines)
	log.Printf("  - t.rows (terminal height): %d", t.rows)
	log.Printf("  - historySize: %d", historySize)
	log.Printf("  - theoretical max content: %d", theoreticalMax)
	log.Printf("  - visibleLines (viewport): %d", visibleLines)

	if totalLines > visibleLines*2 {
		log.Printf("  - WARNING: LARGE BUFFER: %d lines available but viewport only %d", totalLines, visibleLines)
	}

	var scrollOffset int

	if t.IsInHistoryMode() {
		// History mode scrolling
		historySize := t.GetHistorySize()
		currentPos := 0
		if t.screen != nil && t.screen.HistoryScreen != nil {
			currentPos = t.screen.HistoryScreen.HistoryPos
		}

		if totalLines <= visibleLines {
			scrollOffset = 0
		} else {
			maxScrollOffset := totalLines - visibleLines
			if historySize > 0 {
				scrollOffset = maxScrollOffset - ((currentPos * maxScrollOffset) / historySize)
			} else {
				scrollOffset = maxScrollOffset
			}

			if currentPos >= historySize {
				scrollOffset = 0
			}

			if scrollOffset < 0 {
				scrollOffset = 0
			}
			if scrollOffset > maxScrollOffset {
				scrollOffset = maxScrollOffset
			}
		}

		log.Printf("HISTORY MODE: pos=%d/%d -> scrollOffset=%d", currentPos, historySize, scrollOffset)
	} else {
		// Normal mode: show bottom
		if totalLines <= visibleLines {
			scrollOffset = 0
		} else {
			scrollOffset = totalLines - visibleLines
		}
		log.Printf("NORMAL MODE: scrollOffset=%d (showing bottom %d of %d)", scrollOffset, visibleLines, totalLines)
	}

	maxScroll := totalLines - visibleLines
	if maxScroll < 0 {
		maxScroll = 0
	}

	return VirtualScrollState{
		totalLines:    totalLines,
		visibleLines:  visibleLines,
		scrollOffset:  scrollOffset,
		maxScroll:     maxScroll,
		contentHeight: float32(totalLines) * t.charHeight,
	}
}

// New method to log viewport calculation details
func (t *NativeTerminalWidget) logViewportCalculation(viewport VirtualScrollState, totalLinesAvailable int) {
	log.Printf("VIEWPORT RESULT:")
	log.Printf("  - totalLines: %d", viewport.totalLines)
	log.Printf("  - visibleLines: %d", viewport.visibleLines)
	log.Printf("  - scrollOffset: %d", viewport.scrollOffset)
	log.Printf("  - maxScroll: %d", viewport.maxScroll)
	log.Printf("  - contentHeight: %.1f", viewport.contentHeight)
	log.Printf("  - showing lines [%d-%d] of %d available",
		viewport.scrollOffset,
		viewport.scrollOffset+viewport.visibleLines-1,
		totalLinesAvailable)

	// Flag potential issues
	utilizationPercent := float64(viewport.visibleLines) / float64(viewport.totalLines) * 100
	if utilizationPercent < 50 {
		log.Printf("  - WARNING: LOW UTILIZATION: Only showing %.1f%% of available content", utilizationPercent)
	}

	if viewport.visibleLines < t.rows {
		log.Printf("  - WARNING: VIEWPORT SMALLER THAN TERMINAL: viewport=%d < t.rows=%d", viewport.visibleLines, t.rows)
	}
}

// Enhanced method to check actual container dimensions vs terminal dimensions
func (t *NativeTerminalWidget) debugContainerDimensions(context string) {
	if t.textGrid == nil {
		log.Printf("DEBUG CONTAINER [%s]: textGrid is nil", context)
		return
	}

	containerSize := t.textGrid.Size()
	parentSize := fyne.NewSize(0, 0)

	// Try to get parent container size if available
	if t.scroll != nil {
		parentSize = t.scroll.Size()
	}

	availableRows := int(containerSize.Height / t.charHeight)
	parentRows := int(parentSize.Height / t.charHeight)

	log.Printf("DEBUG CONTAINER [%s]:", context)
	log.Printf("  - TextGrid size: %.1fx%.1f", containerSize.Width, containerSize.Height)
	log.Printf("  - Parent size: %.1fx%.1f", parentSize.Width, parentSize.Height)
	log.Printf("  - TextGrid rows capacity: %d", availableRows)
	log.Printf("  - Parent rows capacity: %d", parentRows)
	log.Printf("  - t.rows setting: %d", t.rows)
	log.Printf("  - charHeight: %.1f", t.charHeight)

	if availableRows > t.rows {
		log.Printf("  - OPPORTUNITY: Container can fit %d rows but t.rows only %d", availableRows, t.rows)
	}
}

// Extract visible content from viewport
func (t *NativeTerminalWidget) extractVisibleContent(allLines []string, viewport VirtualScrollState) []string {
	visibleContent := make([]string, viewport.visibleLines)

	for i := 0; i < viewport.visibleLines; i++ {
		lineIndex := viewport.scrollOffset + i
		if lineIndex < len(allLines) {
			visibleContent[i] = allLines[lineIndex]
		} else {
			visibleContent[i] = ""
		}
	}

	return visibleContent
}

// Extract visible attributes
func (t *NativeTerminalWidget) extractVisibleAttributes(allAttrs [][]gopyte.Attributes, viewport VirtualScrollState) [][]gopyte.Attributes {
	if len(allAttrs) == 0 {
		return [][]gopyte.Attributes{}
	}

	visibleAttrs := make([][]gopyte.Attributes, viewport.visibleLines)
	for i := 0; i < viewport.visibleLines; i++ {
		lineIndex := viewport.scrollOffset + i
		if lineIndex < len(allAttrs) {
			visibleAttrs[i] = allAttrs[lineIndex]
		}
	}

	return visibleAttrs
}

// Adjust cursor position for viewport
func (t *NativeTerminalWidget) adjustCursorForViewport(cursorX, cursorY int, viewport VirtualScrollState, totalLines int) int {
	if t.IsInHistoryMode() {
		return -1 // Don't show cursor in history mode
	}

	if totalLines <= viewport.visibleLines {
		return cursorY
	} else {
		actualCursorLine := totalLines - viewport.visibleLines + cursorY
		if actualCursorLine >= viewport.scrollOffset && actualCursorLine < viewport.scrollOffset+viewport.visibleLines {
			return actualCursorLine - viewport.scrollOffset
		}
		return -1
	}
}

// Place cursor in line
func (t *NativeTerminalWidget) placeCursorInLine(line *string, cursorX int) {
	if line == nil || cursorX < 0 {
		return
	}

	currentLine := *line
	runes := []rune(currentLine)

	if cursorX < len(runes) {
		// Replace character at cursor position with block cursor
		runes[cursorX] = '█'
		*line = string(runes)
	} else if cursorX < t.cols {
		// Extend line with spaces and place cursor
		padLen := cursorX - len(runes)
		if padLen > 0 {
			padding := strings.Repeat(" ", padLen)
			*line = currentLine + padding + "█"
		} else {
			*line = currentLine + "█"
		}
	}
}

// Apply colors (simplified)
func (t *NativeTerminalWidget) applyColors(lines []string, attrs [][]gopyte.Attributes) {
	if len(t.textGrid.Rows) == 0 || len(attrs) == 0 {
		t.textGrid.Refresh()
		return
	}

	for rowIdx, line := range lines {
		if rowIdx >= len(t.textGrid.Rows) || rowIdx >= len(attrs) {
			break
		}

		row := t.textGrid.Rows[rowIdx]
		runes := []rune(line)
		lineAttrs := attrs[rowIdx]

		for charIdx, char := range runes {
			if charIdx >= len(row.Cells) || charIdx >= len(lineAttrs) {
				break
			}

			attr := lineAttrs[charIdx]

			if row.Cells[charIdx].Style == nil {
				row.Cells[charIdx].Style = &widget.CustomTextGridStyle{}
			}

			style := row.Cells[charIdx].Style.(*widget.CustomTextGridStyle)

			// Map colors using theme-aware mappings
			if fgColor := t.mapColor(attr.Fg); fgColor != nil {
				style.FGColor = fgColor
			}
			if bgColor := t.mapColor(attr.Bg); bgColor != nil {
				style.BGColor = bgColor
			}

			row.Cells[charIdx].Rune = char
		}
	}

	t.textGrid.Refresh()
}

// Map gopyte color to Fyne color - now theme-aware
func (t *NativeTerminalWidget) mapColor(colorName string) color.Color {
	if colorName == "" || colorName == "default" {
		return nil
	}

	// Get theme-aware color mappings
	mappings := GetTerminalColorMappings()

	if fyneColor, exists := mappings[colorName]; exists {
		return fyneColor
	}

	return mappings["white"]
}

// New debug method to force expansion of viewport
func (t *NativeTerminalWidget) debugExpandViewport() {
	t.mutex.Lock()
	defer t.mutex.Unlock()

	log.Printf("DEBUG: Attempting to expand viewport...")

	// Get actual available space
	if t.textGrid != nil {
		containerSize := t.textGrid.Size()
		maxPossibleRows := int(containerSize.Height / t.charHeight)

		log.Printf("DEBUG: Container height %.1f allows %d rows vs current t.rows=%d",
			containerSize.Height, maxPossibleRows, t.rows)

		// Temporarily expand t.rows to see if this fixes the issue
		if maxPossibleRows > t.rows {
			oldRows := t.rows
			t.rows = maxPossibleRows
			log.Printf("DEBUG: Temporarily expanded t.rows from %d to %d", oldRows, maxPossibleRows)

			// Force a redraw with expanded viewport
			t.updatePending = true

			// Restore after a brief moment (in production you'd make this configurable)
			// t.rows = oldRows  // Comment this out to keep the expansion
		}
	}
}

// Call this method during scroll events to monitor what's happening
func (t *NativeTerminalWidget) debugScrollEvent(direction string, lines int) {
	log.Printf("SCROLL DEBUG [%s by %d]:", direction, lines)
	t.logBufferState("DURING_SCROLL")

	historyPos := t.screen.GetHistoryPos()
	historySize := t.screen.GetHistorySize()
	log.Printf("  - History position after scroll: %d/%d", historyPos, historySize)

	// Check if we're hitting limits
	if direction == "UP" && historyPos >= historySize {
		log.Printf("  - WARNING: HIT TOP LIMIT: Cannot scroll up further")
	}
	if direction == "DOWN" && historyPos <= 0 {
		log.Printf("  - WARNING: HIT BOTTOM LIMIT: Cannot scroll down further")
	}
}