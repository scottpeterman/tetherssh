package gopyte

// Screen represents a terminal screen that can handle ANSI escape sequences
type Screen interface {
	// Basic drawing
	Draw(text string)
	Bell()
	Backspace()
	Tab()
	Linefeed()
	CarriageReturn()
	ShiftOut()
	ShiftIn()

	// Cursor movement
	CursorUp(count int)
	CursorDown(count int)
	CursorForward(count int)
	CursorBack(count int)
	CursorUp1(count int)
	CursorDown1(count int)
	CursorPosition(line, column int)
	CursorToColumn(column int)
	CursorToLine(line int)

	// Screen manipulation
	Reset()
	Index()
	ReverseIndex()
	SetTabStop()
	ClearTabStop(how int)
	SaveCursor()
	RestoreCursor()

	// Line operations
	InsertLines(count int)
	DeleteLines(count int)
	InsertCharacters(count int)
	DeleteCharacters(count int)
	EraseCharacters(count int)
	EraseInLine(how int, private bool)
	EraseInDisplay(how int)

	// Mode setting
	SetMode(modes []int, private bool)
	ResetMode(modes []int, private bool)

	// Character sets
	DefineCharset(code, mode string)

	// Scrolling regions
	SetMargins(top, bottom int)

	// Graphics
	SelectGraphicRendition(params []int)

	// Reporting
	ReportDeviceAttributes(mode int, private bool)
	ReportDeviceStatus(mode int)

	// Window operations
	SetTitle(title string)
	SetIconName(name string)

	// Misc
	AlignmentDisplay()
	Debug(args ...interface{})
	WriteProcessInput(data string)
}

// Note: GetDisplay() and GetCursor() are available on NativeScreen
// and HistoryScreen as concrete methods, not part of the interface.
// This maintains backward compatibility with MockScreen and PythonScreen.
