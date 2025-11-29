package gopyte_test

import (
	"fmt"
	"strings"
	"testing"

	"tetherssh/internal/gopyte"
)

// TestVimScrollMargins tests scroll region behavior critical for vim
func TestVimScrollMargins(t *testing.T) {
	screen := gopyte.NewWideCharScreen(80, 24, 1000)
	stream := gopyte.NewStream(screen, false)

	// Fill screen with numbered lines for tracking
	for i := 0; i < 24; i++ {
		line := fmt.Sprintf("Line %02d: This is line number %d", i+1, i+1)
		stream.Feed(line + "\r\n")
	}

	// Verify initial state
	display := screen.GetDisplay()
	if len(display) < 24 {
		t.Fatalf("Expected 24 lines, got %d", len(display))
	}

	// Test 1: Set scroll region (lines 5-20, vim-style for splits)
	stream.Feed("\x1b[5;20r") // DECSTBM: set margins to lines 5-20

	// Cursor should move to top of scroll region
	cursorX, cursorY := screen.GetCursor()
	if cursorX != 0 || cursorY != 4 { // 0-based, so line 5 = index 4
		t.Errorf("After setting scroll region, cursor should be at (0,4), got (%d,%d)", cursorX, cursorY)
	}

	// Test 2: Scroll within region - add content at bottom of scroll region
	stream.Feed("\x1b[20H") // Move to line 20 (bottom of scroll region)
	stream.Feed("NEW LINE AT BOTTOM\r\n")

	display = screen.GetDisplay()

	// Line 5 should have scrolled up, line 20 should have the new content
	// Lines 1-4 and 21-24 should be unchanged
	if !strings.Contains(display[19], "NEW LINE AT BOTTOM") {
		t.Errorf("Expected new content at line 20, got: %s", display[19])
	}

	// Test 3: Reset scroll region and verify full screen scrolling resumes
	stream.Feed("\x1b[r") // Reset margins
	cursorX, cursorY = screen.GetCursor()
	if cursorX != 0 || cursorY != 0 {
		t.Errorf("After reset, cursor should be at (0,0), got (%d,%d)", cursorX, cursorY)
	}
}

// TestVimAlternateScreenTransition tests the alternate screen behavior
func TestVimAlternateScreenTransition(t *testing.T) {
	screen := gopyte.NewWideCharScreen(80, 24, 1000)
	stream := gopyte.NewStream(screen, false)

	// Add some content to main screen and scroll it into history
	for i := 0; i < 50; i++ {
		line := fmt.Sprintf("Main screen line %d", i+1)
		stream.Feed(line + "\r\n")
	}

	historySize := screen.GetHistorySize()

	if historySize == 0 {
		t.Fatal("Expected history to be populated")
	}

	// Enter alternate screen (vim does this)
	stream.Feed("\x1b[?1049h") // DECSET 1049 - alternate screen

	if !screen.IsUsingAlternate() {
		t.Error("Should be using alternate screen")
	}

	// Add content to alternate screen
	for i := 0; i < 10; i++ {
		stream.Feed(fmt.Sprintf("Vim line %d\r\n", i+1))
	}

	altDisplay := screen.GetDisplay()
	if strings.Contains(altDisplay[0], "Main screen") {
		t.Error("Alternate screen should not show main screen content")
	}

	// Exit alternate screen
	stream.Feed("\x1b[?1049l") // DECRST 1049 - exit alternate screen

	if screen.IsUsingAlternate() {
		t.Error("Should have exited alternate screen")
	}

	// Verify main screen is restored
	restoredDisplay := screen.GetDisplay()

	// The display should show the bottom of our original content
	lastLine := restoredDisplay[len(restoredDisplay)-1]
	if !strings.Contains(lastLine, "Main screen line") {
		t.Errorf("Main screen not properly restored. Last line: %s", lastLine)
	}

	// History should still be intact
	if screen.GetHistorySize() != historySize {
		t.Errorf("History size changed: expected %d, got %d", historySize, screen.GetHistorySize())
	}
}

// TestVimScrollingInAlternateScreen tests scrolling behavior within vim
func TestVimScrollingInAlternateScreen(t *testing.T) {
	screen := gopyte.NewWideCharScreen(80, 24, 1000)
	stream := gopyte.NewStream(screen, false)

	// Enter alternate screen
	stream.Feed("\x1b[?1049h")

	// Fill alternate screen with content
	for i := 0; i < 30; i++ {
		stream.Feed(fmt.Sprintf("Vim content line %d\r\n", i+1))
	}

	// In alternate screen mode, manual scrolling should not work
	// because there's no history buffer
	initialDisplay := screen.GetDisplay()

	// Try to scroll up (this should be no-op in alternate screen)
	screen.ScrollUp(5)

	afterScrollDisplay := screen.GetDisplay()

	// Display should be unchanged since alternate screen has no history
	for i, line := range initialDisplay {
		if i < len(afterScrollDisplay) && line != afterScrollDisplay[i] {
			t.Errorf("Alternate screen scrolling should be no-op. Line %d changed from '%s' to '%s'",
				i, line, afterScrollDisplay[i])
		}
	}

	// Exit alternate screen
	stream.Feed("\x1b[?1049l")
}

// TestVimIndexAndReverseIndex tests cursor movement commands that vim uses heavily
func TestVimIndexAndReverseIndex(t *testing.T) {
	screen := gopyte.NewWideCharScreen(80, 24, 1000)
	stream := gopyte.NewStream(screen, false)

	// Set up scroll region for testing
	stream.Feed("\x1b[5;20r") // Scroll region lines 5-20

	// Move to bottom of scroll region
	stream.Feed("\x1b[20H") // Line 20

	// Test IND (Index) - should scroll within region
	stream.Feed("\x1bD") // ESC D - Index

	_, cursorY := screen.GetCursor()
	if cursorY != 19 { // Should stay at line 20 (0-based index 19) after scrolling
		t.Errorf("After IND at scroll region bottom, cursor Y should be 19, got %d", cursorY)
	}

	// Move to top of scroll region
	stream.Feed("\x1b[5H") // Line 5

	// Test RI (Reverse Index) - should reverse scroll within region
	stream.Feed("\x1bM") // ESC M - Reverse Index

	_, cursorY = screen.GetCursor()
	if cursorY != 4 { // Should stay at line 5 (0-based index 4) after reverse scrolling
		t.Errorf("After RI at scroll region top, cursor Y should be 4, got %d", cursorY)
	}
}

// TestVimCursorPositioning tests absolute cursor positioning within scroll regions
func TestVimCursorPositioning(t *testing.T) {
	screen := gopyte.NewWideCharScreen(80, 24, 1000)
	stream := gopyte.NewStream(screen, false)

	// Test positioning without scroll region
	stream.Feed("\x1b[10;20H") // Move to line 10, column 20
	cursorX, cursorY := screen.GetCursor()
	if cursorX != 19 || cursorY != 9 { // 0-based
		t.Errorf("Absolute positioning failed: expected (19,9), got (%d,%d)", cursorX, cursorY)
	}

	// Set scroll region and test origin mode
	stream.Feed("\x1b[5;20r") // Lines 5-20
	stream.Feed("\x1b[?6h")   // DECOM - origin mode (coordinates relative to scroll region)

	stream.Feed("\x1b[1;1H") // Move to "line 1, column 1" in origin mode
	cursorX, cursorY = screen.GetCursor()
	if cursorX != 0 || cursorY != 4 { // Should be at line 5 (0-based index 4)
		t.Errorf("Origin mode positioning failed: expected (0,4), got (%d,%d)", cursorX, cursorY)
	}

	// Reset origin mode
	stream.Feed("\x1b[?6l") // Reset DECOM

	stream.Feed("\x1b[1;1H") // Now should go to absolute line 1
	cursorX, cursorY = screen.GetCursor()
	if cursorX != 0 || cursorY != 0 {
		t.Errorf("After DECOM reset, positioning failed: expected (0,0), got (%d,%d)", cursorX, cursorY)
	}
}

// TestHistoryScrollingEdgeCases tests the specific edge cases in history scrolling
func TestHistoryScrollingEdgeCases(t *testing.T) {
	screen := gopyte.NewWideCharScreen(80, 24, 1000)
	stream := gopyte.NewStream(screen, false)

	// Generate enough content to create substantial history
	for i := 0; i < 100; i++ {
		stream.Feed(fmt.Sprintf("History line %03d\r\n", i+1))
	}

	historySize := screen.GetHistorySize()
	if historySize == 0 {
		t.Fatal("Expected history to be created")
	}

	// Test scrolling to absolute top
	maxScroll := screen.GetMaxHistoryPos()
	screen.ScrollUp(maxScroll)

	if !screen.IsAtTopOfHistory() {
		t.Error("Should be at top of history after scrolling maximum amount")
	}

	// Verify we can see the oldest content
	display := screen.GetDisplay()
	if !strings.Contains(display[0], "History line 001") {
		t.Errorf("At top of history, should see oldest content, got: %s", display[0])
	}

	// Test scrolling to absolute bottom
	screen.ScrollToBottom()

	if !screen.IsAtBottomOfHistory() {
		t.Error("Should be at bottom of history after ScrollToBottom")
	}

	if screen.IsViewingHistory() {
		t.Error("Should not be viewing history when at bottom")
	}

	// Test the edge case: scroll up exactly to fill screen with history
	screen.ScrollUp(24) // One screen worth

	display = screen.GetDisplay()
	// Should show recent history content, not current screen
	found := false
	for _, line := range display {
		if strings.Contains(line, "History line") {
			found = true
			break
		}
	}
	if !found {
		t.Error("After scrolling up 24 lines, should see history content")
	}
}

// TestComplexVimWorkflow tests a realistic vim editing session
func TestComplexVimWorkflow(t *testing.T) {
	screen := gopyte.NewWideCharScreen(80, 24, 1000)
	stream := gopyte.NewStream(screen, false)

	// Simulate initial terminal state with some commands
	stream.Feed("$ vim testfile.txt\r\n")

	// Vim enters alternate screen
	stream.Feed("\x1b[?1049h")
	stream.Feed("\x1b[?1h\x1b=") // Application keypad mode

	// Vim sets up the screen (typical vim initialization)
	stream.Feed("\x1b[H\x1b[2J") // Clear screen, home cursor

	// Vim creates a file with line numbers
	for i := 1; i <= 50; i++ {
		content := fmt.Sprintf("%3d  This is line number %d in the file", i, i)
		stream.Feed(content)
		if i < 50 {
			stream.Feed("\r\n")
		}
	}

	// Set scroll region for vim (reserve bottom line for status)
	stream.Feed("\x1b[1;23r") // Lines 1-23 (leave line 24 for status)

	// Add status line
	stream.Feed("\x1b[24H") // Go to status line
	stream.Feed("-- INSERT -- testfile.txt [Modified]")

	display := screen.GetDisplay()

	// Verify status line
	if !strings.Contains(display[23], "INSERT") {
		t.Errorf("Status line not found: %s", display[23])
	}

	// Test scrolling within vim's scroll region
	stream.Feed("\x1b[23H") // Bottom of text area
	stream.Feed("\r\nNew line added at bottom")

	// The first content line should have scrolled up
	display = screen.GetDisplay()
	if strings.Contains(display[0], "This is line number 1") {
		t.Error("First line should have scrolled up when adding content at bottom of scroll region")
	}

	// Exit vim (back to main screen)
	stream.Feed("\x1b[?1049l")

	// Should return to shell prompt
	if screen.IsUsingAlternate() {
		t.Error("Should have exited alternate screen")
	}

	display = screen.GetDisplay()
	// Should see the original shell command
	found := false
	for _, line := range display {
		if strings.Contains(line, "vim testfile.txt") {
			found = true
			break
		}
	}
	if !found {
		t.Error("Should see original shell command after exiting vim")
	}
}

// BenchmarkHistoryScrolling tests performance of scrolling operations
func BenchmarkHistoryScrolling(b *testing.B) {
	screen := gopyte.NewWideCharScreen(80, 24, 10000)
	stream := gopyte.NewStream(screen, false)

	// Create large history
	for i := 0; i < 5000; i++ {
		stream.Feed(fmt.Sprintf("Line %d with some content for testing\r\n", i))
	}

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		// Scroll up
		screen.ScrollUp(10)
		_ = screen.GetDisplay()

		// Scroll down
		screen.ScrollDown(5)
		_ = screen.GetDisplay()
	}
}
