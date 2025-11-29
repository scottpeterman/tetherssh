package gopyte

import (
	"fmt"
	"log"
	"strings"

	runewidth "github.com/mattn/go-runewidth"
)

// WideCharScreen adds wide character (CJK, emoji) support to HistoryScreen
type WideCharScreen struct {
	*HistoryScreen

	// Track cell widths (0 = continuation, 1 = normal, 2 = wide start)
	cellWidths     [][]int
	altCellWidths  [][]int
	mainCellWidths [][]int

	// Alternate screen state
	usingAlternate bool
	altBuffer      [][]rune
	altAttrs       [][]Attributes
	altCursor      Cursor
	mainBuffer     [][]rune
	mainAttrs      [][]Attributes
	mainCursor     Cursor
	HistoryPos     int

	// Virtual scrolling optimization
	virtualScrolling   bool
	viewportStart      int            // First visible line index (in total content)
	viewportEnd        int            // Last visible line index (in total content)
	lastRequestedLines int            // Track what was last requested
	displayCache       []string       // Cache for rendered display
	attributeCache     [][]Attributes // Cache for attributes
	cacheValid         bool           // Is the cache still valid?
	totalContentLines  int            // Total lines available (History + current)
}

// NewWideCharScreen creates a screen with wide character support and History
func NewWideCharScreen(columns, lines, maxHistory int) *WideCharScreen {
	HistoryScreen := NewHistoryScreen(columns, lines, maxHistory)

	w := &WideCharScreen{
		HistoryScreen:  HistoryScreen,
		usingAlternate: false,

		// Initialize virtual scrolling
		virtualScrolling:   true, // Enable by default for better performance
		viewportStart:      0,
		viewportEnd:        0,
		lastRequestedLines: 0,
		displayCache:       nil,
		attributeCache:     nil,
		cacheValid:         false,
		totalContentLines:  0,
	}

	// Initialize cell width tracking for main buffer
	w.cellWidths = make([][]int, lines)
	for i := 0; i < lines; i++ {
		w.cellWidths[i] = make([]int, columns)
		for j := 0; j < columns; j++ {
			w.cellWidths[i][j] = 1 // Default to normal width
		}
	}

	// CRITICAL: Link the cellWidths to HistoryScreen for proper History capture
	w.HistoryScreen.cellWidths = w.cellWidths

	// Initialize for alternate buffer
	w.altCellWidths = make([][]int, lines)
	for i := 0; i < lines; i++ {
		w.altCellWidths[i] = make([]int, columns)
		for j := 0; j < columns; j++ {
			w.altCellWidths[i][j] = 1
		}
	}

	// Initialize alternate screen buffers
	w.altBuffer = make([][]rune, lines)
	w.altAttrs = make([][]Attributes, lines)
	for i := 0; i < lines; i++ {
		w.altBuffer[i] = make([]rune, columns)
		w.altAttrs[i] = make([]Attributes, columns)
		for j := 0; j < columns; j++ {
			w.altBuffer[i][j] = ' '
			w.altAttrs[i][j] = Attributes{Fg: "default", Bg: "default"}
		}
	}

	// Store reference for later use
	w.mainCellWidths = w.cellWidths

	return w
}

// IsUsingAlternate returns true if in alternate screen mode
func (w *WideCharScreen) IsUsingAlternate() bool {
	return w.usingAlternate
}

// Override Draw to handle wide characters and emojis
func (w *WideCharScreen) Draw(text string) {
	// Invalidate cache when new content arrives
	w.invalidateCache()

	// Exit History mode if in main screen and viewing History
	if !w.usingAlternate && w.IsViewingHistory() {
		w.ScrollToBottom()
	}

	// Process each character with width awareness
	for _, ch := range text {
		w.drawChar(ch)
	}
}

// drawChar handles a single character with width calculation
func (w *WideCharScreen) drawChar(ch rune) {
	// Get the display width of the character
	charWidth := runewidth.RuneWidth(ch)

	// Handle zero-width characters (combining marks, etc.)
	if charWidth == 0 {
		w.handleZeroWidth(ch)
		return
	}

	// Check if the character fits at current position
	if w.cursor.X+charWidth > w.columns {
		if w.autoWrap {
			// Wide character doesn't fit, wrap to next line
			w.cursor.X = 0
			w.cursor.Y++
			if w.cursor.Y >= w.lines {
				if w.usingAlternate {
					w.scrollUpNoHistory()
				} else {
					// Use HistoryScreen's scrolling which captures width info
					w.addToHistory(0)
					w.scrollUpInternal()
				}
				w.cursor.Y = w.lines - 1
			}
		} else {
			// Can't place character at edge without wrapping
			return
		}
	}

	// CRITICAL FIX: Bounds check against actual buffer sizes, not just w.lines/w.columns
	// After resize, buffers might not match the new dimensions yet
	if w.cursor.Y < 0 || w.cursor.X < 0 {
		return
	}
	if w.cursor.Y >= len(w.buffer) || w.cursor.Y >= len(w.cellWidths) || w.cursor.Y >= len(w.attrs) {
		return
	}
	if w.cursor.X >= len(w.buffer[w.cursor.Y]) || w.cursor.X >= len(w.cellWidths[w.cursor.Y]) || w.cursor.X >= len(w.attrs[w.cursor.Y]) {
		return
	}

	// Clear any wide character we're overwriting
	w.clearCellAt(w.cursor.Y, w.cursor.X)

	w.buffer[w.cursor.Y][w.cursor.X] = ch
	w.attrs[w.cursor.Y][w.cursor.X] = w.cursor.Attrs
	w.cellWidths[w.cursor.Y][w.cursor.X] = charWidth

	if charWidth == 2 {
		// Mark the next cell as continuation - with bounds check
		nextX := w.cursor.X + 1
		if nextX < len(w.buffer[w.cursor.Y]) && nextX < len(w.cellWidths[w.cursor.Y]) && nextX < len(w.attrs[w.cursor.Y]) {
			w.buffer[w.cursor.Y][nextX] = 0 // Null char for continuation
			w.attrs[w.cursor.Y][nextX] = w.cursor.Attrs
			w.cellWidths[w.cursor.Y][nextX] = 0 // Continuation marker
		}
	}

	w.cursor.X += charWidth
}

// BACKSPACE FIX: Override Backspace to handle wide characters properly
func (w *WideCharScreen) Backspace() {
	if w.cursor.X > 0 {
		// Move cursor back
		w.cursor.X--

		// Clear the character at the new cursor position
		if w.cursor.Y < w.lines && w.cursor.X < w.columns {
			// Handle wide characters properly
			if w.cellWidths[w.cursor.Y][w.cursor.X] == 0 {
				// We're on a continuation cell, move back to the start
				if w.cursor.X > 0 {
					w.cursor.X--
				}
			}

			// Clear the cell(s)
			w.clearCellAt(w.cursor.Y, w.cursor.X)
		}
	}
}

// Enhanced GetDisplay with virtual scrolling

func (w *WideCharScreen) GetDisplay() []string {
	fmt.Printf("WideCharScreen.GetDisplay: START\n")

	// In alternate screen mode, no virtual scrolling needed
	if w.usingAlternate {
		fmt.Printf("WideCharScreen.GetDisplay: using alternate screen, rendering current content\n")
		w.invalidateCache()
		result := w.renderCurrentScreenContent()
		fmt.Printf("WideCharScreen.GetDisplay: alternate screen returned %d lines\n", len(result))
		return result
	}

	// Calculate total available content with detailed buffer information
	HistorySize := w.GetHistorySize()
	actualBufferLines := 0
	if w.HistoryScreen != nil && w.HistoryScreen.History != nil {
		actualBufferLines = w.HistoryScreen.History.Len()
	}
	currentScreenLines := w.lines
	totalLines := HistorySize + currentScreenLines
	w.totalContentLines = totalLines

	fmt.Printf("WideCharScreen.GetDisplay: BUFFER ANALYSIS:\n")
	fmt.Printf("  - actualBufferLines (linked list): %d\n", actualBufferLines)
	fmt.Printf("  - HistorySize (reported): %d\n", HistorySize)
	fmt.Printf("  - currentScreenLines: %d\n", currentScreenLines)
	fmt.Printf("  - virtualTotalLines: %d\n", totalLines)
	fmt.Printf("  - viewportLines (terminal rows): %d\n", w.lines)

	// Add debug info about History state
	if w.HistoryScreen != nil {
		fmt.Printf("WideCharScreen.GetDisplay: ViewingHistory=%v, HistoryPos=%d/%d\n",
			w.HistoryScreen.ViewingHistory, w.HistoryScreen.HistoryPos, HistorySize)
	}

	// If we have virtual scrolling enabled and cache is valid, use it
	if w.virtualScrolling && w.cacheValid && w.displayCache != nil {
		fmt.Printf("WideCharScreen.GetDisplay: using cached result (%d lines)\n", len(w.displayCache))
		return w.displayCache
	}

	var linesToRender []string

	if w.IsViewingHistory() {
		fmt.Printf("WideCharScreen.GetDisplay: viewing History, calling getProgressiveHistoryContent\n")
		linesToRender = w.getProgressiveHistoryContent()
	} else {
		fmt.Printf("WideCharScreen.GetDisplay: normal mode, calling getRecentContext\n")
		linesToRender = w.getRecentContext()
	}

	fmt.Printf("getProgressiveHistoryContent: got %d lines to render\n", len(linesToRender))

	// ENHANCED DEBUG: Show buffer analysis and content verification
	if len(linesToRender) > 0 {
		fmt.Printf("WideCharScreen.GetDisplay: CONTENT VERIFICATION:\n")
		fmt.Printf("  - returned %d lines for viewport\n", len(linesToRender))
		fmt.Printf("  - viewport covers virtual lines [%d-%d] of %d total\n",
			w.viewportStart, w.viewportEnd-1, totalLines)

		// Show first line content to verify we're getting the right data
		firstLine := linesToRender[0]
		if len(firstLine) > 60 {
			firstLine = firstLine[:60] + "..."
		}
		fmt.Printf("  - first line: %q\n", firstLine)

		if len(linesToRender) > 1 {
			lastIdx := len(linesToRender) - 1
			lastLine := linesToRender[lastIdx]
			if len(lastLine) > 60 {
				lastLine = lastLine[:60] + "..."
			}
			fmt.Printf("  - last line: %q\n", lastLine)
		}

		// Show what range of actual History we're accessing
		if w.viewportStart < HistorySize {
			HistoryLines := min(w.viewportEnd, HistorySize) - w.viewportStart
			fmt.Printf("  - showing %d History lines + %d current screen lines\n",
				HistoryLines, len(linesToRender)-HistoryLines)
		} else {
			fmt.Printf("  - showing %d current screen lines only\n", len(linesToRender))
		}
	}

	// Cache the result
	w.displayCache = linesToRender
	w.attributeCache = w.getAttributesForLines(linesToRender)
	w.cacheValid = true

	fmt.Printf("WideCharScreen.GetDisplay: END, returning %d lines\n", len(linesToRender))
	return linesToRender
}

// Enhanced GetAttributes with caching
func (w *WideCharScreen) GetAttributes() [][]Attributes {
	// In alternate screen mode, no virtual scrolling needed
	if w.usingAlternate {
		return w.extractCurrentScreenAttributes()
	}

	// If we have cached attributes and they're valid, use them
	if w.virtualScrolling && w.cacheValid && w.attributeCache != nil {
		return w.attributeCache
	}

	// This will trigger GetDisplay() which will populate attributeCache
	_ = w.GetDisplay()

	return w.attributeCache
}

func (w *WideCharScreen) getProgressiveHistoryContent() []string {
	fmt.Printf("getProgressiveHistoryContent: START\n")

	HistorySize := w.GetHistorySize()
	actualBufferLines := 0
	if w.HistoryScreen != nil && w.HistoryScreen.History != nil {
		actualBufferLines = w.HistoryScreen.History.Len()
	}
	virtualTotalLines := HistorySize + w.lines
	viewableScreenLines := w.lines

	fmt.Printf("getProgressiveHistoryContent: BUFFER DEBUG - actualBufferLines=%d, HistorySize=%d, screenLines=%d, virtualTotal=%d\n",
		actualBufferLines, HistorySize, viewableScreenLines, virtualTotalLines)

	if HistorySize == 0 {
		fmt.Printf("getProgressiveHistoryContent: no History, rendering current screen\n")
		return w.renderCurrentScreenContent()
	}

	currentHistoryPos := w.HistoryScreen.HistoryPos
	fmt.Printf("getProgressiveHistoryContent: HistorySize=%d, currentHistoryPos=%d\n", HistorySize, currentHistoryPos)

	// SIMPLE APPROACH: Map History position directly to content position
	// When currentHistoryPos = 0: show recent content (bottom of History + current screen)
	// When currentHistoryPos = HistorySize: show oldest content (top of History + some context)

	totalAvailableLines := HistorySize + w.lines

	// Calculate how much content to show
	displayLines := w.lines * 3 // Show 3 screens worth for context
	if displayLines > totalAvailableLines {
		displayLines = totalAvailableLines
	}

	// Map History position to start position
	// currentHistoryPos=0 -> show end of content (recent)
	// currentHistoryPos=max -> show beginning of content (oldest)

	var startPos int
	if currentHistoryPos == 0 {
		// At bottom - show recent content
		startPos = totalAvailableLines - displayLines
		if startPos < 0 {
			startPos = 0
		}
	} else {
		// In History - map position directly
		// Higher currentHistoryPos = show older content (lower startPos)
		maxPos := HistorySize
		scrollRatio := float64(currentHistoryPos) / float64(maxPos)

		// When scrollRatio = 1.0 (at top of History), startPos = 0
		// When scrollRatio = 0.0 (at bottom), startPos = totalAvailableLines - displayLines
		maxStartPos := totalAvailableLines - displayLines
		if maxStartPos < 0 {
			maxStartPos = 0
		}

		startPos = maxStartPos - int(scrollRatio*float64(maxStartPos))

		// Ensure we don't go negative
		if startPos < 0 {
			startPos = 0
		}
	}

	endPos := startPos + displayLines
	if endPos > totalAvailableLines {
		endPos = totalAvailableLines
		startPos = endPos - displayLines
		if startPos < 0 {
			startPos = 0
		}
	}

	w.viewportStart = startPos
	w.viewportEnd = endPos

	fmt.Printf("getProgressiveHistoryContent: SIMPLIFIED - HistoryPos=%d/%d maps to lines [%d-%d] of %d total\n",
		currentHistoryPos, HistorySize, startPos, endPos-1, totalAvailableLines)

	result := w.renderLinesInRange(startPos, endPos)
	fmt.Printf("getProgressiveHistoryContent: END, returning %d lines\n", len(result))
	return result
}

// Get recent context for normal typing mode
func (w *WideCharScreen) getRecentContext() []string {
	HistorySize := w.GetHistorySize()

	// In normal mode, show a reasonable amount of recent History
	maxRecentHistory := 200 // Show up to 200 lines of recent History for immediate scroll-back

	if HistorySize <= maxRecentHistory {
		// Show all History + current screen
		w.viewportStart = 0
		w.viewportEnd = HistorySize + w.lines
		return w.renderLinesInRange(0, HistorySize+w.lines)
	} else {
		// Show only recent History + current screen
		recentStart := HistorySize - maxRecentHistory
		w.viewportStart = recentStart
		w.viewportEnd = HistorySize + w.lines
		return w.renderLinesInRange(recentStart, HistorySize+w.lines)
	}
}

func (w *WideCharScreen) renderLinesInRange(start, end int) []string {
	if start < 0 {
		start = 0
	}

	HistorySize := w.GetHistorySize()
	totalLines := HistorySize + w.lines

	if end > totalLines {
		end = totalLines
	}

	if start >= end {
		fmt.Printf("renderLinesInRange: Invalid range [%d-%d], returning empty\n", start, end)
		return []string{}
	}

	fmt.Printf("renderLinesInRange: Rendering range [%d-%d] of %d total lines\n", start, end-1, totalLines)

	result := make([]string, 0, end-start)

	// Render History lines in range
	if start < HistorySize {
		HistoryStart := start
		HistoryEnd := end
		if HistoryEnd > HistorySize {
			HistoryEnd = HistorySize
		}

		fmt.Printf("renderLinesInRange: Getting History lines [%d-%d] from %d total History\n",
			HistoryStart, HistoryEnd-1, HistorySize)

		HistoryLines := w.getHistoryLinesInRange(HistoryStart, HistoryEnd)
		result = append(result, HistoryLines...)

		fmt.Printf("renderLinesInRange: Added %d History lines\n", len(HistoryLines))
	}

	// Render current screen lines in range
	if end > HistorySize {
		currentStart := 0
		if start > HistorySize {
			currentStart = start - HistorySize
		}
		currentEnd := end - HistorySize
		if currentEnd > w.lines {
			currentEnd = w.lines
		}

		if currentStart < currentEnd {
			fmt.Printf("renderLinesInRange: Getting current screen lines [%d-%d] from %d total screen\n",
				currentStart, currentEnd-1, w.lines)

			currentLines := w.renderCurrentScreenContent()
			if currentStart < len(currentLines) {
				endIdx := currentEnd
				if endIdx > len(currentLines) {
					endIdx = len(currentLines)
				}
				screenSlice := currentLines[currentStart:endIdx]
				result = append(result, screenSlice...)

				fmt.Printf("renderLinesInRange: Added %d current screen lines\n", len(screenSlice))
			}
		}
	}

	fmt.Printf("renderLinesInRange: Final result has %d lines\n", len(result))

	// Debug: Show first and last line to verify content
	if len(result) > 0 {
		first := result[0]
		if len(first) > 50 {
			first = first[:50] + "..."
		}
		fmt.Printf("renderLinesInRange: First line: %q\n", first)

		if len(result) > 1 {
			last := result[len(result)-1]
			if len(last) > 50 {
				last = last[:50] + "..."
			}
			fmt.Printf("renderLinesInRange: Last line: %q\n", last)
		}
	}

	return result
}

// ENHANCED getHistoryLinesInRange with better debugging
func (w *WideCharScreen) getHistoryLinesInRange(start, end int) []string {
	if w.HistoryScreen == nil || w.HistoryScreen.History == nil {
		fmt.Printf("getHistoryLinesInRange: No History available\n")
		return []string{}
	}

	if start >= end {
		fmt.Printf("getHistoryLinesInRange: Invalid range [%d-%d]\n", start, end)
		return []string{}
	}

	HistorySize := w.HistoryScreen.History.Len()
	if start >= HistorySize {
		fmt.Printf("getHistoryLinesInRange: Start %d >= History size %d\n", start, HistorySize)
		return []string{}
	}

	fmt.Printf("getHistoryLinesInRange: Extracting lines [%d-%d] from %d total History lines\n",
		start, end-1, HistorySize)

	result := make([]string, 0, end-start)
	index := 0

	// Iterate through History list but only process lines in our range
	for elem := w.HistoryScreen.History.Front(); elem != nil && index < end; elem = elem.Next() {
		if index >= start {
			if histLine, ok := elem.Value.(HistoryLine); ok {
				line := w.renderLineWithWidths(histLine.Chars, histLine.CellWidths)
				line = strings.TrimRight(line, " ")
				result = append(result, line)
			} else {
				fmt.Printf("getHistoryLinesInRange: Invalid History line at index %d\n", index)
				result = append(result, "")
			}
		}
		index++
	}

	fmt.Printf("getHistoryLinesInRange: Extracted %d lines from range [%d-%d]\n",
		len(result), start, end-1)

	return result
}

// Get attributes for the specific rendered lines
func (w *WideCharScreen) getAttributesForLines(lines []string) [][]Attributes {
	result := make([][]Attributes, len(lines))

	HistorySize := w.GetHistorySize()

	// For each line, determine if it's from History or current screen
	for i := 0; i < len(lines); i++ {
		if w.viewportStart+i < HistorySize {
			// History line
			result[i] = w.getHistoryAttributesForLine(w.viewportStart + i)
		} else {
			// Current screen line
			screenLine := (w.viewportStart + i) - HistorySize
			if screenLine < w.lines {
				result[i] = w.extractCurrentLineAttributes(screenLine)
			} else {
				result[i] = []Attributes{}
			}
		}
	}

	return result
}

// Get attributes for a specific History line
func (w *WideCharScreen) getHistoryAttributesForLine(HistoryIndex int) []Attributes {
	if w.HistoryScreen == nil || w.HistoryScreen.History == nil {
		return []Attributes{}
	}

	index := 0
	for elem := w.HistoryScreen.History.Front(); elem != nil; elem = elem.Next() {
		if index == HistoryIndex {
			if histLine, ok := elem.Value.(HistoryLine); ok {
				return w.extractAttributesWithWidths(histLine.Attrs, histLine.CellWidths)
			}
		}
		index++
	}

	return []Attributes{}
}

// scrollUpNoHistory scrolls without saving to History (for alternate screen)
func (w *WideCharScreen) scrollUpNoHistory() {
	// Move all lines up by one
	copy(w.buffer[0:], w.buffer[1:])
	copy(w.attrs[0:], w.attrs[1:])
	copy(w.cellWidths[0:], w.cellWidths[1:])

	// Clear the last line
	lastLine := w.lines - 1
	w.buffer[lastLine] = make([]rune, w.columns)
	w.attrs[lastLine] = make([]Attributes, w.columns)
	w.cellWidths[lastLine] = make([]int, w.columns)
	for i := 0; i < w.columns; i++ {
		w.buffer[lastLine][i] = ' '
		w.attrs[lastLine][i] = Attributes{Fg: "default", Bg: "default"}
		w.cellWidths[lastLine][i] = 1
	}
}

// handleZeroWidth handles zero-width combining characters
func (w *WideCharScreen) handleZeroWidth(ch rune) {
	// Combining characters attach to the previous character
	if w.cursor.X > 0 {
		// Combine with previous character
		prevX := w.cursor.X - 1
		if w.cellWidths[w.cursor.Y][prevX] == 2 && prevX > 0 {
			// Previous is a wide character, combine with its start
			prevX--
		}

		// Append the combining character
		existing := w.buffer[w.cursor.Y][prevX]
		if existing != 0 && existing != ' ' {
			// In a real implementation, we'd normalize the combination
			// For now, we'll just store the base character
		}
	} else if w.cursor.Y > 0 {
		// Combine with last character of previous line
		prevY := w.cursor.Y - 1
		prevX := w.columns - 1

		// Find the last actual character
		for prevX >= 0 && w.cellWidths[prevY][prevX] == 0 {
			prevX--
		}

		if prevX >= 0 && w.buffer[prevY][prevX] != ' ' {
			// Would combine here in full implementation
		}
	}
}

// clearCellAt clears a cell, handling wide characters properly
func (w *WideCharScreen) clearCellAt(y, x int) {
	// Bounds check against actual array sizes, not just w.lines/w.columns
	if y < 0 || x < 0 {
		return
	}
	if y >= len(w.buffer) || y >= len(w.cellWidths) || y >= len(w.attrs) {
		return
	}
	if x >= len(w.buffer[y]) || x >= len(w.cellWidths[y]) || x >= len(w.attrs[y]) {
		return
	}

	width := w.cellWidths[y][x]

	// If this is a continuation cell, clear the start cell too
	if width == 0 && x > 0 {
		w.clearCellAt(y, x-1)
		return
	}

	// Clear this cell
	w.buffer[y][x] = ' '
	w.attrs[y][x] = Attributes{Fg: "default", Bg: "default"}
	w.cellWidths[y][x] = 1

	// If this was a wide character, clear its continuation
	if width == 2 && x+1 < len(w.buffer[y]) && x+1 < len(w.cellWidths[y]) && x+1 < len(w.attrs[y]) {
		w.buffer[y][x+1] = ' '
		w.attrs[y][x+1] = Attributes{Fg: "default", Bg: "default"}
		w.cellWidths[y][x+1] = 1
	}
}

// renderCurrentScreenContent renders current screen content respecting wide characters
func (w *WideCharScreen) renderCurrentScreenContent() []string {
	currentLines := make([]string, w.lines)
	for y := 0; y < w.lines; y++ {
		runes := make([]rune, 0, w.columns)
		for x := 0; x < w.columns; x++ {
			// Skip continuation cells
			if len(w.cellWidths) > y && len(w.cellWidths[y]) > x && w.cellWidths[y][x] == 0 {
				continue
			}
			if len(w.buffer) > y && len(w.buffer[y]) > x {
				ch := w.buffer[y][x]
				if ch != 0 { // Don't include null characters
					runes = append(runes, ch)
				}
			}
		}
		currentLines[y] = string(runes)
	}
	return currentLines
}

// extractCurrentScreenAttributes extracts attributes respecting wide characters
func (w *WideCharScreen) extractCurrentScreenAttributes() [][]Attributes {
	currentAttrs := make([][]Attributes, w.lines)
	for y := 0; y < w.lines; y++ {
		currentAttrs[y] = w.extractCurrentLineAttributes(y)
	}
	return currentAttrs
}

// extractCurrentLineAttributes extracts attributes for current line respecting character widths
func (w *WideCharScreen) extractCurrentLineAttributes(y int) []Attributes {
	if y >= len(w.attrs) {
		return []Attributes{}
	}

	result := make([]Attributes, 0, w.columns)
	for x := 0; x < w.columns; x++ {
		// Skip continuation cells (width 0)
		if len(w.cellWidths) > y && len(w.cellWidths[y]) > x && w.cellWidths[y][x] == 0 {
			continue
		}

		if x < len(w.attrs[y]) {
			result = append(result, w.attrs[y][x])
		} else {
			result = append(result, Attributes{Fg: "default", Bg: "default"})
		}
	}

	return result
}

// Alternate screen handling
func (w *WideCharScreen) EnterAlternateScreen() {
	if w.usingAlternate {
		return
	}
	w.switchToAlternate()
}

func (w *WideCharScreen) ExitAlternateScreen() {
	if !w.usingAlternate {
		return
	}
	w.switchToMain()
}

func (w *WideCharScreen) switchToAlternate() {
	w.invalidateCache()

	// CRITICAL FIX: Ensure altBuffer and altAttrs match current screen dimensions
	// This handles the case where screen was resized before switching to alternate
	w.resizeAlternateBuffers(w.columns, w.lines)

	// Get actual buffer size (might differ from w.lines if HistoryScreen wasn't fully resized)
	actualBufferRows := len(w.buffer)
	if actualBufferRows > w.lines {
		actualBufferRows = w.lines
	}

	// Save main screen state - only copy what actually exists
	w.mainBuffer = make([][]rune, w.lines)
	w.mainAttrs = make([][]Attributes, w.lines)
	for i := 0; i < w.lines; i++ {
		w.mainBuffer[i] = make([]rune, w.columns)
		w.mainAttrs[i] = make([]Attributes, w.columns)

		// Only copy if source row exists
		if i < actualBufferRows && i < len(w.buffer) {
			srcLen := len(w.buffer[i])
			if srcLen > w.columns {
				srcLen = w.columns
			}
			copy(w.mainBuffer[i][:srcLen], w.buffer[i][:srcLen])

			if i < len(w.attrs) {
				attrLen := len(w.attrs[i])
				if attrLen > w.columns {
					attrLen = w.columns
				}
				copy(w.mainAttrs[i][:attrLen], w.attrs[i][:attrLen])
			}
		} else {
			// Initialize empty row
			for j := 0; j < w.columns; j++ {
				w.mainBuffer[i][j] = ' '
				w.mainAttrs[i][j] = Attributes{Fg: "default", Bg: "default"}
			}
		}
	}
	w.mainCursor = w.cursor
	w.mainCellWidths = w.cellWidths

	// Switch to alternate buffers
	w.buffer = w.altBuffer
	w.attrs = w.altAttrs
	w.cursor = w.altCursor
	w.cellWidths = w.altCellWidths
	w.usingAlternate = true

	// Update HistoryScreen's cellWidths reference
	w.HistoryScreen.cellWidths = w.cellWidths

	// Clear alternate screen
	w.clearScreen()
}

func (w *WideCharScreen) switchToMain() {
	w.invalidateCache()

	// Save alternate screen state
	w.altBuffer = w.buffer
	w.altAttrs = w.attrs
	w.altCursor = w.cursor
	w.altCellWidths = w.cellWidths

	// Restore main screen state
	if w.mainBuffer != nil && len(w.mainBuffer) > 0 {
		w.buffer = w.mainBuffer
		w.attrs = w.mainAttrs
		w.cursor = w.mainCursor
		w.cellWidths = w.mainCellWidths
	}
	w.usingAlternate = false

	// Restore HistoryScreen's cellWidths reference
	if w.cellWidths != nil {
		w.HistoryScreen.cellWidths = w.cellWidths
	}
}

func (w *WideCharScreen) clearScreen() {
	// Use actual buffer lengths to avoid index out of range
	bufferRows := len(w.buffer)
	if bufferRows > w.lines {
		bufferRows = w.lines
	}

	for i := 0; i < bufferRows; i++ {
		rowLen := len(w.buffer[i])
		if rowLen > w.columns {
			rowLen = w.columns
		}
		for j := 0; j < rowLen; j++ {
			w.buffer[i][j] = ' '
			if i < len(w.attrs) && j < len(w.attrs[i]) {
				w.attrs[i][j] = Attributes{Fg: "default", Bg: "default"}
			}
			if i < len(w.cellWidths) && j < len(w.cellWidths[i]) {
				w.cellWidths[i][j] = 1
			}
		}
	}
	w.cursor.X = 0
	w.cursor.Y = 0
}

// Terminal mode handling
func (w *WideCharScreen) SetMode(modes []int, private bool) {
	for _, mode := range modes {
		if private {
			switch mode {
			case 1049: // Alternate screen buffer
				log.Printf("WideCharScreen: Entering alternate screen mode")
				w.EnterAlternateScreen()
			default:
				if w.HistoryScreen != nil {
					w.HistoryScreen.SetMode(modes, private)
				}
			}
		} else {
			if w.HistoryScreen != nil {
				w.HistoryScreen.SetMode(modes, private)
			}
		}
	}
}

func (w *WideCharScreen) ResetMode(modes []int, private bool) {
	for _, mode := range modes {
		if private {
			switch mode {
			case 1049: // Exit alternate screen buffer
				log.Printf("WideCharScreen: Exiting alternate screen mode")
				w.ExitAlternateScreen()
			default:
				if w.HistoryScreen != nil {
					w.HistoryScreen.ResetMode(modes, private)
				}
			}
		} else {
			if w.HistoryScreen != nil {
				w.HistoryScreen.ResetMode(modes, private)
			}
		}
	}
}

// Utility methods
func (w *WideCharScreen) GetCursor() (int, int) {
	return w.cursor.X, w.cursor.Y
}

func (w *WideCharScreen) GetBuffer() [][]rune {
	return w.buffer
}

func (w *WideCharScreen) GetHistorySize() int {
	if w.HistoryScreen != nil {
		return w.HistoryScreen.GetHistorySize()
	}
	return 0
}

func (w *WideCharScreen) IsViewingHistory() bool {
	if w.HistoryScreen != nil {
		return w.HistoryScreen.IsViewingHistory()
	}
	return false
}

// In wide_char_screen.go, replace the ScrollUp and ScrollDown methods:

func (w *WideCharScreen) ScrollUp(lines int) {
	// CRITICAL FIX: Complete no-op in alternate screen mode
	if w.usingAlternate {
		return // No scrolling in alternate screen
	}

	w.invalidateCache()
	if w.HistoryScreen != nil {
		w.HistoryScreen.ScrollUp(lines)
	}
}

func (w *WideCharScreen) ScrollDown(lines int) {
	// CRITICAL FIX: Complete no-op in alternate screen mode
	if w.usingAlternate {
		return // No scrolling in alternate screen
	}

	w.invalidateCache()
	if w.HistoryScreen != nil {
		w.HistoryScreen.ScrollDown(lines)
	}
}

func (w *WideCharScreen) ScrollToBottom() {
	// CRITICAL FIX: Complete no-op in alternate screen mode
	if w.usingAlternate {
		return // No scrolling in alternate screen
	}

	w.invalidateCache()
	if w.HistoryScreen != nil {
		w.HistoryScreen.ScrollToBottom()
	}
}

// Cache management
func (w *WideCharScreen) invalidateCache() {
	w.cacheValid = false
	w.displayCache = nil
	w.attributeCache = nil
}
func (w *WideCharScreen) InvalidateCache() {
	w.cacheValid = false
	w.displayCache = nil
	w.attributeCache = nil
}
func (w *WideCharScreen) EnableVirtualScrolling() {
	w.virtualScrolling = true
	w.invalidateCache()
}

func (w *WideCharScreen) DisableVirtualScrolling() {
	w.virtualScrolling = false
	w.invalidateCache()
}

// History management
func (w *WideCharScreen) SetMaxHistoryLines(maxLines int) {
	if w.HistoryScreen != nil {
		w.HistoryScreen.maxHistory = maxLines

		// Trim if necessary
		if w.HistoryScreen.History != nil {
			for w.HistoryScreen.History.Len() > maxLines && w.HistoryScreen.History.Len() > 0 {
				w.HistoryScreen.History.Remove(w.HistoryScreen.History.Front())
			}
		}
	}
}

// Resize handling
func (w *WideCharScreen) Resize(newCols, newLines int) {
	if newCols <= 0 || newLines <= 0 {
		return
	}

	log.Printf("WideCharScreen.Resize: %dx%d -> %dx%d", w.columns, w.lines, newCols, newLines)

	w.invalidateCache()

	// If viewing History, return to live view first
	if !w.usingAlternate && w.IsViewingHistory() {
		w.ScrollToBottom()
	}

	// Let HistoryScreen resize first (this should resize buffer and attrs)
	w.HistoryScreen.Resize(newCols, newLines)

	// Update geometry
	w.columns = newCols
	w.lines = newLines

	// CRITICAL FIX: Verify and fix buffer sizes after HistoryScreen.Resize
	// HistoryScreen.Resize might not properly resize all arrays
	w.ensureBufferSizes(newCols, newLines)

	// Rebuild width grids
	w.cellWidths = w.rebuildWidthGrid(w.cellWidths, newCols, newLines)
	w.altCellWidths = w.rebuildWidthGrid(w.altCellWidths, newCols, newLines)

	// Update references
	if !w.usingAlternate {
		w.mainCellWidths = w.cellWidths
		w.HistoryScreen.cellWidths = w.cellWidths
	}

	// Resize alternate buffers
	w.resizeAlternateBuffers(newCols, newLines)

	log.Printf("WideCharScreen.Resize complete: buffer=%d rows, cellWidths=%d rows",
		len(w.buffer), len(w.cellWidths))
}

// ensureBufferSizes makes sure all buffers match the expected dimensions
func (w *WideCharScreen) ensureBufferSizes(cols, lines int) {
	// Check and fix w.buffer (inherited from HistoryScreen)
	if len(w.buffer) < lines {
		for len(w.buffer) < lines {
			newRow := make([]rune, cols)
			for j := 0; j < cols; j++ {
				newRow[j] = ' '
			}
			w.buffer = append(w.buffer, newRow)
		}
	} else if len(w.buffer) > lines {
		w.buffer = w.buffer[:lines]
	}

	// Ensure each row has correct column count
	for i := 0; i < len(w.buffer); i++ {
		if len(w.buffer[i]) < cols {
			oldLen := len(w.buffer[i])
			newRow := make([]rune, cols)
			copy(newRow, w.buffer[i])
			for j := oldLen; j < cols; j++ {
				newRow[j] = ' '
			}
			w.buffer[i] = newRow
		} else if len(w.buffer[i]) > cols {
			w.buffer[i] = w.buffer[i][:cols]
		}
	}

	// Check and fix w.attrs
	if len(w.attrs) < lines {
		for len(w.attrs) < lines {
			newRow := make([]Attributes, cols)
			for j := 0; j < cols; j++ {
				newRow[j] = Attributes{Fg: "default", Bg: "default"}
			}
			w.attrs = append(w.attrs, newRow)
		}
	} else if len(w.attrs) > lines {
		w.attrs = w.attrs[:lines]
	}

	// Ensure each attrs row has correct column count
	for i := 0; i < len(w.attrs); i++ {
		if len(w.attrs[i]) < cols {
			oldLen := len(w.attrs[i])
			newRow := make([]Attributes, cols)
			copy(newRow, w.attrs[i])
			for j := oldLen; j < cols; j++ {
				newRow[j] = Attributes{Fg: "default", Bg: "default"}
			}
			w.attrs[i] = newRow
		} else if len(w.attrs[i]) > cols {
			w.attrs[i] = w.attrs[i][:cols]
		}
	}

	// Sync back to HistoryScreen
	w.HistoryScreen.buffer = w.buffer
	w.HistoryScreen.attrs = w.attrs
}

func (w *WideCharScreen) rebuildWidthGrid(grid [][]int, newCols, newLines int) [][]int {
	if grid == nil {
		grid = make([][]int, 0)
	}

	// Adjust rows
	if len(grid) > newLines {
		grid = grid[:newLines]
	} else if len(grid) < newLines {
		for len(grid) < newLines {
			row := make([]int, newCols)
			for j := 0; j < newCols; j++ {
				row[j] = 1
			}
			grid = append(grid, row)
		}
	}

	// Adjust columns
	for y := 0; y < newLines; y++ {
		row := grid[y]
		if len(row) > newCols {
			row = row[:newCols]
		} else if len(row) < newCols {
			for len(row) < newCols {
				row = append(row, 1)
			}
		}
		grid[y] = row
	}
	return grid
}

func (w *WideCharScreen) resizeAlternateBuffers(newCols, newLines int) {
	// Resize alternate buffer
	if len(w.altBuffer) > newLines {
		w.altBuffer = w.altBuffer[:newLines]
		w.altAttrs = w.altAttrs[:newLines]
	} else if len(w.altBuffer) < newLines {
		for len(w.altBuffer) < newLines {
			newRow := make([]rune, newCols)
			newAttrRow := make([]Attributes, newCols)
			for j := 0; j < newCols; j++ {
				newRow[j] = ' '
				newAttrRow[j] = Attributes{Fg: "default", Bg: "default"}
			}
			w.altBuffer = append(w.altBuffer, newRow)
			w.altAttrs = append(w.altAttrs, newAttrRow)
		}
	}

	// Resize columns in alternate buffer
	for i := 0; i < newLines; i++ {
		if len(w.altBuffer[i]) > newCols {
			w.altBuffer[i] = w.altBuffer[i][:newCols]
			w.altAttrs[i] = w.altAttrs[i][:newCols]
		} else if len(w.altBuffer[i]) < newCols {
			oldLen := len(w.altBuffer[i])
			for j := oldLen; j < newCols; j++ {
				w.altBuffer[i] = append(w.altBuffer[i], ' ')
				w.altAttrs[i] = append(w.altAttrs[i], Attributes{Fg: "default", Bg: "default"})
			}
		}
	}
}

// Helper methods from HistoryScreen
func (w *WideCharScreen) renderLineWithWidths(chars []rune, widths []int) string {
	if len(chars) == 0 {
		return ""
	}

	runes := make([]rune, 0, len(chars))
	for i, ch := range chars {
		// Skip continuation cells (width 0)
		if i < len(widths) && widths[i] == 0 {
			continue
		}

		// Skip null characters used for wide char continuations
		if ch != 0 {
			runes = append(runes, ch)
		}
	}

	return string(runes)
}

func (w *WideCharScreen) extractAttributesWithWidths(attrs []Attributes, widths []int) []Attributes {
	if len(attrs) == 0 {
		return []Attributes{}
	}

	result := make([]Attributes, 0, len(attrs))
	for i, attr := range attrs {
		// Skip continuation cells (width 0)
		if i < len(widths) && widths[i] == 0 {
			continue
		}

		result = append(result, attr)
	}

	return result
}

// ADD these delegate methods to your wide_char_screen.go file
// Put them with the other delegate methods around line 400+

// GetHistoryPos delegates to HistoryScreen
func (w *WideCharScreen) GetHistoryPos() int {
	if w.HistoryScreen != nil {
		return w.HistoryScreen.GetHistoryPos()
	}
	return 0
}

// GetMaxHistoryPos delegates to HistoryScreen
func (w *WideCharScreen) GetMaxHistoryPos() int {
	if w.HistoryScreen != nil {
		return w.HistoryScreen.GetMaxHistoryPos()
	}
	return 0
}

// IsAtTopOfHistory delegates to HistoryScreen
func (w *WideCharScreen) IsAtTopOfHistory() bool {
	if w.HistoryScreen != nil {
		return w.HistoryScreen.IsAtTopOfHistory()
	}
	return false
}

// IsAtBottomOfHistory delegates to HistoryScreen
func (w *WideCharScreen) IsAtBottomOfHistory() bool {
	if w.HistoryScreen != nil {
		return w.HistoryScreen.IsAtBottomOfHistory()
	}
	return true
}

// ADD this method to the end of your wide_char_screen.go file:

// DebugViewportState provides detailed information about current viewport state
func (w *WideCharScreen) DebugViewportState() {
	if w.HistoryScreen == nil {
		fmt.Printf("DebugViewportState: HistoryScreen is nil\n")
		return
	}

	historySize := w.GetHistorySize()
	currentPos := w.HistoryScreen.HistoryPos
	totalContent := historySize + w.lines

	fmt.Printf("=== VIEWPORT DEBUG ===\n")
	fmt.Printf("History: %d lines, position: %d/%d\n", historySize, currentPos, historySize)
	fmt.Printf("Current screen: %d lines\n", w.lines)
	fmt.Printf("Total content: %d lines\n", totalContent)
	fmt.Printf("Viewport: [%d-%d] of %d\n", w.viewportStart, w.viewportEnd-1, totalContent)
	fmt.Printf("In history mode: %v\n", w.IsViewingHistory())

	// Show what content the viewport should be showing
	if currentPos > 0 {
		progress := float64(currentPos) / float64(historySize)
		fmt.Printf("Scroll progress: %.1f%% toward top\n", progress*100)

		if currentPos >= historySize {
			fmt.Printf("Should show: OLDEST history content\n")
		} else {
			fmt.Printf("Should show: Mixed content, %d lines into history\n", currentPos)
		}
	} else {
		fmt.Printf("Should show: RECENT content (bottom)\n")
	}
	fmt.Printf("=====================\n")
}