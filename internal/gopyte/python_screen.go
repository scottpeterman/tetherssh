package gopyte

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"time"
)

type PythonScreen struct {
	cmd    *exec.Cmd
	stdin  io.WriteCloser
	stdout *bufio.Scanner
	stderr io.ReadCloser
}

func NewPythonScreen(columns, lines int) (*PythonScreen, error) {
	// Try to find Python executable
	pythonCmd := "python"
	if _, err := exec.LookPath(pythonCmd); err != nil {
		pythonCmd = "python"
		if _, err := exec.LookPath(pythonCmd); err != nil {
			return nil, fmt.Errorf("Python not found in PATH")
		}
	}

	// Find the pyte_server.py file
	// Try several locations relative to where tests might run from
	possiblePaths := []string{
		"pyte/pyte_server.py",       // From project root
		"../pyte/pyte_server.py",    // From gopyte_test directory
		"./pyte_server.py",          // Current directory
		"../../pyte/pyte_server.py", // In case we're deeper
	}

	var serverPath string
	for _, path := range possiblePaths {
		if _, err := os.Stat(path); err == nil {
			serverPath, _ = filepath.Abs(path)
			break
		}
	}

	if serverPath == "" {
		// Try to find it relative to the executable
		if executable, err := os.Executable(); err == nil {
			execDir := filepath.Dir(executable)
			testPath := filepath.Join(execDir, "..", "pyte", "pyte_server.py")
			if _, err := os.Stat(testPath); err == nil {
				serverPath = testPath
			}
		}
	}

	if serverPath == "" {
		return nil, fmt.Errorf("could not find pyte_server.py in any expected location")
	}

	fmt.Fprintf(os.Stderr, "Using Python server at: %s\n", serverPath)

	cmd := exec.Command(pythonCmd, serverPath)

	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, fmt.Errorf("failed to create stdin pipe: %w", err)
	}

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("failed to create stdout pipe: %w", err)
	}

	stderr, err := cmd.StderrPipe()
	if err != nil {
		return nil, fmt.Errorf("failed to create stderr pipe: %w", err)
	}

	// Capture stderr for debugging
	go func() {
		scanner := bufio.NewScanner(stderr)
		for scanner.Scan() {
			fmt.Fprintf(os.Stderr, "Python stderr: %s\n", scanner.Text())
		}
	}()

	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("failed to start Python screen: %w", err)
	}

	ps := &PythonScreen{
		cmd:    cmd,
		stdin:  stdin,
		stdout: bufio.NewScanner(stdout),
		stderr: stderr,
	}

	// Give the Python process a moment to start
	time.Sleep(100 * time.Millisecond)

	// Initialize with size
	err = ps.call("__init__", []interface{}{columns, lines}, nil)
	if err != nil {
		ps.Close()
		return nil, fmt.Errorf("failed to initialize screen: %w", err)
	}

	return ps, nil
}

func (s *PythonScreen) call(method string, args []interface{}, kwargs map[string]interface{}) error {
	if s == nil || s.stdin == nil {
		return fmt.Errorf("PythonScreen not initialized")
	}

	if args == nil {
		args = []interface{}{}
	}
	if kwargs == nil {
		kwargs = map[string]interface{}{}
	}

	request := map[string]interface{}{
		"method": method,
		"args":   args,
		"kwargs": kwargs,
	}

	data, err := json.Marshal(request)
	if err != nil {
		return fmt.Errorf("failed to marshal request: %w", err)
	}

	_, err = fmt.Fprintf(s.stdin, "%s\n", data)
	if err != nil {
		return fmt.Errorf("failed to write to stdin: %w", err)
	}

	// Set a timeout for reading response
	done := make(chan bool, 1)
	var response map[string]interface{}
	var scanErr error

	go func() {
		if s.stdout.Scan() {
			if err := json.Unmarshal(s.stdout.Bytes(), &response); err != nil {
				scanErr = fmt.Errorf("failed to unmarshal response: %w", err)
			}
		} else {
			scanErr = fmt.Errorf("no response from Python screen")
		}
		done <- true
	}()

	select {
	case <-done:
		if scanErr != nil {
			return scanErr
		}
	case <-time.After(2 * time.Second):
		return fmt.Errorf("timeout waiting for Python response")
	}

	if success, ok := response["success"].(bool); ok && !success {
		errMsg := "unknown error"
		if msg, ok := response["error"].(string); ok {
			errMsg = msg
		}
		if trace, ok := response["trace"].(string); ok {
			return fmt.Errorf("Python error: %s\nTrace: %s", errMsg, trace)
		}
		return fmt.Errorf("Python error: %s", errMsg)
	}

	return nil
}

// Basic operations
func (s *PythonScreen) Draw(text string) {
	s.call("draw", []interface{}{text}, nil)
}

func (s *PythonScreen) Bell() {
	s.call("bell", nil, nil)
}

func (s *PythonScreen) Backspace() {
	s.call("backspace", nil, nil)
}

func (s *PythonScreen) Tab() {
	s.call("tab", nil, nil)
}

func (s *PythonScreen) Linefeed() {
	s.call("linefeed", nil, nil)
}

func (s *PythonScreen) CarriageReturn() {
	s.call("carriage_return", nil, nil)
}

func (s *PythonScreen) ShiftOut() {
	s.call("shift_out", nil, nil)
}

func (s *PythonScreen) ShiftIn() {
	s.call("shift_in", nil, nil)
}

// Cursor movement
func (s *PythonScreen) CursorUp(count int) {
	s.call("cursor_up", []interface{}{count}, nil)
}

func (s *PythonScreen) CursorDown(count int) {
	s.call("cursor_down", []interface{}{count}, nil)
}

func (s *PythonScreen) CursorForward(count int) {
	s.call("cursor_forward", []interface{}{count}, nil)
}

func (s *PythonScreen) CursorBack(count int) {
	s.call("cursor_back", []interface{}{count}, nil)
}

func (s *PythonScreen) CursorUp1(count int) {
	s.call("cursor_up1", []interface{}{count}, nil)
}

func (s *PythonScreen) CursorDown1(count int) {
	s.call("cursor_down1", []interface{}{count}, nil)
}

func (s *PythonScreen) CursorPosition(line, column int) {
	s.call("cursor_position", []interface{}{line, column}, nil)
}

func (s *PythonScreen) CursorToColumn(column int) {
	s.call("cursor_to_column", []interface{}{column}, nil)
}

func (s *PythonScreen) CursorToLine(line int) {
	s.call("cursor_to_line", []interface{}{line}, nil)
}

// Screen manipulation
func (s *PythonScreen) Reset() {
	s.call("reset", nil, nil)
}

func (s *PythonScreen) Index() {
	s.call("index", nil, nil)
}

func (s *PythonScreen) ReverseIndex() {
	s.call("reverse_index", nil, nil)
}

func (s *PythonScreen) SetTabStop() {
	s.call("set_tab_stop", nil, nil)
}

func (s *PythonScreen) ClearTabStop(how int) {
	s.call("clear_tab_stop", []interface{}{how}, nil)
}

func (s *PythonScreen) SaveCursor() {
	s.call("save_cursor", nil, nil)
}

func (s *PythonScreen) RestoreCursor() {
	s.call("restore_cursor", nil, nil)
}

// Line operations
func (s *PythonScreen) InsertLines(count int) {
	s.call("insert_lines", []interface{}{count}, nil)
}

func (s *PythonScreen) DeleteLines(count int) {
	s.call("delete_lines", []interface{}{count}, nil)
}

func (s *PythonScreen) InsertCharacters(count int) {
	s.call("insert_characters", []interface{}{count}, nil)
}

func (s *PythonScreen) DeleteCharacters(count int) {
	s.call("delete_characters", []interface{}{count}, nil)
}

func (s *PythonScreen) EraseCharacters(count int) {
	s.call("erase_characters", []interface{}{count}, nil)
}

func (s *PythonScreen) EraseInLine(how int, private bool) {
	s.call("erase_in_line", []interface{}{how}, map[string]interface{}{"private": private})
}

func (s *PythonScreen) EraseInDisplay(how int) {
	s.call("erase_in_display", []interface{}{how}, nil)
}

// Modes
func (s *PythonScreen) SetMode(modes []int, private bool) {
	args := make([]interface{}, len(modes))
	for i, m := range modes {
		args[i] = m
	}
	s.call("set_mode", args, map[string]interface{}{"private": private})
}

func (s *PythonScreen) ResetMode(modes []int, private bool) {
	args := make([]interface{}, len(modes))
	for i, m := range modes {
		args[i] = m
	}
	s.call("reset_mode", args, map[string]interface{}{"private": private})
}

// Attributes
func (s *PythonScreen) SelectGraphicRendition(attrs []int) {
	args := make([]interface{}, len(attrs))
	for i, a := range attrs {
		args[i] = a
	}
	s.call("select_graphic_rendition", args, nil)
}

// Charset
func (s *PythonScreen) DefineCharset(code, mode string) {
	s.call("define_charset", []interface{}{code, mode}, nil)
}

// Margins
func (s *PythonScreen) SetMargins(top, bottom int) {
	s.call("set_margins", []interface{}{top, bottom}, nil)
}

// Reports
func (s *PythonScreen) ReportDeviceAttributes(mode int, private bool) {
	s.call("report_device_attributes", []interface{}{mode}, map[string]interface{}{"private": private})
}

func (s *PythonScreen) ReportDeviceStatus(mode int) {
	s.call("report_device_status", []interface{}{mode}, nil)
}

// Window operations
func (s *PythonScreen) SetTitle(title string) {
	s.call("set_title", []interface{}{title}, nil)
}

func (s *PythonScreen) SetIconName(name string) {
	s.call("set_icon_name", []interface{}{name}, nil)
}

// Alignment
func (s *PythonScreen) AlignmentDisplay() {
	s.call("alignment_display", nil, nil)
}

// Debug
func (s *PythonScreen) Debug(args ...interface{}) {
	s.call("debug", args, nil)
}

// Process communication
func (s *PythonScreen) WriteProcessInput(data string) {
	s.call("write_process_input", []interface{}{data}, nil)
}

func (s *PythonScreen) Close() error {
	if s == nil {
		return nil
	}
	if s.stdin != nil {
		s.stdin.Close()
	}
	if s.cmd != nil {
		return s.cmd.Wait()
	}
	return nil
}
