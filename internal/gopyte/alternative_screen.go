package gopyte

import (
	"container/list"
)

// AlternateScreen adds alternative screen buffer support to HistoryScreen
// This is used by applications like vim, less, etc.
type AlternateScreen struct {
	*HistoryScreen

	// Alternative screen state
	mainBuffer   [][]rune
	mainAttrs    [][]Attributes
	mainCursor   Cursor
	mainTabStops map[int]bool
	mainHistory  *list.List

	altBuffer   [][]rune
	altAttrs    [][]Attributes
	altCursor   Cursor
	altTabStops map[int]bool

	usingAlternate bool
}

// NewAlternateScreen creates a screen with both main and alternate buffers
func NewAlternateScreen(columns, lines, maxHistory int) *AlternateScreen {
	a := &AlternateScreen{
		HistoryScreen:  NewHistoryScreen(columns, lines, maxHistory),
		usingAlternate: false,
	}

	// Initialize alternate buffer
	a.altBuffer = make([][]rune, lines)
	a.altAttrs = make([][]Attributes, lines)
	for i := 0; i < lines; i++ {
		a.altBuffer[i] = make([]rune, columns)
		a.altAttrs[i] = make([]Attributes, columns)
		for j := 0; j < columns; j++ {
			a.altBuffer[i][j] = ' '
		}
	}

	// Initialize alternate tab stops
	a.altTabStops = make(map[int]bool)
	for i := 0; i < columns; i += 8 {
		a.altTabStops[i] = true
	}

	return a
}

// Override SetMode to handle alternate screen switching
func (a *AlternateScreen) SetMode(modes []int, private bool) {
	if private {
		for _, mode := range modes {
			switch mode {
			case 1049, 1047, 47: // Alternate screen modes
				if !a.usingAlternate {
					a.switchToAlternate()
				}
			case 1048: // Save cursor
				if a.usingAlternate {
					a.altCursor = a.cursor
				} else {
					a.mainCursor = a.cursor
				}
			}
		}
	}

	// Call parent implementation for other modes
	a.HistoryScreen.SetMode(modes, private)
}

// Override ResetMode to handle alternate screen switching
func (a *AlternateScreen) ResetMode(modes []int, private bool) {
	if private {
		for _, mode := range modes {
			switch mode {
			case 1049, 1047, 47: // Exit alternate screen
				if a.usingAlternate {
					a.switchToMain()
				}
			case 1048: // Restore cursor
				if a.usingAlternate {
					a.cursor = a.altCursor
				} else {
					a.cursor = a.mainCursor
				}
			}
		}
	}

	// Call parent implementation for other modes
	a.HistoryScreen.ResetMode(modes, private)
}

// switchToAlternate switches to the alternate screen buffer
func (a *AlternateScreen) switchToAlternate() {
	// Save main screen state
	a.mainBuffer = a.buffer
	a.mainAttrs = a.attrs
	a.mainCursor = a.cursor
	a.mainTabStops = a.tabStops
	a.mainHistory = a.History

	// Clear alternate buffer before switching
	for i := 0; i < a.lines; i++ {
		for j := 0; j < a.columns; j++ {
			a.altBuffer[i][j] = ' '
			a.altAttrs[i][j] = DefaultAttributes()
		}
	}

	// Switch to alternate
	a.buffer = a.altBuffer
	a.attrs = a.altAttrs
	a.cursor = Cursor{X: 0, Y: 0, Attrs: DefaultAttributes()}
	a.tabStops = a.altTabStops

	// Alternate screen doesn't use History, use empty list
	a.History = list.New()
	a.usingAlternate = true

	// If we were viewing History, exit that mode
	if a.ViewingHistory {
		a.ViewingHistory = false
		a.HistoryPos = 0
	}
}

// switchToMain switches back to the main screen buffer
func (a *AlternateScreen) switchToMain() {
	if !a.usingAlternate {
		return
	}

	// Save alternate state (in case we switch back)
	a.altBuffer = a.buffer
	a.altAttrs = a.attrs
	a.altCursor = a.cursor
	a.altTabStops = a.tabStops

	// Restore main screen
	a.buffer = a.mainBuffer
	a.attrs = a.mainAttrs
	a.cursor = a.mainCursor
	a.tabStops = a.mainTabStops
	a.History = a.mainHistory

	a.usingAlternate = false
}

// Override methods that shouldn't save to History in alternate mode

func (a *AlternateScreen) Linefeed() {
	if a.usingAlternate {
		// Check if at bottom BEFORE incrementing
		if a.cursor.Y == a.lines-1 {
			// At bottom, scroll without History
			a.scrollUpNoHistory()
			// Stay at bottom
		} else {
			// Not at bottom, move down
			a.cursor.Y++
		}

		if a.newlineMode {
			a.cursor.X = 0
		}
	} else {
		// Use parent implementation with History
		a.HistoryScreen.Linefeed()
	}
}

func (a *AlternateScreen) Index() {
	if a.usingAlternate {
		// Check if at bottom BEFORE incrementing
		if a.cursor.Y == a.lines-1 {
			// At bottom, scroll without History
			a.scrollUpNoHistory()
			// Stay at bottom
		} else {
			// Not at bottom, move down
			a.cursor.Y++
		}
	} else {
		a.HistoryScreen.Index()
	}
}

// scrollUpNoHistory scrolls without saving to History (for alternate screen)
func (a *AlternateScreen) scrollUpNoHistory() {
	// Move all lines up by one
	copy(a.buffer[0:], a.buffer[1:])
	copy(a.attrs[0:], a.attrs[1:])

	// Clear the last line
	lastLine := a.lines - 1
	a.buffer[lastLine] = make([]rune, a.columns)
	a.attrs[lastLine] = make([]Attributes, a.columns)
	for i := 0; i < a.columns; i++ {
		a.buffer[lastLine][i] = ' '
	}
}

// Override Draw to handle alternate screen
func (a *AlternateScreen) Draw(text string) {
	if a.usingAlternate {
		// Don't exit History mode in alternate screen (there is no History)
		// Just draw normally using the base implementation
		a.drawTextDirect(text)
	} else {
		a.HistoryScreen.Draw(text)
	}
}

// drawTextDirect draws text without History handling
func (a *AlternateScreen) drawTextDirect(text string) {
	for _, ch := range text {
		// Check if we need to wrap
		if a.cursor.X >= a.columns {
			if a.autoWrap {
				a.cursor.X = 0
				a.cursor.Y++
				if a.cursor.Y >= a.lines {
					a.scrollUpNoHistory()
					a.cursor.Y = a.lines - 1
				}
			} else {
				a.cursor.X = a.columns - 1
			}
		}

		// Place character
		if a.cursor.Y < a.lines && a.cursor.X < a.columns {
			a.buffer[a.cursor.Y][a.cursor.X] = ch
			a.attrs[a.cursor.Y][a.cursor.X] = a.cursor.Attrs
			a.cursor.X++
		}
	}
}

// ensureRowSize makes sure row slices match the current column count.
func (a *AlternateScreen) ensureRowSize() {
	y := 0
	for y < a.lines {
		// buffer
		if len(a.buffer[y]) != a.columns {
			if len(a.buffer[y]) > a.columns {
				a.buffer[y] = a.buffer[y][:a.columns]
			} else {
				diff := a.columns - len(a.buffer[y])
				pad := make([]rune, diff)
				i := 0
				for i < diff {
					pad[i] = ' '
					i++
				}
				a.buffer[y] = append(a.buffer[y], pad...)
			}
		}
		// attrs
		if len(a.attrs[y]) != a.columns {
			if len(a.attrs[y]) > a.columns {
				a.attrs[y] = a.attrs[y][:a.columns]
			} else {
				diff := a.columns - len(a.attrs[y])
				pad := make([]Attributes, diff)
				i := 0
				for i < diff {
					pad[i] = DefaultAttributes()
					i++
				}
				a.attrs[y] = append(a.attrs[y], pad...)
			}
		}
		y++
	}
}

// Reset clears the current (active) buffer safely without writing out of bounds.
func (a *AlternateScreen) Reset() {
	// Normalize row widths first
	a.ensureRowSize()

	y := 0
	for y < a.lines {
		x := 0
		for x < a.columns { // strictly <, not <=
			a.buffer[y][x] = ' '
			a.attrs[y][x] = DefaultAttributes()
			x++
		}
		y++
	}

	a.cursor.X = 0
	a.cursor.Y = 0
	a.savedCursor.X = 0
	a.savedCursor.Y = 0

	// Rebuild tab stops at every 8th column
	a.tabStops = make(map[int]bool)
	i := 0
	for i < a.columns {
		a.tabStops[i] = true
		i += 8
	}
}

// IsUsingAlternate returns true if using alternate screen buffer
func (a *AlternateScreen) IsUsingAlternate() bool {
	return a.usingAlternate
}

// Override History methods to disable in alternate screen
func (a *AlternateScreen) ScrollUp(lines int) {
	if !a.usingAlternate {
		a.HistoryScreen.ScrollUp(lines)
	}
	// No-op in alternate screen
}

func (a *AlternateScreen) ScrollDown(lines int) {
	if !a.usingAlternate {
		a.HistoryScreen.ScrollDown(lines)
	}
	// No-op in alternate screen
}

func (a *AlternateScreen) ScrollToBottom() {
	if !a.usingAlternate {
		a.HistoryScreen.ScrollToBottom()
	}
	// No-op in alternate screen
}

// Resize adjusts both main and alternate buffers.
// Policy:
// - If usingAlternate: resize ONLY the alt buffer, NO History changes.
// - If on main: resize main; when shrinking rows, also push bottom lines into History.
func (a *AlternateScreen) Resize(newCols, newLines int) {
	if newCols <= 0 || newLines <= 0 {
		return
	}
	if a.usingAlternate {
		// Resize the alt buffer “in place” by temporarily making it active,
		// delegating to base, then restoring invariants already held.
		// (We are already on alt; Native/History paths operate on a.buffer/a.attrs)
		a.HistoryScreen.Resize(newCols, newLines) // History code is inert here (alt uses empty list)
		// Rebuild alt tab stops for the new width
		a.altTabStops = make(map[int]bool)
		for i := 0; i < newCols; i += 8 {
			a.altTabStops[i] = true
		}
		return
	}

	// Not using alternate: we must resize the MAIN buffer/state.
	// Temporarily switch pointers to main state, call HistoryScreen.Resize, then restore.
	// Save currently active (main) state is already in a.buffer/a.attrs/etc.
	a.HistoryScreen.Resize(newCols, newLines)

	// Rebuild main tab stops captured in a.tabStops by HistoryScreen.Resize → NativeScreen.Resize
	// Nothing else to do; alt buffer will be resized lazily on first entry if desired.
}
