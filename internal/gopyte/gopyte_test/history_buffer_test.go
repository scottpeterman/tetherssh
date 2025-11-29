package gopyte_test

import (
	"fmt"
	"strings"
	"testing"

	gopyte "github.com/scottpeterman/gopyte/gopyte"
)

// TestHistoryBufferDebug tests the history buffer behavior with correct expectations
func TestHistoryBufferDebug(t *testing.T) {
	t.Log("=== HISTORY BUFFER TESTING ===")

	// Create screen with explicit history size
	screen := gopyte.NewWideCharScreen(80, 24, 1000) // 1000 lines of history
	stream := gopyte.NewStream(screen, false)

	t.Log("Screen configuration:")
	t.Logf("  Columns: 80")
	t.Logf("  Lines: 24")
	t.Logf("  Max History: 1000")

	// Test 1: Basic scrolling with 30 lines
	t.Run("BasicScrolling", func(t *testing.T) {
		screen.Reset()

		// Add exactly 30 lines (6 more than screen height)
		for i := 1; i <= 30; i++ {
			stream.Feed(fmt.Sprintf("Line %d\n", i))
		}

		display := screen.GetDisplay()
		histSize := screen.GetHistorySize()

		t.Logf("After adding 30 lines:")
		t.Logf("  History size: %d", histSize)
		t.Logf("  First visible line: %q", strings.TrimSpace(display[0]))
		t.Logf("  Last visible line: %q", strings.TrimSpace(display[23]))

		// With trailing newlines: 7 lines in history (Lines 1-7)
		// Screen shows Lines 8-30, with Line 30 at position 22
		// Position 23 is empty (cursor position after last \n)
		if histSize != 7 {
			t.Errorf("Expected 7 lines in history, got %d", histSize)
		}

		if !strings.Contains(display[0], "Line 8") {
			t.Errorf("First line should be 'Line 8', got %q", display[0])
		}

		// Line 30 should be at position 22, position 23 should be empty
		if !strings.Contains(display[22], "Line 30") {
			t.Errorf("Line 30 should be at position 22, got %q", display[22])
		}

		if strings.TrimSpace(display[23]) != "" {
			t.Errorf("Last line should be empty (cursor position), got %q", display[23])
		}
	})

	// Test 2: Large scroll with 100 lines
	t.Run("LargeScroll", func(t *testing.T) {
		screen.Reset()

		// Add 100 lines
		for i := 1; i <= 100; i++ {
			stream.Feed(fmt.Sprintf("Line %d\n", i))
		}

		histSize := screen.GetHistorySize()
		display := screen.GetDisplay()

		t.Logf("After adding 100 lines:")
		t.Logf("  History size: %d", histSize)
		t.Logf("  First visible: %q", strings.TrimSpace(display[0]))
		t.Logf("  Last visible: %q", strings.TrimSpace(display[23]))

		// Should have 77 lines in history (100 - 23 visible - cursor line)
		expectedHistory := 77
		if histSize != expectedHistory {
			t.Errorf("Expected %d lines in history, got %d", expectedHistory, histSize)
		}

		// Screen should show lines 78-100, with 100 at position 22
		if !strings.Contains(display[0], "Line 78") {
			t.Errorf("First line should be 'Line 78', got %q", display[0])
		}

		if !strings.Contains(display[22], "Line 100") {
			t.Errorf("Line 100 should be at position 22, got %q", display[22])
		}
	})

	// Test 3: Different line endings
	t.Run("LineEndings", func(t *testing.T) {
		screen.Reset()

		// Test with \r\n (Windows style)
		for i := 1; i <= 30; i++ {
			stream.Feed(fmt.Sprintf("CRLF Line %d\r\n", i))
		}

		histSize := screen.GetHistorySize()
		display := screen.GetDisplay()

		t.Logf("With \\r\\n line endings:")
		t.Logf("  History size: %d", histSize)
		t.Logf("  First visible: %q", strings.TrimSpace(display[0]))

		// Should behave the same as \n
		if histSize != 7 {
			t.Errorf("CRLF: Expected 7 lines in history, got %d", histSize)
		}

		if !strings.Contains(display[0], "CRLF Line 8") {
			t.Errorf("CRLF: Expected 'CRLF Line 8', got %q", display[0])
		}
	})

	// Test 4: Long lines that wrap
	t.Run("LongLines", func(t *testing.T) {
		screen.Reset()

		// Create lines longer than 80 chars (should wrap)
		longLine := strings.Repeat("X", 100) // 100 chars, should wrap

		for i := 1; i <= 20; i++ {
			stream.Feed(fmt.Sprintf("%d: %s\n", i, longLine))
		}

		histSize := screen.GetHistorySize()
		display := screen.GetDisplay()

		t.Logf("With long wrapped lines:")
		t.Logf("  History size: %d", histSize)
		t.Logf("  Display[0]: %q", display[0])

		// Long lines will cause more scrolling
		if histSize == 0 {
			t.Error("No history with wrapped lines - unexpected!")
		}
	})

	// Test 5: History access (scrolling up/down)
	t.Run("HistoryAccess", func(t *testing.T) {
		screen.Reset()

		// Add lines
		for i := 1; i <= 50; i++ {
			stream.Feed(fmt.Sprintf("Access Line %d\n", i))
		}

		histSize := screen.GetHistorySize()
		t.Logf("History size after 50 lines: %d", histSize)

		// Expected: 50 - 23 visible - 1 cursor = 27
		if histSize != 27 {
			t.Errorf("Expected 27 lines in history, got %d", histSize)
		}

		display := screen.GetDisplay()
		t.Logf("Current view shows: %q to %q",
			strings.TrimSpace(display[0]),
			strings.TrimSpace(display[22]))

		// Try scrolling up
		screen.ScrollUp(5)
		if screen.IsViewingHistory() {
			t.Log("✅ Can scroll into history")
			display = screen.GetDisplay()
			t.Logf("After scrolling up 5: %q", strings.TrimSpace(display[0]))
		} else {
			t.Log("❌ Cannot scroll into history")
		}

		// Return to bottom
		screen.ScrollToBottom()
	})

	// Test 6: Alternate screen shouldn't have history
	t.Run("AlternateScreenHistory", func(t *testing.T) {
		screen.Reset()

		// Add content to main screen
		for i := 1; i <= 30; i++ {
			stream.Feed(fmt.Sprintf("Main %d\n", i))
		}

		mainHistSize := screen.GetHistorySize()
		t.Logf("Main screen history: %d", mainHistSize)

		// Switch to alternate screen
		stream.Feed("\x1b[?1049h")

		// Add content to alternate screen
		for i := 1; i <= 30; i++ {
			stream.Feed(fmt.Sprintf("Alt %d\n", i))
		}

		altHistSize := screen.GetHistorySize()
		t.Logf("Alternate screen history: %d", altHistSize)

		if altHistSize != 0 {
			t.Errorf("Alternate screen should have 0 history, got %d", altHistSize)
		}

		// Switch back to main
		stream.Feed("\x1b[?1049l")

		restoredHistSize := screen.GetHistorySize()
		t.Logf("Back to main, history: %d", restoredHistSize)

		if restoredHistSize != mainHistSize {
			t.Errorf("History not preserved: was %d, now %d", mainHistSize, restoredHistSize)
		}
	})

	// Test 7: Vim-like behavior (no trailing newline)
	t.Run("NoTrailingNewline", func(t *testing.T) {
		screen.Reset()

		// Add 30 lines but NO newline after the last one
		for i := 1; i <= 30; i++ {
			if i < 30 {
				stream.Feed(fmt.Sprintf("NoTail %d\n", i))
			} else {
				stream.Feed(fmt.Sprintf("NoTail %d", i)) // No \n
			}
		}

		histSize := screen.GetHistorySize()
		display := screen.GetDisplay()

		t.Logf("Without trailing newline:")
		t.Logf("  History size: %d", histSize)
		t.Logf("  First visible: %q", strings.TrimSpace(display[0]))
		t.Logf("  Last visible: %q", strings.TrimSpace(display[23]))

		// Without trailing newline: 6 history, Lines 7-30 visible
		if histSize != 6 {
			t.Errorf("Expected 6 lines in history (no trailing \\n), got %d", histSize)
		}

		if !strings.Contains(display[0], "NoTail 7") {
			t.Errorf("First line should be 'NoTail 7' (no trailing \\n), got %q", display[0])
		}

		if !strings.Contains(display[23], "NoTail 30") {
			t.Errorf("Last line should be 'NoTail 30' (no trailing \\n), got %q", display[23])
		}
	})

	// Test 8: Production test scenario
	t.Run("ProductionTestScenario", func(t *testing.T) {
		screen.Reset()

		// This is what the production test does
		var seq strings.Builder
		for i := 1; i <= 30; i++ {
			seq.WriteString(fmt.Sprintf("Line %d\r\n", i))
		}

		stream.Feed(seq.String())

		display := screen.GetDisplay()
		histSize := screen.GetHistorySize()

		t.Logf("Production test scenario:")
		t.Logf("  Fed: 30 lines with \\r\\n")
		t.Logf("  History size: %d", histSize)
		t.Logf("  Display[0]: %q", display[0])
		t.Logf("  Display[23]: %q", display[23])

		// Count non-empty lines
		nonEmptyLines := 0
		for i, line := range display {
			if strings.TrimSpace(line) != "" {
				nonEmptyLines++
				if i < 5 || i >= 19 {
					t.Logf("  Display[%d]: %q", i, strings.TrimSpace(line))
				}
			}
		}
		t.Logf("  Non-empty lines on screen: %d", nonEmptyLines)

		// Expected: 7 history, Line 8 first, 23 non-empty lines
		if histSize != 7 {
			t.Errorf("Expected 7 history, got %d", histSize)
		}

		if !strings.Contains(display[0], "Line 8") {
			t.Errorf("Expected first line to be 'Line 8', got %q", display[0])
		}

		if nonEmptyLines != 23 {
			t.Errorf("Expected 23 non-empty lines, got %d", nonEmptyLines)
		}
	})
}
