package gopyte

import (
	"fmt"
	"log"
	"strings"
)

// Screen represents a native Go terminal screen

type NativeScreen struct {
	columns int
	lines   int

	// Core data
	buffer [][]rune       // The actual character data
	attrs  [][]Attributes // Attributes for each cell
	cursor Cursor
	saved  *Cursor // For save/restore cursor

	// Simple state
	title     string
	iconName  string
	decomMode bool
	// Modes (we'll add as needed)
	autoWrap    bool
	newlineMode bool // LNM - if true, LF also does CR

	// Tab stops
	tabStops map[int]bool

	// Scroll regions (DECSTBM support)
	scrollTop       int  // Top of scroll region (0-based)
	scrollBottom    int  // Bottom of scroll region (0-based)
	scrollRegionSet bool // Whether custom scroll region is active
}

type Margins struct {
	Top    int
	Bottom int
}

type Cell struct {
	Char  rune
	Attrs Attributes
	Width int // 0 for continuation, 1 for normal, 2 for wide
}

type Cursor struct {
	X      int
	Y      int
	Attrs  Attributes // Current drawing attributes
	Hidden bool       // For DECTCEM mode
}

type Attributes struct {
	Fg            string // Foreground color ("default", "red", etc.)
	Bg            string // Background color
	Bold          bool
	Italics       bool
	Underscore    bool
	Strikethrough bool
	Reverse       bool
	Blink         bool
}

// NewNativeScreen creates a new terminal screen

func NewNativeScreen(columns, lines int) *NativeScreen {
	s := &NativeScreen{
		columns:         columns,
		lines:           lines,
		buffer:          make([][]rune, lines),
		attrs:           make([][]Attributes, lines),
		cursor:          Cursor{X: 0, Y: 0},
		autoWrap:        true,
		newlineMode:     true, // Default to Unix behavior where LF implies CR
		tabStops:        make(map[int]bool),
		scrollTop:       0,
		scrollBottom:    lines - 1,
		scrollRegionSet: false,
	}

	// Initialize buffer with spaces
	for i := 0; i < lines; i++ {
		s.buffer[i] = make([]rune, columns)
		s.attrs[i] = make([]Attributes, columns)
		for j := 0; j < columns; j++ {
			s.buffer[i][j] = ' '
		}
	}

	// Default tab stops every 8 columns
	for i := 0; i < columns; i += 8 {
		s.tabStops[i] = true
	}

	return s
}

func (s *NativeScreen) Draw(text string) {
	for _, ch := range text {
		// Check if we need to wrap
		if s.cursor.X >= s.columns {
			if s.autoWrap {
				s.cursor.X = 0
				s.cursor.Y++
				if s.cursor.Y >= s.lines {
					s.scrollUp()
					s.cursor.Y = s.lines - 1
				}
			} else {
				s.cursor.X = s.columns - 1
			}
		}

		// Place character
		if s.cursor.Y < s.lines && s.cursor.X < s.columns {
			s.buffer[s.cursor.Y][s.cursor.X] = ch
			s.attrs[s.cursor.Y][s.cursor.X] = s.cursor.Attrs
			s.cursor.X++
		}
	}
}

// 8. SavePoint support (for DECSC/DECRC)
type Savepoint struct {
	Cursor    Cursor
	G0Charset []rune
	G1Charset []rune
	Charset   int
	Origin    bool // DECOM mode
	Wrap      bool // DECAWM mode
}

func (s *NativeScreen) Bell() {
	// No-op for screen emulation
}

func (s *NativeScreen) Backspace() {
	if s.cursor.X > 0 {
		s.cursor.X--
	}
}

func (s *NativeScreen) Tab() {
	// Move to next tab stop
	for x := s.cursor.X + 1; x < s.columns; x++ {
		if s.tabStops[x] {
			s.cursor.X = x
			return
		}
	}
	s.cursor.X = s.columns - 1
}

func (s *NativeScreen) CarriageReturn() {
	s.cursor.X = 0
}

func (s *NativeScreen) ShiftOut() {
	// Character set switching - implement when needed
}

func (s *NativeScreen) ShiftIn() {
	// Character set switching - implement when needed
}

// === Cursor Movement ===

func (s *NativeScreen) CursorUp(count int) {
	newY := s.cursor.Y - count
	if s.scrollRegionSet && newY < s.scrollTop {
		s.cursor.Y = s.scrollTop
	} else if newY < 0 {
		s.cursor.Y = 0
	} else {
		s.cursor.Y = newY
	}
}

func (s *NativeScreen) CursorDown(count int) {
	newY := s.cursor.Y + count
	if s.scrollRegionSet && newY > s.scrollBottom {
		s.cursor.Y = s.scrollBottom
	} else if newY >= s.lines {
		s.cursor.Y = s.lines - 1
	} else {
		s.cursor.Y = newY
	}
}

func (s *NativeScreen) CursorForward(count int) {
	s.cursor.X += count
	if s.cursor.X >= s.columns {
		s.cursor.X = s.columns - 1
	}
}

func (s *NativeScreen) CursorBack(count int) {
	s.cursor.X -= count
	if s.cursor.X < 0 {
		s.cursor.X = 0
	}
}

func (s *NativeScreen) CursorUp1(count int) {
	// Move up and to column 0
	newY := s.cursor.Y - count
	if s.scrollRegionSet && newY < s.scrollTop {
		s.cursor.Y = s.scrollTop
	} else if newY < 0 {
		s.cursor.Y = 0
	} else {
		s.cursor.Y = newY
	}
	s.cursor.X = 0
}

func (s *NativeScreen) CursorDown1(count int) {
	// Move down and to column 0
	newY := s.cursor.Y + count
	if s.scrollRegionSet && newY > s.scrollBottom {
		s.cursor.Y = s.scrollBottom
	} else if newY >= s.lines {
		s.cursor.Y = s.lines - 1
	} else {
		s.cursor.Y = newY
	}
	s.cursor.X = 0
}

func (s *NativeScreen) CursorPosition(line, column int) {
	// Convert from 1-based to 0-based
	newY := line - 1
	newX := column - 1

	// Handle DECOM (origin mode) - coordinates relative to scroll region when enabled
	if s.decomMode && s.scrollRegionSet {
		// In origin mode with scroll region: coordinates are relative to scroll region
		newY += s.scrollTop
		// Clamp to scroll region
		if newY < s.scrollTop {
			newY = s.scrollTop
		} else if newY > s.scrollBottom {
			newY = s.scrollBottom
		}
	} else {
		// Not in origin mode: coordinates are absolute screen coordinates
		// Clamp to full screen
		if newY < 0 {
			newY = 0
		} else if newY >= s.lines {
			newY = s.lines - 1
		}
	}

	// Clamp X coordinate (always to full screen width)
	if newX < 0 {
		newX = 0
	} else if newX >= s.columns {
		newX = s.columns - 1
	}

	s.cursor.Y = newY
	s.cursor.X = newX

	log.Printf("CursorPosition: DECOM=%v, scrollRegion=%v, final pos=(%d,%d)",
		s.decomMode, s.scrollRegionSet, newX, newY)
}

func (s *NativeScreen) CursorToColumn(column int) {
	s.cursor.X = column - 1
	if s.cursor.X < 0 {
		s.cursor.X = 0
	} else if s.cursor.X >= s.columns {
		s.cursor.X = s.columns - 1
	}
}

func (s *NativeScreen) CursorToLine(line int) {
	newY := line - 1

	// Handle DECOM (origin mode) - if scroll region is set, line is relative to scroll region
	if s.scrollRegionSet {
		newY += s.scrollTop
		if newY < s.scrollTop {
			newY = s.scrollTop
		} else if newY > s.scrollBottom {
			newY = s.scrollBottom
		}
	} else {
		if newY < 0 {
			newY = 0
		} else if newY >= s.lines {
			newY = s.lines - 1
		}
	}

	s.cursor.Y = newY
}

// === Screen Manipulation ===

func (s *NativeScreen) Reset() {
	// Clear everything
	for i := 0; i < s.lines; i++ {
		for j := 0; j < s.columns; j++ {
			s.buffer[i][j] = ' '
			s.attrs[i][j] = Attributes{}
		}
	}

	// Reset cursor
	s.cursor = Cursor{X: 0, Y: 0}
	s.saved = nil

	// Reset modes
	s.autoWrap = true
	s.newlineMode = true

	// Reset scroll regions
	s.scrollTop = 0
	s.scrollBottom = s.lines - 1
	s.scrollRegionSet = false

	// Reset tab stops
	s.tabStops = make(map[int]bool)
	for i := 0; i < s.columns; i += 8 {
		s.tabStops[i] = true
	}
}

func (s *NativeScreen) scrollWithinMargins(top, bottom int) {
	// Move lines up within the margin area
	for y := top; y < bottom; y++ {
		s.buffer[y] = s.buffer[y+1]
		s.attrs[y] = s.attrs[y+1]
	}

	// Clear the bottom line in margin
	s.buffer[bottom] = make([]rune, s.columns)
	s.attrs[bottom] = make([]Attributes, s.columns)
	for x := 0; x < s.columns; x++ {
		s.buffer[bottom][x] = ' '
		s.attrs[bottom][x] = DefaultAttributes()
	}
}

func (s *NativeScreen) reverseScrollWithinMargins(top, bottom int) {
	// Move lines down within the margin area
	for y := bottom; y > top; y-- {
		s.buffer[y] = s.buffer[y-1]
		s.attrs[y] = s.attrs[y-1]
	}

	// Clear the top line in margin
	s.buffer[top] = make([]rune, s.columns)
	s.attrs[top] = make([]Attributes, s.columns)
	for x := 0; x < s.columns; x++ {
		s.buffer[top][x] = ' '
		s.attrs[top][x] = DefaultAttributes()
	}
}

func DefaultAttributes() Attributes {
	return Attributes{
		Fg: "default",
		Bg: "default",
	}
}

func (s *NativeScreen) SelectGraphicRendition(params []int) {
	if len(params) == 0 || (len(params) == 1 && params[0] == 0) {
		// Reset all attributes
		s.cursor.Attrs = DefaultAttributes()
		return
	}

	for i := 0; i < len(params); i++ {
		switch params[i] {
		case 0: // Reset
			s.cursor.Attrs = DefaultAttributes()
		case 1: // Bold
			s.cursor.Attrs.Bold = true
		case 3: // Italic
			s.cursor.Attrs.Italics = true
		case 4: // Underline
			s.cursor.Attrs.Underscore = true
		case 5: // Blink
			s.cursor.Attrs.Blink = true
		case 7: // Reverse
			s.cursor.Attrs.Reverse = true
		case 9: // Strikethrough
			s.cursor.Attrs.Strikethrough = true
		case 22: // Not bold
			s.cursor.Attrs.Bold = false
		case 23: // Not italic
			s.cursor.Attrs.Italics = false
		case 24: // Not underline
			s.cursor.Attrs.Underscore = false
		case 25: // Not blink
			s.cursor.Attrs.Blink = false
		case 27: // Not reverse
			s.cursor.Attrs.Reverse = false
		case 29: // Not strikethrough
			s.cursor.Attrs.Strikethrough = false
		// Foreground colors
		case 30:
			s.cursor.Attrs.Fg = "black"
		case 31:
			s.cursor.Attrs.Fg = "red"
		case 32:
			s.cursor.Attrs.Fg = "green"
		case 33:
			s.cursor.Attrs.Fg = "brown"
		case 34:
			s.cursor.Attrs.Fg = "blue"
		case 35:
			s.cursor.Attrs.Fg = "magenta"
		case 36:
			s.cursor.Attrs.Fg = "cyan"
		case 37:
			s.cursor.Attrs.Fg = "white"
		case 39:
			s.cursor.Attrs.Fg = "default"
		// Background colors
		case 40:
			s.cursor.Attrs.Bg = "black"
		case 41:
			s.cursor.Attrs.Bg = "red"
		case 42:
			s.cursor.Attrs.Bg = "green"
		case 43:
			s.cursor.Attrs.Bg = "brown"
		case 44:
			s.cursor.Attrs.Bg = "blue"
		case 45:
			s.cursor.Attrs.Bg = "magenta"
		case 46:
			s.cursor.Attrs.Bg = "cyan"
		case 47:
			s.cursor.Attrs.Bg = "white"
		case 49:
			s.cursor.Attrs.Bg = "default"
		// 256 colors
		case 38, 48:
			if i+2 < len(params) && params[i+1] == 5 {
				// 256 color mode
				color := params[i+2]
				if params[i] == 38 {
					s.cursor.Attrs.Fg = color256ToString(color)
				} else {
					s.cursor.Attrs.Bg = color256ToString(color)
				}
				i += 2
			}
		}
	}
}

// Helper for 256 color conversion
func color256ToString(n int) string {
	// For now, just return the number as string
	// Could map to actual color names or RGB values
	return fmt.Sprintf("color%d", n)
}

// In screen.go, fix the Index() method:

func (s *NativeScreen) Index() {
	if s.scrollRegionSet {
		// Within scroll region - check if we need to scroll
		if s.cursor.Y >= s.scrollBottom {
			// At or past bottom of scroll region - scroll within region
			s.scrollWithinMargins(s.scrollTop, s.scrollBottom)
			// Cursor stays at scrollBottom after scrolling
			s.cursor.Y = s.scrollBottom
		} else {
			// Not at bottom - move down normally
			s.cursor.Y++
		}
	} else {
		// No scroll region - check screen boundaries
		if s.cursor.Y >= s.lines-1 {
			// At bottom of screen - scroll entire screen
			s.scrollUp()
			// Cursor stays at bottom
			s.cursor.Y = s.lines - 1
		} else {
			// Not at bottom - move down normally
			s.cursor.Y++
		}
	}
}

// Also fix the Linefeed method in screen.go:
func (s *NativeScreen) Linefeed() {
	if s.scrollRegionSet {
		// Within scroll region - check if we need to scroll
		if s.cursor.Y >= s.scrollBottom {
			// At or past bottom of scroll region - scroll within region
			s.scrollWithinMargins(s.scrollTop, s.scrollBottom)
			// Cursor stays at scrollBottom after scrolling
			s.cursor.Y = s.scrollBottom
		} else {
			// Not at bottom of region - move down normally
			s.cursor.Y++
		}
	} else {
		// No scroll region - check screen boundaries
		if s.cursor.Y >= s.lines-1 {
			// At bottom of screen - scroll entire screen
			s.scrollUp()
			// Cursor stays at bottom
			s.cursor.Y = s.lines - 1
		} else {
			// Not at bottom - move down normally
			s.cursor.Y++
		}
	}

	// In newline mode (typical for Unix), LF also does CR
	if s.newlineMode {
		s.cursor.X = 0
	}
}

func (s *NativeScreen) ReverseIndex() {
	if s.scrollRegionSet {
		// Within scroll region - check boundaries
		if s.cursor.Y <= s.scrollTop {
			// At top of scroll region - reverse scroll within region only
			s.reverseScrollWithinMargins(s.scrollTop, s.scrollBottom)
			// Cursor stays at scrollTop
		} else {
			// Not at top - move up normally
			s.cursor.Y--
		}
	} else {
		// No scroll region - check screen boundaries
		if s.cursor.Y <= 0 {
			// At top of screen - reverse scroll entire screen
			s.scrollDown()
			// Cursor stays at top
		} else {
			// Not at top - move up normally
			s.cursor.Y--
		}
	}
}
func (s *NativeScreen) SetTabStop() {
	s.tabStops[s.cursor.X] = true
}

func (s *NativeScreen) ClearTabStop(how int) {
	switch how {
	case 0: // Clear tab at current position
		delete(s.tabStops, s.cursor.X)
	case 3: // Clear all tabs
		s.tabStops = make(map[int]bool)
	}
}

func (s *NativeScreen) SaveCursor() {
	saved := s.cursor // Copy
	s.saved = &saved
}

func (s *NativeScreen) RestoreCursor() {
	if s.saved != nil {
		s.cursor = *s.saved
		// Ensure cursor is within current scroll region bounds
		if s.scrollRegionSet {
			if s.cursor.Y < s.scrollTop {
				s.cursor.Y = s.scrollTop
			} else if s.cursor.Y > s.scrollBottom {
				s.cursor.Y = s.scrollBottom
			}
		}
		// Ensure cursor is within screen bounds
		if s.cursor.Y < 0 {
			s.cursor.Y = 0
		} else if s.cursor.Y >= s.lines {
			s.cursor.Y = s.lines - 1
		}
		if s.cursor.X < 0 {
			s.cursor.X = 0
		} else if s.cursor.X >= s.columns {
			s.cursor.X = s.columns - 1
		}
	}
}

// === Line Operations ===

func (s *NativeScreen) InsertLines(count int) {
	// Insert blank lines at cursor position within scroll region
	top := s.scrollTop
	bottom := s.scrollBottom

	if s.cursor.Y < top || s.cursor.Y > bottom {
		return // Outside scroll region
	}

	for i := 0; i < count && s.cursor.Y <= bottom; i++ {
		// Shift lines down within scroll region
		for y := bottom; y > s.cursor.Y; y-- {
			s.buffer[y] = s.buffer[y-1]
			s.attrs[y] = s.attrs[y-1]
		}

		// Clear the inserted line
		s.buffer[s.cursor.Y] = make([]rune, s.columns)
		s.attrs[s.cursor.Y] = make([]Attributes, s.columns)
		for j := 0; j < s.columns; j++ {
			s.buffer[s.cursor.Y][j] = ' '
			s.attrs[s.cursor.Y][j] = DefaultAttributes()
		}
	}
}

func (s *NativeScreen) DeleteLines(count int) {
	// Delete lines at cursor position within scroll region
	top := s.scrollTop
	bottom := s.scrollBottom

	if s.cursor.Y < top || s.cursor.Y > bottom {
		return // Outside scroll region
	}

	for i := 0; i < count && s.cursor.Y <= bottom; i++ {
		// Shift lines up within scroll region
		for y := s.cursor.Y; y < bottom; y++ {
			s.buffer[y] = s.buffer[y+1]
			s.attrs[y] = s.attrs[y+1]
		}

		// Clear the last line in scroll region
		s.buffer[bottom] = make([]rune, s.columns)
		s.attrs[bottom] = make([]Attributes, s.columns)
		for j := 0; j < s.columns; j++ {
			s.buffer[bottom][j] = ' '
			s.attrs[bottom][j] = DefaultAttributes()
		}
	}
}

func (s *NativeScreen) InsertCharacters(count int) {
	// Insert spaces at cursor position
	line := s.buffer[s.cursor.Y]
	for i := 0; i < count && s.cursor.X < s.columns; i++ {
		// Shift characters right
		copy(line[s.cursor.X+1:], line[s.cursor.X:s.columns-1])
		line[s.cursor.X] = ' '
	}
}

func (s *NativeScreen) DeleteCharacters(count int) {
	// Delete characters at cursor position
	line := s.buffer[s.cursor.Y]
	for i := 0; i < count && s.cursor.X < s.columns; i++ {
		// Shift characters left
		if s.cursor.X < s.columns-1 {
			copy(line[s.cursor.X:], line[s.cursor.X+1:])
		}
		line[s.columns-1] = ' '
	}
}

func (s *NativeScreen) EraseCharacters(count int) {
	// Erase characters at cursor position
	for i := 0; i < count && s.cursor.X+i < s.columns; i++ {
		s.buffer[s.cursor.Y][s.cursor.X+i] = ' '
		s.attrs[s.cursor.Y][s.cursor.X+i] = DefaultAttributes()
	}
}

func (s *NativeScreen) EraseInLine(how int, private bool) {
	switch how {
	case 0: // From cursor to end of line
		for x := s.cursor.X; x < s.columns; x++ {
			s.buffer[s.cursor.Y][x] = ' '
			s.attrs[s.cursor.Y][x] = DefaultAttributes()
		}
	case 1: // From beginning to cursor
		for x := 0; x <= s.cursor.X && x < s.columns; x++ {
			s.buffer[s.cursor.Y][x] = ' '
			s.attrs[s.cursor.Y][x] = DefaultAttributes()
		}
	case 2: // Entire line
		for x := 0; x < s.columns; x++ {
			s.buffer[s.cursor.Y][x] = ' '
			s.attrs[s.cursor.Y][x] = DefaultAttributes()
		}
	}
}

func (s *NativeScreen) EraseInDisplay(how int) {
	switch how {
	case 0: // From cursor to end
		s.EraseInLine(0, false)
		for y := s.cursor.Y + 1; y < s.lines; y++ {
			for x := 0; x < s.columns; x++ {
				s.buffer[y][x] = ' '
				s.attrs[y][x] = DefaultAttributes()
			}
		}
	case 1: // From beginning to cursor
		s.EraseInLine(1, false)
		for y := 0; y < s.cursor.Y; y++ {
			for x := 0; x < s.columns; x++ {
				s.buffer[y][x] = ' '
				s.attrs[y][x] = DefaultAttributes()
			}
		}
	case 2, 3: // Entire screen
		for y := 0; y < s.lines; y++ {
			for x := 0; x < s.columns; x++ {
				s.buffer[y][x] = ' '
				s.attrs[y][x] = DefaultAttributes()
			}
		}
	}
}

// === Mode Management ===

func (s *NativeScreen) SetMode(modes []int, private bool) {
	for _, mode := range modes {
		if private {
			// Private modes (DEC modes)
			switch mode {
			case 7: // DECAWM - Auto wrap mode
				s.autoWrap = true
			case 6: // DECOM - Origin mode
				s.decomMode = true
				// Move cursor to origin of scroll region (or screen if no region)
				if s.scrollRegionSet {
					s.cursor.X = 0
					s.cursor.Y = s.scrollTop
				} else {
					s.cursor.X = 0
					s.cursor.Y = 0
				}
				log.Printf("DECOM origin mode SET - cursor moved to (%d, %d)", s.cursor.X, s.cursor.Y)
			}
		} else {
			// Standard modes
			switch mode {
			case 20: // LNM - Newline mode
				s.newlineMode = true
			}
		}
	}
}

func (s *NativeScreen) ResetMode(modes []int, private bool) {
	for _, mode := range modes {
		if private {
			// Private modes (DEC modes)
			switch mode {
			case 7: // DECAWM - Auto wrap mode
				s.autoWrap = false
			case 6: // DECOM - Origin mode
				s.decomMode = false
				// Move cursor to absolute screen origin
				s.cursor.X = 0
				s.cursor.Y = 0
				log.Printf("DECOM origin mode RESET - cursor moved to (0, 0)")
			}
		} else {
			// Standard modes
			switch mode {
			case 20: // LNM - Newline mode
				s.newlineMode = false
			}
		}
	}
}

func (s *NativeScreen) DefineCharset(code, mode string) {
	// TODO: Implement charset switching
}

func (s *NativeScreen) SetMargins(top, bottom int) {
	log.Printf("SetMargins called: top=%d, bottom=%d (screen: %dx%d)", top, bottom, s.columns, s.lines)

	if top == 0 && bottom == 0 {
		// Reset to full screen
		s.scrollRegionSet = false
		s.scrollTop = 0
		s.scrollBottom = s.lines - 1
		log.Printf("Reset scroll region to full screen: 0-%d", s.lines-1)
	} else {
		// Set custom scroll region (convert from 1-based to 0-based)
		s.scrollRegionSet = true
		s.scrollTop = top - 1
		s.scrollBottom = bottom - 1

		// Clamp to screen bounds
		if s.scrollTop < 0 {
			s.scrollTop = 0
		}
		if s.scrollBottom >= s.lines {
			s.scrollBottom = s.lines - 1
		}
		if s.scrollTop > s.scrollBottom {
			s.scrollTop = 0
			s.scrollBottom = s.lines - 1
			s.scrollRegionSet = false
		}

		log.Printf("Set scroll region: %d-%d (0-based)", s.scrollTop, s.scrollBottom)
	}

	// Move cursor to top-left of scroll region (vim expects this)
	s.cursor.X = 0
	s.cursor.Y = s.scrollTop
	log.Printf("Moved cursor to scroll region origin: (%d, %d)", s.cursor.X, s.cursor.Y)
}

func (s *NativeScreen) ReportDeviceAttributes(mode int, private bool) {
	// TODO: Implement if needed
}

func (s *NativeScreen) ReportDeviceStatus(mode int) {
	// TODO: Implement if needed
}

func (s *NativeScreen) SetTitle(title string) {
	s.title = title
}

func (s *NativeScreen) SetIconName(name string) {
	s.iconName = name
}

func (s *NativeScreen) AlignmentDisplay() {
	// Fill screen with 'E' for alignment test
	for y := 0; y < s.lines; y++ {
		for x := 0; x < s.columns; x++ {
			s.buffer[y][x] = 'E'
		}
	}
}

func (s *NativeScreen) Debug(args ...interface{}) {
	// Could log somewhere if needed
}

func (s *NativeScreen) WriteProcessInput(data string) {
	// This would write back to the process - not needed for basic emulation
}

// === Helper methods ===

func (s *NativeScreen) scrollUp() {
	if s.scrollRegionSet {
		// Scroll within the defined region
		s.scrollWithinMargins(s.scrollTop, s.scrollBottom)
	} else {
		// Full screen scroll
		copy(s.buffer[0:], s.buffer[1:])
		copy(s.attrs[0:], s.attrs[1:])

		// Clear the last line
		lastLine := s.lines - 1
		s.buffer[lastLine] = make([]rune, s.columns)
		s.attrs[lastLine] = make([]Attributes, s.columns)
		for i := 0; i < s.columns; i++ {
			s.buffer[lastLine][i] = ' '
			s.attrs[lastLine][i] = DefaultAttributes()
		}
	}
}

func (s *NativeScreen) scrollDown() {
	if s.scrollRegionSet {
		// Reverse scroll within the defined region
		s.reverseScrollWithinMargins(s.scrollTop, s.scrollBottom)
	} else {
		// Full screen reverse scroll
		copy(s.buffer[1:], s.buffer[0:s.lines-1])
		copy(s.attrs[1:], s.attrs[0:s.lines-1])

		// Clear the first line
		s.buffer[0] = make([]rune, s.columns)
		s.attrs[0] = make([]Attributes, s.columns)
		for i := 0; i < s.columns; i++ {
			s.buffer[0][i] = ' '
			s.attrs[0][i] = DefaultAttributes()
		}
	}
}

// === Utility methods for testing ===

func (s *NativeScreen) GetDisplay() []string {
	lines := make([]string, s.lines)
	for i := 0; i < s.lines; i++ {
		lines[i] = strings.TrimRight(string(s.buffer[i]), " ")
	}
	return lines
}

func (s *NativeScreen) GetCursor() (int, int) {
	return s.cursor.X, s.cursor.Y
}

// Resize adjusts columns/lines on the base NativeScreen.
// - Column shrink: hard-truncate each row; grow: right-pad with spaces + default attrs
// - Row shrink: drop bottom rows; grow: append blank rows
// - Rebuild tab stops every 8 cols
// - Clamp cursor and scroll regions
func (s *NativeScreen) Resize(newCols, newLines int) {
	if newCols <= 0 || newLines <= 0 {
		return
	}
	if newCols == s.columns && newLines == s.lines {
		return
	}

	oldCols := s.columns
	oldLines := s.lines

	// Columns
	if newCols != oldCols {
		row := 0
		for row < oldLines && row < len(s.buffer) {
			// shrink
			if newCols < oldCols {
				if len(s.buffer[row]) > newCols {
					s.buffer[row] = s.buffer[row][:newCols]
					s.attrs[row] = s.attrs[row][:newCols]
				}
			} else {
				// grow
				add := newCols - len(s.buffer[row])
				if add > 0 {
					nb := make([]rune, len(s.buffer[row])+add)
					copy(nb, s.buffer[row])
					for i := len(s.buffer[row]); i < len(nb); i++ {
						nb[i] = ' '
					}
					na := make([]Attributes, len(s.attrs[row])+add)
					copy(na, s.attrs[row])
					for i := len(s.attrs[row]); i < len(na); i++ {
						na[i] = DefaultAttributes()
					}
					s.buffer[row] = nb
					s.attrs[row] = na
				}
			}
			row++
		}
	}

	// Rows
	if newLines < oldLines {
		// shrink: keep top portion, drop bottom lines
		s.buffer = s.buffer[:newLines]
		s.attrs = s.attrs[:newLines]
	} else if newLines > oldLines {
		// grow: append blank rows
		add := newLines - oldLines
		for i := 0; i < add; i++ {
			rowB := make([]rune, newCols)
			rowA := make([]Attributes, newCols)
			for x := 0; x < newCols; x++ {
				rowB[x] = ' '
				rowA[x] = DefaultAttributes()
			}
			s.buffer = append(s.buffer, rowB)
			s.attrs = append(s.attrs, rowA)
		}
	}

	// Commit new geometry
	s.columns = newCols
	s.lines = newLines

	// Update scroll regions
	if s.scrollBottom >= newLines {
		s.scrollBottom = newLines - 1
	}
	if s.scrollTop >= newLines {
		s.scrollTop = newLines - 1
	}
	if s.scrollTop > s.scrollBottom {
		s.scrollTop = 0
		s.scrollBottom = newLines - 1
		s.scrollRegionSet = false
	}

	// Clamp cursor
	if s.cursor.Y >= s.lines {
		s.cursor.Y = s.lines - 1
	}
	if s.cursor.X >= s.columns {
		s.cursor.X = s.columns - 1
	}
	if s.cursor.Y < 0 {
		s.cursor.Y = 0
	}
	if s.cursor.X < 0 {
		s.cursor.X = 0
	}

	// Rebuild tab stops
	s.tabStops = make(map[int]bool)
	for i := 0; i < s.columns; i += 8 {
		s.tabStops[i] = true
	}

	log.Printf("Screen resized to %dx%d, scroll region: %d-%d", newCols, newLines, s.scrollTop, s.scrollBottom)
}
