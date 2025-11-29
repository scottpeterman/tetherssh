// pty_common.go - Shared PTY interfaces and methods
package main

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"syscall"
	"time"

	"github.com/creack/pty" // This is available on Unix systems
)

// PTY interface abstractions for cross-platform support
type PTYInterface interface {
	Write([]byte) (int, error)
	Read([]byte) (int, error)
	Close() error
	Resize(cols, rows int) error
}

// PTYManager contains all PTY-related state and functionality
type PTYManager struct {
	pty PTYInterface
	cmd *exec.Cmd

	// History buffer management
	historyBuffer     []string
	maxHistoryLines   int
	currentHistoryPos int
	inHistoryMode     bool

	// Virtual scrolling state
	virtualOffset    int
	maxVirtualOffset int
	viewportHeight   int
}

// Unix PTY wrapper - kept here since it uses creack/pty which works on Unix
type UnixPTY struct {
	ptyFile *os.File
	cmd     *exec.Cmd
}

func (u *UnixPTY) Write(data []byte) (int, error) {
	if u.ptyFile != nil {
		return u.ptyFile.Write(data)
	}
	return 0, fmt.Errorf("PTY file not available")
}

func (u *UnixPTY) Read(data []byte) (int, error) {
	if u.ptyFile != nil {
		return u.ptyFile.Read(data)
	}
	return 0, fmt.Errorf("PTY file not available")
}

func (u *UnixPTY) Close() error {
	var errs []error

	if u.ptyFile != nil {
		if err := u.ptyFile.Close(); err != nil {
			errs = append(errs, err)
		}
	}

	if u.cmd != nil && u.cmd.Process != nil {
		// Try graceful shutdown first
		if runtime.GOOS != "windows" {
			u.cmd.Process.Signal(syscall.SIGTERM)
			time.Sleep(100 * time.Millisecond)
		}

		if err := u.cmd.Process.Kill(); err != nil {
			errs = append(errs, err)
		}
	}

	if len(errs) > 0 {
		return fmt.Errorf("multiple close errors: %v", errs)
	}
	return nil
}

func (u *UnixPTY) Resize(cols, rows int) error {
	if u.ptyFile != nil {
		return pty.Setsize(u.ptyFile, &pty.Winsize{
			Rows: uint16(rows),
			Cols: uint16(cols),
		})
	}
	return fmt.Errorf("PTY file not available")
}

// Shared terminal widget methods that work on all platforms
func (t *NativeTerminalWidget) StartShell() error {
	log.Printf("Starting shell with unified PTY management")

	// Initialize PTY manager if not exists
	if t.ptyManager == nil {
		t.ptyManager = &PTYManager{
			maxHistoryLines: 2000,
			viewportHeight:  t.rows,
			historyBuffer:   make([]string, 0, 1000),
		}
	}

	var err error

	if runtime.GOOS == "windows" {
		t.ptyManager.pty, err = t.createWindowsPTY()
	} else {
		t.ptyManager.pty, err = t.createUnixPTY()
	}

	if err != nil {
		return fmt.Errorf("failed to create PTY: %v", err)
	}

	log.Printf("PTY created successfully on %s", runtime.GOOS)

	// Start reading from PTY in background
	go t.readFromPTYUnified()
	return nil
}

func (t *NativeTerminalWidget) readFromPTYUnified() {
	buffer := make([]byte, 4096)

	for {
		select {
		case <-t.ctx.Done():
			log.Printf("PTY reader stopping due to context cancellation")
			return
		default:
			if t.ptyManager != nil && t.ptyManager.pty != nil {
				n, err := t.ptyManager.pty.Read(buffer)
				if err != nil {
					if !strings.Contains(err.Error(), "closed") {
						log.Printf("PTY read error: %v", err)
					}
					return
				}

				if n > 0 {
					// Create copy and send to processing
					data := make([]byte, n)
					copy(data, buffer[:n])

					select {
					case t.updateChannel <- data:
						// Successfully queued
					case <-t.ctx.Done():
						return
					default:
						log.Printf("Update channel full, dropping %d bytes", n)
					}
				}
			}
		}
	}
}

func (t *NativeTerminalWidget) processTerminalDataUnified(data []byte) {
	t.mutex.Lock()
	defer t.mutex.Unlock()

	// Feed data to gopyte stream for parsing FIRST
	func() {
		defer func() {
			if r := recover(); r != nil {
				log.Printf("Error feeding data to stream: %v", r)
			}
		}()
		t.stream.Feed(string(data))
	}()

	// CRITICAL FIX: Handle alternate screen mode completely differently
	if t.screen != nil && t.screen.IsUsingAlternate() {
		log.Printf("Alternate screen mode: forcing immediate update after data processing")

		// In alternate screen mode, vim/apps handle everything
		// Just mark for immediate display update and skip all history management
		t.updatePending = true

		// Force cache invalidation to ensure display refreshes
		if t.screen != nil {
			t.screen.InvalidateCache()
		}

		// Extract and log any window title changes for debugging
		t.extractWindowTitle(string(data))

		return // Skip ALL history management in alternate screen mode
	}

	// MAIN TERMINAL MODE - Only execute this section for normal shell use
	log.Printf("Main terminal mode: processing data with history management")

	// Track if we were previously in history mode
	wasInHistoryMode := false
	if t.ptyManager != nil {
		wasInHistoryMode = t.ptyManager.inHistoryMode
	}

	// Update history buffer with new content (main terminal only)
	t.updateHistoryBuffer()

	// Handle mode transitions for main terminal only
	if wasInHistoryMode {
		// Force exit history mode when new output arrives in main terminal
		log.Printf("New output arrived in main terminal, exiting history mode")
		if t.ptyManager != nil {
			t.ptyManager.inHistoryMode = false
			t.ptyManager.currentHistoryPos = 0
			t.ptyManager.virtualOffset = 0
		}

		// Ensure we scroll to show new content
		if t.screen != nil {
			t.screen.ScrollToBottom()
		}
	}

	// Process escape sequences for special handling (main terminal only)
	t.handleEscapeSequences(string(data))

	// Extract any window title changes
	t.extractWindowTitle(string(data))

	// Mark for display update
	t.updatePending = true

	log.Printf("Main terminal mode: data processing completed, update pending")
}

func (t *NativeTerminalWidget) updateHistoryBuffer() {
	// Skip all history updates in alternate screen mode
	if t.screen != nil && t.screen.IsUsingAlternate() {
		return
	}

	if t.screen == nil {
		return
	}

	// Check if ptyManager exists (nil for SSH connections)
	if t.ptyManager == nil {
		return
	}

	// Get current screen content
	displayLines := t.screen.GetDisplay()

	// Add new lines to history buffer
	for _, line := range displayLines {
		// Only add non-empty lines to history
		trimmed := strings.TrimSpace(line)
		if trimmed != "" {
			t.ptyManager.historyBuffer = append(t.ptyManager.historyBuffer, line)
		}
	}

	// Maintain history size limit
	if len(t.ptyManager.historyBuffer) > t.ptyManager.maxHistoryLines {
		excess := len(t.ptyManager.historyBuffer) - t.ptyManager.maxHistoryLines
		t.ptyManager.historyBuffer = t.ptyManager.historyBuffer[excess:]
	}

	// Update virtual scrolling limits
	totalLines := len(t.ptyManager.historyBuffer)
	if totalLines > t.ptyManager.viewportHeight {
		t.ptyManager.maxVirtualOffset = totalLines - t.ptyManager.viewportHeight
	} else {
		t.ptyManager.maxVirtualOffset = 0
	}
}

func (t *NativeTerminalWidget) ScrollUpInHistory(lines int) {
	// Don't allow manual scrolling in alternate screen mode
	if t.screen != nil && t.screen.IsUsingAlternate() {
		return
	}

	// Check if ptyManager exists (nil for SSH connections)
	if t.ptyManager == nil {
		return
	}

	t.mutex.Lock()
	defer t.mutex.Unlock()

	if !t.ptyManager.inHistoryMode {
		t.ptyManager.inHistoryMode = true
		t.ptyManager.currentHistoryPos = 0
		log.Printf("Entering history mode")
	}

	newPos := t.ptyManager.currentHistoryPos + lines
	maxPos := len(t.ptyManager.historyBuffer) - t.ptyManager.viewportHeight

	if maxPos < 0 {
		maxPos = 0
	}

	if newPos > maxPos {
		newPos = maxPos
	}

	t.ptyManager.currentHistoryPos = newPos
	t.ptyManager.virtualOffset = newPos

	log.Printf("Scrolled up to history position %d/%d", newPos, maxPos)
	t.updatePending = true
}

func (t *NativeTerminalWidget) ScrollDownInHistory(lines int) {
	// Don't allow manual scrolling in alternate screen mode
	if t.screen != nil && t.screen.IsUsingAlternate() {
		return
	}

	// Check if ptyManager exists (nil for SSH connections)
	if t.ptyManager == nil {
		return
	}

	t.mutex.Lock()
	defer t.mutex.Unlock()

	if !t.ptyManager.inHistoryMode {
		return // Already at bottom
	}

	newPos := t.ptyManager.currentHistoryPos - lines

	if newPos <= 0 {
		// Exit history mode
		t.ptyManager.inHistoryMode = false
		t.ptyManager.currentHistoryPos = 0
		t.ptyManager.virtualOffset = 0
		log.Printf("Exited history mode")
	} else {
		t.ptyManager.currentHistoryPos = newPos
		t.ptyManager.virtualOffset = newPos
		log.Printf("Scrolled down to history position %d", newPos)
	}

	t.updatePending = true
}

func (t *NativeTerminalWidget) WriteToPTY(data []byte) error {
	// Check for write override (SSH connections use this)
	if t.writeOverride != nil {
		t.writeOverride(data)
		return nil
	}

	if t.ptyManager != nil && t.ptyManager.pty != nil {
		n, err := t.ptyManager.pty.Write(data)
		if err != nil {
			log.Printf("PTY write error: %v", err)
			return err
		}
		if n != len(data) {
			log.Printf("PTY write incomplete: wrote %d of %d bytes", n, len(data))
		}
		return nil
	}
	return fmt.Errorf("PTY not initialized")
}

func (t *NativeTerminalWidget) ResizePTY(cols, rows int) error {
	if t.ptyManager == nil || t.ptyManager.pty == nil {
		return fmt.Errorf("PTY not initialized")
	}

	err := t.ptyManager.pty.Resize(cols, rows)
	if err != nil {
		log.Printf("Failed to resize PTY to %dx%d: %v", cols, rows, err)
		return err
	}

	t.ptyManager.viewportHeight = rows
	log.Printf("Resized PTY to %dx%d", cols, rows)

	return nil
}

func (t *NativeTerminalWidget) CloseUnified() {
	log.Printf("Closing unified PTY")

	if t.ptyManager != nil && t.ptyManager.pty != nil {
		err := t.ptyManager.pty.Close()
		if err != nil {
			log.Printf("Error closing PTY: %v", err)
		}
		t.ptyManager.pty = nil
	}

	log.Printf("Unified PTY cleanup completed")
}

// Public API methods
func (t *NativeTerminalWidget) GetHistorySizeUnified() int {
	// No history in alternate screen mode
	if t.screen != nil && t.screen.IsUsingAlternate() {
		return 0
	}

	if t.ptyManager != nil {
		return len(t.ptyManager.historyBuffer)
	}
	return 0
}

func (t *NativeTerminalWidget) IsInHistoryModeUnified() bool {
	// Never in history mode when alternate screen is active
	if t.screen != nil && t.screen.IsUsingAlternate() {
		return false
	}

	if t.ptyManager != nil {
		return t.ptyManager.inHistoryMode
	}
	return false
}

func (t *NativeTerminalWidget) GetHistoryPosition() (int, int) {
	// No history position in alternate screen mode
	if t.screen != nil && t.screen.IsUsingAlternate() {
		return 0, 0
	}

	if t.ptyManager != nil {
		return t.ptyManager.currentHistoryPos, len(t.ptyManager.historyBuffer)
	}
	return 0, 0
}

func (t *NativeTerminalWidget) ScrollToTopUnified() {
	// Don't allow scrolling in alternate screen mode
	if t.screen != nil && t.screen.IsUsingAlternate() {
		return
	}

	if t.ptyManager != nil && len(t.ptyManager.historyBuffer) > 0 {
		maxScroll := len(t.ptyManager.historyBuffer) - t.ptyManager.viewportHeight
		if maxScroll > 0 {
			t.ScrollUpInHistory(maxScroll)
		}
	}
}

func (t *NativeTerminalWidget) ScrollToBottomUnified() {
	// Don't interfere with alternate screen mode
	if t.screen != nil && t.screen.IsUsingAlternate() {
		return
	}

	t.mutex.Lock()
	defer t.mutex.Unlock()

	if t.ptyManager != nil {
		t.ptyManager.inHistoryMode = false
		t.ptyManager.currentHistoryPos = 0
		t.ptyManager.virtualOffset = 0
	}

	if t.screen != nil {
		t.screen.ScrollToBottom()
	}

	t.updatePending = true
}

// Helper method to check alternate screen status
func (t *NativeTerminalWidget) IsInAlternateScreen() bool {
	if t.screen != nil {
		return t.screen.IsUsingAlternate()
	}
	return false
}

// Enhanced escape sequence handler for terminal state management
func (t *NativeTerminalWidget) handleEscapeSequences(data string) {
	// Only process main terminal escape sequences
	// In alternate screen mode, gopyte handles all parsing
	if t.IsInAlternateScreen() {
		return
	}

	// TERMINAL MODE TRANSITIONS
	// Alternate screen mode detection
	if strings.Contains(data, "\x1b[?1049h") || strings.Contains(data, "\x1b[?1047h") || strings.Contains(data, "\x1b[?47h") {
		log.Printf("TERMINAL: Entering alternate screen mode - history disabled")
		t.onEnteringAlternateScreen()
	}
	if strings.Contains(data, "\x1b[?1049l") || strings.Contains(data, "\x1b[?1047l") || strings.Contains(data, "\x1b[?47l") {
		log.Printf("TERMINAL: Exiting alternate screen mode - history re-enabled")
		t.onExitingAlternateScreen()
	}

	// Mouse mode detection
	if strings.Contains(data, "\x1b[?1000h") {
		log.Printf("TERMINAL: Mouse reporting enabled")
	}
	if strings.Contains(data, "\x1b[?1000l") {
		log.Printf("TERMINAL: Mouse reporting disabled")
	}

	// Bracketed paste mode
	if strings.Contains(data, "\x1b[?2004h") {
		log.Printf("TERMINAL: Bracketed paste mode enabled")
	}
	if strings.Contains(data, "\x1b[?2004l") {
		log.Printf("TERMINAL: Bracketed paste mode disabled")
	}

	// APPLICATION STATE MANAGEMENT
	// Window title changes (multiple formats)
	t.handleTitleSequences(data)

	// Icon name changes
	t.handleIconNameSequences(data)

	// Cursor visibility changes
	if strings.Contains(data, "\x1b[?25l") {
		log.Printf("TERMINAL: Cursor hidden")
	}
	if strings.Contains(data, "\x1b[?25h") {
		log.Printf("TERMINAL: Cursor shown")
	}

	// Screen clearing detection (useful for shell prompt detection)
	if strings.Contains(data, "\x1b[2J") || strings.Contains(data, "\x1b[H\x1b[2J") {
		log.Printf("TERMINAL: Screen cleared")
		t.onScreenCleared()
	}

	// Bell/notification
	if strings.Contains(data, "\x07") {
		log.Printf("TERMINAL: Bell received")
		t.onBellReceived()
	}

	// SHELL STATE DETECTION
	// Common shell prompts and command completion
	t.detectShellState(data)

	// Background/foreground process notifications
	if strings.Contains(data, "\x1b]777;notify;") {
		t.handleNotificationSequence(data)
	}

	// Terminal size reports (response to size queries)
	if strings.Contains(data, "\x1b[8;") && strings.Contains(data, "t") {
		t.handleTerminalSizeReport(data)
	}
}

// Handle various title sequence formats
func (t *NativeTerminalWidget) handleTitleSequences(data string) {
	// OSC 0 (icon name and window title)
	if idx := strings.Index(data, "\x1b]0;"); idx >= 0 {
		t.extractTitleFromOSC(data[idx:], 0)
	}

	// OSC 1 (icon name only)
	if idx := strings.Index(data, "\x1b]1;"); idx >= 0 {
		t.extractTitleFromOSC(data[idx:], 1)
	}

	// OSC 2 (window title only)
	if idx := strings.Index(data, "\x1b]2;"); idx >= 0 {
		t.extractTitleFromOSC(data[idx:], 2)
	}
}

func (t *NativeTerminalWidget) extractTitleFromOSC(data string, oscType int) {
	start := strings.IndexRune(data, ';')
	if start < 0 {
		return
	}
	start++ // Skip the semicolon

	// Find terminator (BEL, ST_C0, or ST_C1)
	end := strings.IndexAny(data[start:], "\x07\x1b\x9c")
	if end < 0 {
		return
	}

	// Handle ESC\ terminator
	if data[start+end] == '\x1b' && start+end+1 < len(data) && data[start+end+1] == '\\' {
		end-- // Don't include the ESC
	}

	newTitle := data[start : start+end]
	if newTitle != "" {
		switch oscType {
		case 0, 2: // Window title
			if newTitle != t.title {
				oldTitle := t.title
				t.title = newTitle
				log.Printf("TERMINAL: Window title changed from %q to %q", oldTitle, newTitle)
			}
		case 1: // Icon name
			log.Printf("TERMINAL: Icon name set to %q", newTitle)
		}
	}
}

// Handle icon name sequences
func (t *NativeTerminalWidget) handleIconNameSequences(data string) {
	// This is typically handled by extractTitleFromOSC, but can be extended
}

// Detect shell state and command patterns
func (t *NativeTerminalWidget) detectShellState(data string) {
	// Common shell prompt indicators
	if strings.Contains(data, "$ ") || strings.Contains(data, "# ") ||
		strings.Contains(data, "> ") || strings.Contains(data, "% ") {
		// Likely shell prompt - could be used for command history parsing
	}

	// Command completion sequences (bash/zsh)
	if strings.Contains(data, "\x1b[?1004h") {
		log.Printf("TERMINAL: Focus reporting enabled (shell with advanced features)")
	}

	// Directory change notifications (some shells)
	if strings.Contains(data, "\x1b]7;file://") {
		t.handleWorkingDirectoryChange(data)
	}
}

// Handle working directory change notifications
func (t *NativeTerminalWidget) handleWorkingDirectoryChange(data string) {
	start := strings.Index(data, "\x1b]7;file://")
	if start < 0 {
		return
	}
	start += len("\x1b]7;file://")

	end := strings.IndexAny(data[start:], "\x07\x1b")
	if end > 0 {
		workingDir := data[start : start+end]
		log.Printf("TERMINAL: Working directory changed to %s", workingDir)
	}
}

// Handle notification sequences
func (t *NativeTerminalWidget) handleNotificationSequence(data string) {
	// Parse notification format: \x1b]777;notify;TITLE;BODY\x07
	parts := strings.Split(data, ";")
	if len(parts) >= 4 {
		title := parts[2]
		body := parts[3]
		body = strings.TrimSuffix(body, "\x07")
		log.Printf("TERMINAL: Notification - Title: %q, Body: %q", title, body)
	}
}

// Handle terminal size report responses
func (t *NativeTerminalWidget) handleTerminalSizeReport(data string) {
	// Format: ESC[8;HEIGHT;WIDTHt
	if strings.HasPrefix(data, "\x1b[8;") && strings.HasSuffix(data, "t") {
		sizeData := data[4 : len(data)-1] // Remove ESC[8; and t
		parts := strings.Split(sizeData, ";")
		if len(parts) == 2 {
			log.Printf("TERMINAL: Size report - Height: %s, Width: %s", parts[0], parts[1])
		}
	}
}

// Event handlers for terminal state changes
func (t *NativeTerminalWidget) onEnteringAlternateScreen() {
	// Application entered alternate screen (vim, less, etc.)
	// Could be used to adjust UI, disable certain features, etc.
}

func (t *NativeTerminalWidget) onExitingAlternateScreen() {
	// Application exited alternate screen
	// Could be used to restore UI state, re-enable features, etc.
}

func (t *NativeTerminalWidget) onScreenCleared() {
	// Screen was cleared - might indicate new command starting
	// Could be used for command history segmentation
}

func (t *NativeTerminalWidget) onBellReceived() {
	// Bell character received - could trigger visual/audio notification
	// In a full terminal, you might flash the window or play a sound
}