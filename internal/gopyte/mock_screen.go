package gopyte

import "fmt"

// MockScreen is a test implementation that logs all calls
type MockScreen struct {
	Calls []string
}

func NewMockScreen() *MockScreen {
	return &MockScreen{
		Calls: make([]string, 0),
	}
}

func (s *MockScreen) log(method string, args ...interface{}) {
	s.Calls = append(s.Calls, fmt.Sprintf("%s%v", method, args))
}

func (s *MockScreen) Draw(text string)                    { s.log("Draw", text) }
func (s *MockScreen) Bell()                               { s.log("Bell") }
func (s *MockScreen) Backspace()                          { s.log("Backspace") }
func (s *MockScreen) Tab()                                { s.log("Tab") }
func (s *MockScreen) Linefeed()                           { s.log("Linefeed") }
func (s *MockScreen) CarriageReturn()                     { s.log("CarriageReturn") }
func (s *MockScreen) ShiftOut()                           { s.log("ShiftOut") }
func (s *MockScreen) ShiftIn()                            { s.log("ShiftIn") }
func (s *MockScreen) CursorUp(count int)                  { s.log("CursorUp", count) }
func (s *MockScreen) CursorDown(count int)                { s.log("CursorDown", count) }
func (s *MockScreen) CursorForward(count int)             { s.log("CursorForward", count) }
func (s *MockScreen) CursorBack(count int)                { s.log("CursorBack", count) }
func (s *MockScreen) CursorUp1(count int)                 { s.log("CursorUp1", count) }
func (s *MockScreen) CursorDown1(count int)               { s.log("CursorDown1", count) }
func (s *MockScreen) CursorPosition(line, column int)     { s.log("CursorPosition", line, column) }
func (s *MockScreen) CursorToColumn(column int)           { s.log("CursorToColumn", column) }
func (s *MockScreen) CursorToLine(line int)               { s.log("CursorToLine", line) }
func (s *MockScreen) Reset()                              { s.log("Reset") }
func (s *MockScreen) Index()                              { s.log("Index") }
func (s *MockScreen) ReverseIndex()                       { s.log("ReverseIndex") }
func (s *MockScreen) SetTabStop()                         { s.log("SetTabStop") }
func (s *MockScreen) ClearTabStop(how int)                { s.log("ClearTabStop", how) }
func (s *MockScreen) SaveCursor()                         { s.log("SaveCursor") }
func (s *MockScreen) RestoreCursor()                      { s.log("RestoreCursor") }
func (s *MockScreen) InsertLines(count int)               { s.log("InsertLines", count) }
func (s *MockScreen) DeleteLines(count int)               { s.log("DeleteLines", count) }
func (s *MockScreen) InsertCharacters(count int)          { s.log("InsertCharacters", count) }
func (s *MockScreen) DeleteCharacters(count int)          { s.log("DeleteCharacters", count) }
func (s *MockScreen) EraseCharacters(count int)           { s.log("EraseCharacters", count) }
func (s *MockScreen) EraseInLine(how int, private bool)   { s.log("EraseInLine", how, private) }
func (s *MockScreen) EraseInDisplay(how int)              { s.log("EraseInDisplay", how) }
func (s *MockScreen) SetMode(modes []int, private bool)   { s.log("SetMode", modes, private) }
func (s *MockScreen) ResetMode(modes []int, private bool) { s.log("ResetMode", modes, private) }
func (s *MockScreen) SelectGraphicRendition(attrs []int)  { s.log("SelectGraphicRendition", attrs) }
func (s *MockScreen) DefineCharset(code, mode string)     { s.log("DefineCharset", code, mode) }
func (s *MockScreen) SetMargins(top, bottom int)          { s.log("SetMargins", top, bottom) }
func (s *MockScreen) ReportDeviceAttributes(mode int, priv bool) {
	s.log("ReportDeviceAttributes", mode, priv)
}
func (s *MockScreen) ReportDeviceStatus(mode int)   { s.log("ReportDeviceStatus", mode) }
func (s *MockScreen) SetTitle(title string)         { s.log("SetTitle", title) }
func (s *MockScreen) SetIconName(name string)       { s.log("SetIconName", name) }
func (s *MockScreen) AlignmentDisplay()             { s.log("AlignmentDisplay") }
func (s *MockScreen) Debug(args ...interface{})     { s.log("Debug", args...) }
func (s *MockScreen) WriteProcessInput(data string) { s.log("WriteProcessInput", data) }
