package gopyte

import (
	"container/list"
	"fmt"
	"strings"
)

// Enhanced HistoryLine with wide character support
type HistoryLine struct {
	Chars      []rune       // Character data
	Attrs      []Attributes // Color/style attributes
	CellWidths []int        // Width tracking (0=continuation, 1=normal, 2=wide)
}

// HistoryScreen extends NativeScreen with scrollback buffer support
type HistoryScreen struct {
	NativeScreen // Embedded, not pointer

	// History management
	History    *list.List // Doubly-linked list of enhanced HistoryLine
	maxHistory int        // Maximum lines to keep in History
	HistoryPos int        // Current position in History (0 = bottom/current)

	// Saved screen state for viewing History
	savedBuffer     [][]rune
	savedAttrs      [][]Attributes
	savedCellWidths [][]int // Enhanced: save width information
	savedCursor     Cursor
	ViewingHistory  bool

	// Cell width tracking (linked from WideCharScreen)
	cellWidths [][]int
}

// NewHistoryScreen creates a screen with scrollback buffer
func NewHistoryScreen(columns, lines, maxHistory int) *HistoryScreen {
	h := &HistoryScreen{
		NativeScreen:   *NewNativeScreen(columns, lines),
		History:        list.New(),
		maxHistory:     maxHistory,
		HistoryPos:     0,
		ViewingHistory: false,
	}

	// Initialize basic cell width tracking (will be overridden by WideCharScreen)
	h.cellWidths = make([][]int, lines)
	for i := 0; i < lines; i++ {
		h.cellWidths[i] = make([]int, columns)
		for j := 0; j < columns; j++ {
			h.cellWidths[i][j] = 1 // Default to normal width
		}
	}

	return h
}

// GetHistoryLines returns all History lines as strings with wide character support
func (h *HistoryScreen) GetHistoryLines() []string {
	if h.History == nil {
		return []string{}
	}

	var HistoryLines []string

	// Iterate through the History list
	for elem := h.History.Front(); elem != nil; elem = elem.Next() {
		if histLine, ok := elem.Value.(HistoryLine); ok {
			// Process line with wide character awareness
			line := h.renderLineWithWidths(histLine.Chars, histLine.CellWidths)
			// Trim trailing spaces but preserve the line (even if empty after trim)
			line = strings.TrimRight(line, " ")
			HistoryLines = append(HistoryLines, line)
		}
	}

	return HistoryLines
}

// GetHistoryAttributes returns History attributes with wide character support
func (h *HistoryScreen) GetHistoryAttributes() [][]Attributes {
	if h.History == nil {
		return [][]Attributes{}
	}

	var HistoryAttrs [][]Attributes

	for elem := h.History.Front(); elem != nil; elem = elem.Next() {
		if histLine, ok := elem.Value.(HistoryLine); ok {
			// Create attribute array that matches the rendered line
			lineAttrs := h.extractAttributesWithWidths(histLine.Attrs, histLine.CellWidths)
			HistoryAttrs = append(HistoryAttrs, lineAttrs)
		}
	}

	return HistoryAttrs
}
func (h *HistoryScreen) GetHistoryPos() int {
	return h.HistoryPos
}

// renderLineWithWidths renders a line respecting character widths
func (h *HistoryScreen) renderLineWithWidths(chars []rune, widths []int) string {
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

// extractAttributesWithWidths extracts attributes respecting character widths
func (h *HistoryScreen) extractAttributesWithWidths(attrs []Attributes, widths []int) []Attributes {
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

// Replace the Linefeed method in history_screen.go with this:

func (h *HistoryScreen) Linefeed() {
	bufferLen := len(h.buffer)
	if bufferLen == 0 {
		return
	}

	// Use actual buffer size as the limit, not h.lines
	effectiveLines := bufferLen
	if h.lines < effectiveLines {
		effectiveLines = h.lines
	}

	if h.scrollRegionSet {
		// Within scroll region - check if we need to scroll
		// Also bounds-check scrollBottom against buffer
		effectiveBottom := h.scrollBottom
		if effectiveBottom >= bufferLen {
			effectiveBottom = bufferLen - 1
		}

		if h.cursor.Y >= effectiveBottom {
			// At or past bottom of scroll region - scroll within region and save to history
			h.addToHistory(h.scrollTop) // Save the top line of scroll region
			h.scrollWithinMargins(h.scrollTop, effectiveBottom)
			// Cursor stays at scrollBottom after scrolling
			h.cursor.Y = effectiveBottom
		} else {
			// Not at bottom of region - move down normally
			h.cursor.Y++
		}
	} else {
		// No scroll region - check screen boundaries
		if h.cursor.Y >= effectiveLines-1 {
			// At bottom of screen - scroll and save to history
			h.addToHistory(0) // Save the top line
			h.scrollUpInternal()
			// Cursor stays at bottom after scrolling
			h.cursor.Y = effectiveLines - 1
		} else {
			// Not at bottom - move down normally
			h.cursor.Y++
		}
	}

	// In newline mode, also do CR
	if h.newlineMode {
		h.cursor.X = 0
	}
}

// Also add this scrollWithinMargins method to HistoryScreen if it doesn't exist:

func (h *HistoryScreen) scrollWithinMargins(top, bottom int) {
	fmt.Printf("HistoryScreen.scrollWithinMargins: scrolling lines %d to %d (buffer len=%d)\n", top, bottom, len(h.buffer))

	// CRITICAL: Bounds check - bottom must be within actual buffer size
	bufferLen := len(h.buffer)
	if bufferLen == 0 {
		return
	}
	if bottom >= bufferLen {
		bottom = bufferLen - 1
	}
	if top < 0 {
		top = 0
	}
	if top >= bottom {
		return
	}

	// Move lines up within the margin area
	for y := top; y < bottom; y++ {
		if y+1 < bufferLen && y < bufferLen {
			h.buffer[y] = h.buffer[y+1]
			if y < len(h.attrs) && y+1 < len(h.attrs) {
				h.attrs[y] = h.attrs[y+1]
			}
			// Also handle cell widths if available
			if h.cellWidths != nil && y+1 < len(h.cellWidths) && y < len(h.cellWidths) {
				if h.cellWidths[y+1] != nil && h.cellWidths[y] != nil {
					copy(h.cellWidths[y], h.cellWidths[y+1])
				}
			}
		}
	}

	// Clear the bottom line in margin
	if bottom >= 0 && bottom < bufferLen {
		h.buffer[bottom] = make([]rune, h.columns)
		if bottom < len(h.attrs) {
			h.attrs[bottom] = make([]Attributes, h.columns)
		}
		if h.cellWidths != nil && bottom < len(h.cellWidths) {
			h.cellWidths[bottom] = make([]int, h.columns)
		}

		for x := 0; x < h.columns; x++ {
			h.buffer[bottom][x] = ' '
			if bottom < len(h.attrs) {
				h.attrs[bottom][x] = DefaultAttributes()
			}
			if h.cellWidths != nil && bottom < len(h.cellWidths) && h.cellWidths[bottom] != nil {
				h.cellWidths[bottom][x] = 1
			}
		}
	}

	fmt.Printf("HistoryScreen.scrollWithinMargins: cleared line %d\n", bottom)
}
func (h *HistoryScreen) Index() {
	bufferLen := len(h.buffer)
	if bufferLen == 0 {
		return
	}

	// Use actual buffer size as the limit
	effectiveLines := bufferLen
	if h.lines < effectiveLines {
		effectiveLines = h.lines
	}

	// Check if at bottom BEFORE incrementing
	if h.cursor.Y >= effectiveLines-1 {
		// At bottom, scroll
		h.addToHistory(0)
		h.scrollUpInternal()
		// Stay at bottom
		h.cursor.Y = effectiveLines - 1
	} else {
		// Not at bottom, move down
		h.cursor.Y++
	}
}

// scrollUpInternal performs the actual scroll without calling parent
func (h *HistoryScreen) scrollUpInternal() {
	bufferLen := len(h.buffer)
	if bufferLen == 0 {
		return
	}

	// Move all lines up by one
	if bufferLen > 1 {
		copy(h.buffer[0:bufferLen-1], h.buffer[1:bufferLen])
		if len(h.attrs) > 1 {
			copy(h.attrs[0:len(h.attrs)-1], h.attrs[1:len(h.attrs)])
		}

		// Move cell widths if available
		if h.cellWidths != nil && len(h.cellWidths) > 1 {
			copy(h.cellWidths[0:len(h.cellWidths)-1], h.cellWidths[1:len(h.cellWidths)])
		}
	}

	// Clear the last line - use actual buffer length, not h.lines
	lastLine := bufferLen - 1
	if lastLine >= 0 {
		h.buffer[lastLine] = make([]rune, h.columns)
		if lastLine < len(h.attrs) {
			h.attrs[lastLine] = make([]Attributes, h.columns)
		}

		// Clear cell widths for last line
		if h.cellWidths != nil && lastLine < len(h.cellWidths) {
			h.cellWidths[lastLine] = make([]int, h.columns)
			for i := 0; i < h.columns; i++ {
				h.cellWidths[lastLine][i] = 1 // Default to normal width
			}
		}

		for i := 0; i < h.columns; i++ {
			h.buffer[lastLine][i] = ' '
			if lastLine < len(h.attrs) {
				h.attrs[lastLine][i] = Attributes{Fg: "default", Bg: "default"}
			}
		}
	}
}

// Enhanced addToHistory with wide character support
func (h *HistoryScreen) addToHistory(lineNum int) {
	if lineNum >= 0 && lineNum < h.lines {
		// Create a copy of the line with full data
		line := HistoryLine{
			Chars:      make([]rune, h.columns),
			Attrs:      make([]Attributes, h.columns),
			CellWidths: make([]int, h.columns),
		}

		copy(line.Chars, h.buffer[lineNum])
		copy(line.Attrs, h.attrs[lineNum])

		// Copy width information if available
		if h.cellWidths != nil && lineNum < len(h.cellWidths) && h.cellWidths[lineNum] != nil {
			copy(line.CellWidths, h.cellWidths[lineNum])
		} else {
			// Default to normal width
			for i := range line.CellWidths {
				line.CellWidths[i] = 1
			}
		}

		// Add to History
		h.History.PushBack(line)

		// Trim History if it exceeds max
		if h.History.Len() > h.maxHistory {
			h.History.Remove(h.History.Front())
		}
	}
}

func (h *HistoryScreen) ScrollUp(lines int) {
	// Save current screen if we're not already viewing History
	if !h.ViewingHistory {
		h.saveCurrentScreen()
		h.ViewingHistory = true
	}

	// Get the actual maximum position we can reach
	maxHistoryPos := h.History.Len()

	// CRITICAL FIX: Remove the premature boundary check
	// The old code had: availableScroll := maxHistoryPos - h.HistoryPos
	// This prevented reaching the absolute top when HistoryPos was already at max-1

	// Calculate new position directly
	newPos := h.HistoryPos + lines

	// FIXED: Allow reaching the absolute maximum (not maxHistoryPos-1)
	if newPos > maxHistoryPos {
		newPos = maxHistoryPos
	}

	// Only proceed if we actually moved
	actualMoved := newPos - h.HistoryPos
	if actualMoved <= 0 {
		fmt.Printf("HistoryScreen.ScrollUp: Already at top, pos=%d/%d\n", h.HistoryPos, maxHistoryPos)
		return
	}

	// Update position
	h.HistoryPos = newPos
	h.renderHistoryView()

	fmt.Printf("HistoryScreen.ScrollUp: moved up %d lines, pos=%d/%d\n", actualMoved, h.HistoryPos, maxHistoryPos)
}

// Enhanced ScrollDown with better boundary checking
func (h *HistoryScreen) ScrollDown(lines int) {
	if !h.ViewingHistory {
		return
	}

	// Calculate new position
	newPos := h.HistoryPos - lines

	if newPos <= 0 {
		// Return to live view
		h.HistoryPos = 0
		h.restoreCurrentScreen()
		h.ViewingHistory = false
		fmt.Printf("HistoryScreen.ScrollDown: returned to live view\n")
	} else {
		h.HistoryPos = newPos
		h.renderHistoryView()
		fmt.Printf("HistoryScreen.ScrollDown: moved down %d lines, pos=%d\n", lines, h.HistoryPos)
	}
}

// ScrollToBottom returns to the live terminal view
func (h *HistoryScreen) ScrollToBottom() {
	if h.ViewingHistory {
		h.HistoryPos = 0
		h.restoreCurrentScreen()
		h.ViewingHistory = false
	}
}

// Enhanced saveCurrentScreen with cell width support
func (h *HistoryScreen) saveCurrentScreen() {
	h.savedBuffer = make([][]rune, h.lines)
	h.savedAttrs = make([][]Attributes, h.lines)
	h.savedCellWidths = make([][]int, h.lines)

	for i := 0; i < h.lines; i++ {
		h.savedBuffer[i] = make([]rune, h.columns)
		h.savedAttrs[i] = make([]Attributes, h.columns)
		h.savedCellWidths[i] = make([]int, h.columns)

		copy(h.savedBuffer[i], h.buffer[i])
		copy(h.savedAttrs[i], h.attrs[i])

		// Copy cell widths if available
		if h.cellWidths != nil && i < len(h.cellWidths) && h.cellWidths[i] != nil {
			copy(h.savedCellWidths[i], h.cellWidths[i])
		} else {
			// Default to normal width
			for j := range h.savedCellWidths[i] {
				h.savedCellWidths[i][j] = 1
			}
		}
	}
	h.savedCursor = h.cursor
}

// Enhanced restoreCurrentScreen with cell width support
func (h *HistoryScreen) restoreCurrentScreen() {
	if h.savedBuffer != nil {
		h.buffer = h.savedBuffer
		h.attrs = h.savedAttrs
		h.cursor = h.savedCursor

		// Restore cell widths
		if h.savedCellWidths != nil {
			h.cellWidths = h.savedCellWidths
		}

		h.savedBuffer = nil
		h.savedAttrs = nil
		h.savedCellWidths = nil

		// Restore cursor visibility
		h.cursor.Hidden = false
	}
}

// Replace the Draw method in history_screen.go:

func (h *HistoryScreen) Draw(text string) {
	// Exit History mode if we're in it
	if h.ViewingHistory {
		h.ScrollToBottom()
	}

	// Now draw using embedded NativeScreen's implementation
	for _, ch := range text {
		// Check if we need to wrap
		if h.cursor.X >= h.columns {
			if h.autoWrap {
				h.cursor.X = 0

				// SCROLL REGION FIX: Check scroll region boundaries, not just screen boundaries
				if h.scrollRegionSet {
					// Within scroll region - check if at bottom of region
					if h.cursor.Y >= h.scrollBottom {
						// At bottom of scroll region - scroll within region
						h.addToHistory(h.scrollTop) // Save top line of scroll region
						h.scrollWithinMargins(h.scrollTop, h.scrollBottom)
						// Cursor stays at scrollBottom
						h.cursor.Y = h.scrollBottom
					} else {
						// Not at bottom of region - move down normally
						h.cursor.Y++
					}
				} else {
					// No scroll region - check full screen boundaries
					if h.cursor.Y >= h.lines-1 {
						h.addToHistory(0)
						h.scrollUpInternal()
						// Stay at bottom line
						h.cursor.Y = h.lines - 1
					} else {
						h.cursor.Y++
					}
				}
			} else {
				h.cursor.X = h.columns - 1
			}
		}

		// Place character
		if h.cursor.Y < h.lines && h.cursor.X < h.columns {
			h.buffer[h.cursor.Y][h.cursor.X] = ch
			h.attrs[h.cursor.Y][h.cursor.X] = h.cursor.Attrs

			// Set cell width (default to normal width in basic HistoryScreen)
			if h.cellWidths != nil && h.cursor.Y < len(h.cellWidths) && h.cellWidths[h.cursor.Y] != nil {
				h.cellWidths[h.cursor.Y][h.cursor.X] = 1 // This will be overridden by WideCharScreen
			}

			h.cursor.X++
		}
	}
}

// Override EraseInDisplay to handle History clearing
func (h *HistoryScreen) EraseInDisplay(how int) {
	if h.ViewingHistory {
		h.ScrollToBottom()
	}

	// Call embedded implementation
	h.NativeScreen.EraseInDisplay(how)

	// Clear History on full clear (ESC[2J or ESC[3J)
	if how == 2 || how == 3 {
		h.History.Init() // Clear the list
		h.HistoryPos = 0
	}
}

// Override Reset to clear History
func (h *HistoryScreen) Reset() {
	h.NativeScreen.Reset()
	h.History.Init() // Clear History
	h.HistoryPos = 0
	h.ViewingHistory = false
	h.savedBuffer = nil
	h.savedAttrs = nil
	h.savedCellWidths = nil
}

// GetHistorySize returns the current number of lines in History
func (h *HistoryScreen) GetHistorySize() int {
	if h.History == nil {
		return 0
	}
	return h.History.Len()
}

// IsViewingHistory returns true if currently scrolled back in History
func (h *HistoryScreen) IsViewingHistory() bool {
	return h.ViewingHistory
}

// GetDisplay returns the current display as strings (from embedded NativeScreen)
func (h *HistoryScreen) GetDisplay() []string {
	return h.NativeScreen.GetDisplay()
}

// GetCursor returns the current cursor position
func (h *HistoryScreen) GetCursor() (int, int) {
	return h.cursor.X, h.cursor.Y
}

// GetCursorObject returns the cursor object for testing
func (h *HistoryScreen) GetCursorObject() *Cursor {
	return &h.cursor
}

// Enhanced Resize with proper cell width handling
func (h *HistoryScreen) Resize(newCols, newLines int) {
	if newCols <= 0 || newLines <= 0 {
		return
	}

	// If we are viewing History, jump back to live view first.
	if h.ViewingHistory {
		h.ScrollToBottom()
	}

	oldLines := h.lines
	oldCols := h.columns

	// If rows will shrink and we're not in alternate (alt handled elsewhere),
	// push the bottom lines that would be lost into History so they remain reachable.
	if newLines < oldLines {
		cut := oldLines - newLines
		start := oldLines - cut
		for i := start; i < oldLines; i++ {
			h.addToHistory(i)
		}
	}

	// Resize underlying NativeScreen buffers/attrs first with column logic.
	h.NativeScreen.Resize(newCols, newLines)

	// Resize cell width tracking
	h.resizeCellWidths(newCols, newLines, oldCols, oldLines)

	// Update geometry
	h.columns = newCols
	h.lines = newLines
}

// resizeCellWidths handles resizing of cell width arrays
func (h *HistoryScreen) resizeCellWidths(newCols, newLines, oldCols, oldLines int) {
	if h.cellWidths == nil {
		// Initialize if not present
		h.cellWidths = make([][]int, newLines)
		for i := 0; i < newLines; i++ {
			h.cellWidths[i] = make([]int, newCols)
			for j := 0; j < newCols; j++ {
				h.cellWidths[i][j] = 1
			}
		}
		return
	}

	// Resize rows
	if len(h.cellWidths) > newLines {
		h.cellWidths = h.cellWidths[:newLines]
	} else if len(h.cellWidths) < newLines {
		// Add new rows
		for len(h.cellWidths) < newLines {
			newRow := make([]int, newCols)
			for j := 0; j < newCols; j++ {
				newRow[j] = 1 // Default to normal width
			}
			h.cellWidths = append(h.cellWidths, newRow)
		}
	}

	// Resize columns in each row
	for i := 0; i < newLines; i++ {
		if h.cellWidths[i] == nil {
			h.cellWidths[i] = make([]int, newCols)
			for j := 0; j < newCols; j++ {
				h.cellWidths[i][j] = 1
			}
			continue
		}

		if len(h.cellWidths[i]) > newCols {
			h.cellWidths[i] = h.cellWidths[i][:newCols]
		} else if len(h.cellWidths[i]) < newCols {
			// Extend the row
			oldLen := len(h.cellWidths[i])
			for j := oldLen; j < newCols; j++ {
				h.cellWidths[i] = append(h.cellWidths[i], 1)
			}
		}
	}

	// Update any saved cell widths for consistency
	if h.savedCellWidths != nil {
		// Resize saved cell widths to match new dimensions
		// This ensures consistency when restoring from History view
		if len(h.savedCellWidths) > newLines {
			h.savedCellWidths = h.savedCellWidths[:newLines]
		} else if len(h.savedCellWidths) < newLines {
			for len(h.savedCellWidths) < newLines {
				newRow := make([]int, newCols)
				for j := 0; j < newCols; j++ {
					newRow[j] = 1
				}
				h.savedCellWidths = append(h.savedCellWidths, newRow)
			}
		}

		for i := 0; i < len(h.savedCellWidths) && i < newLines; i++ {
			if h.savedCellWidths[i] == nil {
				h.savedCellWidths[i] = make([]int, newCols)
				for j := 0; j < newCols; j++ {
					h.savedCellWidths[i][j] = 1
				}
				continue
			}

			if len(h.savedCellWidths[i]) > newCols {
				h.savedCellWidths[i] = h.savedCellWidths[i][:newCols]
			} else if len(h.savedCellWidths[i]) < newCols {
				oldLen := len(h.savedCellWidths[i])
				for j := oldLen; j < newCols; j++ {
					h.savedCellWidths[i] = append(h.savedCellWidths[i], 1)
				}
			}
		}
	}
}

// GetMaxHistoryPos returns the maximum possible History position
func (h *HistoryScreen) GetMaxHistoryPos() int {
	if h.History == nil {
		return 0
	}
	return h.History.Len()
}

// IsAtTopOfHistory returns true if we're viewing the oldest available History
func (h *HistoryScreen) IsAtTopOfHistory() bool {
	if !h.ViewingHistory {
		return false
	}
	maxPos := h.History.Len()
	return h.HistoryPos >= maxPos
}

// IsAtBottomOfHistory returns true if we're at the current output (not in History)
func (h *HistoryScreen) IsAtBottomOfHistory() bool {
	return !h.ViewingHistory || h.HistoryPos <= 0
}

// GetScrollProgress returns a value between 0.0 (bottom) and 1.0 (top)
func (h *HistoryScreen) GetScrollProgress() float32 {
	if !h.ViewingHistory || h.History.Len() == 0 {
		return 0.0 // At bottom
	}

	maxPos := h.History.Len()
	if maxPos <= 0 {
		return 0.0
	}

	progress := float32(h.HistoryPos) / float32(maxPos)
	if progress > 1.0 {
		progress = 1.0
	}
	if progress < 0.0 {
		progress = 0.0
	}

	return progress
}

// ScrollToTop scrolls to the very beginning of History

func (h *HistoryScreen) ScrollToTop() {
	maxPos := h.History.Len()
	if maxPos <= 0 {
		return
	}

	// Save current screen if not already viewing History
	if !h.ViewingHistory {
		h.saveCurrentScreen()
		h.ViewingHistory = true
	}

	// CRITICAL: Set to the actual maximum, not maxPos-1
	h.HistoryPos = maxPos
	h.renderHistoryView()

	fmt.Printf("HistoryScreen.ScrollToTop: scrolled to absolute top, pos=%d/%d\n", h.HistoryPos, maxPos)
}

// ENHANCED renderHistoryView with better calculation

func (h *HistoryScreen) renderHistoryView() {
	// Clear the buffer first
	for i := 0; i < h.lines; i++ {
		for j := 0; j < h.columns; j++ {
			h.buffer[i][j] = ' '
			h.attrs[i][j] = Attributes{Fg: "default", Bg: "default"}
			if h.cellWidths != nil && i < len(h.cellWidths) && h.cellWidths[i] != nil {
				h.cellWidths[i][j] = 1
			}
		}
	}

	totalHistoryLines := h.History.Len()
	if totalHistoryLines == 0 {
		return
	}

	// ENHANCED CALCULATION: Handle the case where HistoryPos = totalHistoryLines
	var startHistoryIndex int
	var numHistoryLinesToShow int

	// CRITICAL FIX: When HistoryPos equals totalHistoryLines, we want to show the very beginning
	if h.HistoryPos >= totalHistoryLines {
		// At absolute top - show the oldest content first
		numHistoryLinesToShow = min(h.lines, totalHistoryLines)
		startHistoryIndex = 0 // Start from the very beginning

		fmt.Printf("renderHistoryView: At absolute top - showing %d lines from start\n", numHistoryLinesToShow)
	} else if h.HistoryPos >= h.lines {
		// Can fill entire screen with History
		numHistoryLinesToShow = h.lines
		startHistoryIndex = totalHistoryLines - h.HistoryPos
	} else {
		// Show some History + some current screen
		numHistoryLinesToShow = h.HistoryPos
		startHistoryIndex = totalHistoryLines - h.HistoryPos
	}

	// Safety bounds
	if startHistoryIndex < 0 {
		startHistoryIndex = 0
	}
	if numHistoryLinesToShow > h.lines {
		numHistoryLinesToShow = h.lines
	}
	if numHistoryLinesToShow > totalHistoryLines {
		numHistoryLinesToShow = totalHistoryLines
	}

	fmt.Printf("renderHistoryView: HistoryPos=%d/%d -> show %d History lines starting from index %d\n",
		h.HistoryPos, totalHistoryLines, numHistoryLinesToShow, startHistoryIndex)

	// Fill screen with History
	lineIdx := 0
	elem := h.History.Front()

	// Skip to start position
	for i := 0; i < startHistoryIndex && elem != nil; i++ {
		elem = elem.Next()
	}

	// Fill from History
	for lineIdx < numHistoryLinesToShow && elem != nil && lineIdx < h.lines {
		histLine := elem.Value.(HistoryLine)

		// Ensure we don't exceed array bounds
		if lineIdx < len(h.buffer) {
			copy(h.buffer[lineIdx], histLine.Chars)
			copy(h.attrs[lineIdx], histLine.Attrs)

			if h.cellWidths != nil && lineIdx < len(h.cellWidths) && h.cellWidths[lineIdx] != nil {
				if len(histLine.CellWidths) > 0 {
					copy(h.cellWidths[lineIdx], histLine.CellWidths)
				} else {
					for j := range h.cellWidths[lineIdx] {
						h.cellWidths[lineIdx][j] = 1
					}
				}
			}
		}

		elem = elem.Next()
		lineIdx++
	}

	// Fill remaining lines from saved buffer if needed
	if lineIdx < h.lines && h.savedBuffer != nil && numHistoryLinesToShow < h.lines {
		savedStart := 0
		linesToCopyFromSaved := h.lines - numHistoryLinesToShow

		for i := 0; i < linesToCopyFromSaved && lineIdx < h.lines && savedStart < len(h.savedBuffer); i++ {
			copy(h.buffer[lineIdx], h.savedBuffer[savedStart])
			copy(h.attrs[lineIdx], h.savedAttrs[savedStart])

			if h.cellWidths != nil && lineIdx < len(h.cellWidths) && h.cellWidths[lineIdx] != nil {
				if h.savedCellWidths != nil && savedStart < len(h.savedCellWidths) && h.savedCellWidths[savedStart] != nil {
					copy(h.cellWidths[lineIdx], h.savedCellWidths[savedStart])
				} else {
					for j := range h.cellWidths[lineIdx] {
						h.cellWidths[lineIdx][j] = 1
					}
				}
			}

			lineIdx++
			savedStart++
		}
	}

	// Hide cursor when viewing History
	h.cursor.Hidden = true

	fmt.Printf("renderHistoryView: filled %d lines total\n", lineIdx)
}