package gopyte_test

import (
	"fmt"
	gopyte "github.com/scottpeterman/gopyte/gopyte"
	"strings"
	"testing"
)

func TestHistoryScreenIndex(t *testing.T) {
	// Test that index() properly saves lines to history
	screen := gopyte.NewHistoryScreen(5, 5, 50)
	stream := gopyte.NewStream(screen, false)

	// Fill the screen with line numbers
	for i := 0; i < 5; i++ {
		stream.Feed(fmt.Sprintf("%d", i))
		if i < 4 {
			stream.Feed("\n")
		}
	}

	// Initial state - no history
	if screen.GetHistorySize() != 0 {
		t.Errorf("Initial history size should be 0, got %d", screen.GetHistorySize())
	}

	// Move to last line and trigger index (scroll)
	stream.Feed("\x1b[5;1H") // Move to last line
	stream.Feed("\x1bD")     // Index (IND)

	// Should have saved one line to history
	if screen.GetHistorySize() != 1 {
		t.Errorf("History size after first index: expected 1, got %d", screen.GetHistorySize())
	}

	// Multiple indexes
	for i := 0; i < 5; i++ {
		stream.Feed("\x1bD") // Index
	}

	if screen.GetHistorySize() != 6 {
		t.Errorf("History size after multiple indexes: expected 6, got %d", screen.GetHistorySize())
	}
}

func TestHistoryScreenBasicScrollback(t *testing.T) {
	// Create a small screen for easier testing
	screen := gopyte.NewHistoryScreen(40, 5, 100)
	stream := gopyte.NewStream(screen, false)

	// Fill more than screen height to trigger scrolling
	for i := 1; i <= 10; i++ {
		stream.Feed(fmt.Sprintf("Line %d\n", i))
	}

	// After 10 lines with newlines, we should see lines 7-10 plus an empty line
	// (because the last \n creates a new empty line)
	display := screen.GetDisplay()

	// Check visible lines
	expectedStart := 7       // Adjusted based on actual behavior
	for i := 0; i < 4; i++ { // Check first 4 lines (5th is empty from last \n)
		expected := fmt.Sprintf("Line %d", expectedStart+i)
		if !strings.Contains(display[i], expected) {
			t.Errorf("Line %d: expected to contain %q, got %q", i, expected, display[i])
		}
	}

	// Check history size (should have lines 1-6)
	histSize := screen.GetHistorySize()
	if histSize != 6 {
		t.Errorf("History size: expected 6, got %d", histSize)
	}
}

func TestHistoryScreenPrevNextPage(t *testing.T) {
	screen := gopyte.NewHistoryScreen(4, 4, 40)
	stream := gopyte.NewStream(screen, false)

	// Fill screen with 40 lines (10 screens worth)
	for i := 0; i < 40; i++ {
		stream.Feed(fmt.Sprintf("%d\n", i))
	}

	// After 40 lines with newlines, we're showing lines 37-39 plus empty line
	display := screen.GetDisplay()
	if !strings.Contains(display[0], "37") {
		t.Errorf("Before scroll: expected line 37, got %q", display[0])
	}

	// Test prev_page (scroll up by 2 lines)
	screen.ScrollUp(2)
	display = screen.GetDisplay()

	// Should now show lines 35-38
	if !strings.Contains(display[0], "35") {
		t.Errorf("After scroll up 2: expected line 35, got %q", display[0])
	}

	// Test scroll down by 1
	screen.ScrollDown(1)
	display = screen.GetDisplay()

	// Should show lines 36-39
	if !strings.Contains(display[0], "36") {
		t.Errorf("After scroll down 1: expected line 36, got %q", display[0])
	}

	// Scroll to bottom
	screen.ScrollToBottom()
	display = screen.GetDisplay()

	// Should be back to showing lines 37-39 plus empty
	if !strings.Contains(display[0], "37") {
		t.Errorf("After scroll to bottom: expected line 37, got %q", display[0])
	}
}

func TestHistoryScreenNotEnoughLines(t *testing.T) {
	// Small history buffer that can't hold a full page
	screen := gopyte.NewHistoryScreen(5, 5, 6)
	stream := gopyte.NewStream(screen, false)

	// Add just 6 lines (will fill screen + 1 in history)
	for i := 0; i < 6; i++ {
		stream.Feed(fmt.Sprintf("%d\n", i))
	}

	// Should have at least 1 line in history
	histSize := screen.GetHistorySize()
	if histSize < 1 {
		t.Errorf("Expected at least 1 line in history, got %d", histSize)
	}

	// display := screen.GetDisplay()
	// The screen should show the last 5 lines (including possible empty line from last \n)

	// Try to scroll up to see history
	screen.ScrollUp(3)
	// display = screen.GetDisplay()

	// We should be able to see some earlier content now
	// The exact content depends on implementation, but we should be viewing history
	if !screen.IsViewingHistory() {
		t.Error("Should be viewing history after scroll up")
	}

	// Scroll back down to bottom
	screen.ScrollToBottom()
	if screen.IsViewingHistory() {
		t.Error("Should not be viewing history after scroll to bottom")
	}
}

func TestHistoryScreenDrawExitsHistory(t *testing.T) {
	screen := gopyte.NewHistoryScreen(5, 5, 50)
	stream := gopyte.NewStream(screen, false)

	// Fill with content
	for i := 0; i < 25; i++ {
		stream.Feed(fmt.Sprintf("%d\n", i))
	}

	// Scroll up into history
	screen.ScrollUp(10)
	if !screen.IsViewingHistory() {
		t.Error("Should be viewing history after ScrollUp")
	}

	// Drawing new content should exit history mode
	stream.Feed("NEW")
	if screen.IsViewingHistory() {
		t.Error("Should exit history mode when new content is drawn")
	}

	// Should see the new content
	display := screen.GetDisplay()
	found := false
	for _, line := range display {
		if strings.Contains(line, "NEW") {
			found = true
			break
		}
	}
	if !found {
		t.Error("New content not visible after exiting history mode")
	}
}

func TestHistoryScreenCursorHidden(t *testing.T) {
	screen := gopyte.NewHistoryScreen(5, 5, 50)
	stream := gopyte.NewStream(screen, false)

	// Fill with content
	for i := 0; i < 25; i++ {
		stream.Feed(fmt.Sprintf("%d\n", i))
	}

	// Cursor should be visible initially
	cursor := screen.GetCursorObject()
	if cursor.Hidden {
		t.Error("Cursor should be visible initially")
	}

	// Scroll up - cursor should be hidden
	screen.ScrollUp(5)
	if !cursor.Hidden {
		t.Error("Cursor should be hidden when viewing history")
	}

	// Scroll to bottom - cursor should be visible again
	screen.ScrollToBottom()
	if cursor.Hidden {
		t.Error("Cursor should be visible when at bottom")
	}
}

func TestHistoryScreenEraseInDisplay(t *testing.T) {
	screen := gopyte.NewHistoryScreen(5, 5, 6)
	stream := gopyte.NewStream(screen, false)

	// Add content
	for i := 0; i < 5; i++ {
		stream.Feed(fmt.Sprintf("%d\n", i))
	}

	// Scroll up to view history
	screen.ScrollUp(2)

	// Erase display with mode 3 (clear history)
	stream.Feed("\x1b[3J")

	// History should be cleared
	if screen.GetHistorySize() != 0 {
		t.Errorf("History should be cleared after ESC[3J, got size %d", screen.GetHistorySize())
	}

	// Should have exited history mode
	if screen.IsViewingHistory() {
		t.Error("Should exit history mode after erase")
	}
}

// Benchmark history operations
func BenchmarkHistoryScreenScrolling(b *testing.B) {
	screen := gopyte.NewHistoryScreen(80, 24, 10000)
	stream := gopyte.NewStream(screen, false)

	// Fill with content
	for i := 0; i < 1000; i++ {
		stream.Feed(fmt.Sprintf("Line %d with some content to make it realistic\n", i))
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		// Scroll up and down
		screen.ScrollUp(100)
		screen.ScrollDown(50)
		screen.ScrollUp(25)
		screen.ScrollToBottom()
	}
}

func BenchmarkHistoryScreenLargeContent(b *testing.B) {
	for i := 0; i < b.N; i++ {
		screen := gopyte.NewHistoryScreen(80, 24, 10000)
		stream := gopyte.NewStream(screen, false)

		// Simulate a large amount of output
		for j := 0; j < 1000; j++ {
			stream.Feed(fmt.Sprintf("Line %d: Lorem ipsum dolor sit amet, consectetur adipiscing elit.\n", j))
		}
	}
}
