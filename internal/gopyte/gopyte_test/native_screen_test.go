package gopyte_test

import (
	gopyte "github.com/scottpeterman/gopyte/gopyte"
	"strings"
	"testing"
)

func TestNativeScreenBasics(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected []string // Expected non-empty lines
		cursorX  int
		cursorY  int
	}{
		{
			name:     "Simple text",
			input:    "Hello World",
			expected: []string{"Hello World"},
			cursorX:  11,
			cursorY:  0,
		},
		{
			name:     "Text with newline",
			input:    "Line 1\nLine 2",
			expected: []string{"Line 1", "Line 2"},
			cursorX:  6, // After "Line 2"
			cursorY:  1, // Second line
		},
		{
			name:     "Carriage return",
			input:    "Hello\rWorld",
			expected: []string{"World"},
			cursorX:  5,
			cursorY:  0,
		},
		{
			name:     "Tab",
			input:    "A\tB\tC",
			expected: []string{"A       B       C"},
			cursorX:  17,
			cursorY:  0,
		},
		{
			name:     "Backspace",
			input:    "Hello\b\b\b",
			expected: []string{"Hello"},
			cursorX:  2,
			cursorY:  0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			screen := gopyte.NewNativeScreen(80, 24)
			stream := gopyte.NewStream(screen, false)

			stream.Feed(tt.input)

			display := screen.GetDisplay()
			x, y := screen.GetCursor()

			// Check cursor position
			if x != tt.cursorX || y != tt.cursorY {
				t.Errorf("Cursor position: got (%d,%d), want (%d,%d)",
					x, y, tt.cursorX, tt.cursorY)
			}

			// Check display content
			nonEmpty := []string{}
			for _, line := range display {
				if line != "" {
					nonEmpty = append(nonEmpty, line)
				}
			}

			if len(nonEmpty) != len(tt.expected) {
				t.Fatalf("Line count: got %d, want %d\nGot: %v\nWant: %v",
					len(nonEmpty), len(tt.expected), nonEmpty, tt.expected)
			}

			for i, line := range tt.expected {
				if nonEmpty[i] != line {
					t.Errorf("Line %d: got %q, want %q", i, nonEmpty[i], line)
				}
			}
		})
	}
}

func TestNativeScreenCursorMovement(t *testing.T) {
	screen := gopyte.NewNativeScreen(80, 24)
	stream := gopyte.NewStream(screen, false)

	// Test cursor positioning
	stream.Feed("\x1b[10;20H") // Move to row 10, col 20
	x, y := screen.GetCursor()
	if x != 19 || y != 9 { // 0-based
		t.Errorf("CursorPosition: got (%d,%d), want (19,9)", x, y)
	}

	// Test cursor up
	stream.Feed("\x1b[3A") // Move up 3
	x, y = screen.GetCursor()
	if y != 6 {
		t.Errorf("CursorUp: got y=%d, want 6", y)
	}

	// Test cursor forward
	stream.Feed("\x1b[5C") // Move right 5
	x, y = screen.GetCursor()
	if x != 24 {
		t.Errorf("CursorForward: got x=%d, want 24", x)
	}

	// Test save/restore
	stream.Feed("\x1b7")     // Save cursor
	stream.Feed("\x1b[1;1H") // Move to home
	stream.Feed("\x1b8")     // Restore cursor
	x, y = screen.GetCursor()
	if x != 24 || y != 6 {
		t.Errorf("Save/Restore: got (%d,%d), want (24,6)", x, y)
	}
}

func TestNativeScreenErasing(t *testing.T) {
	screen := gopyte.NewNativeScreen(80, 24)
	stream := gopyte.NewStream(screen, false)

	// Fill some content
	stream.Feed("Line 1\nLine 2\nLine 3")

	// Clear from cursor to end of screen
	stream.Feed("\x1b[1;3H") // Move to line 1, col 3
	stream.Feed("\x1b[0J")   // Clear from cursor to end

	display := screen.GetDisplay()

	// First line should be "Li" (position 0-1)
	if !strings.HasPrefix(display[0], "Li") || len(strings.TrimSpace(display[0])) != 2 {
		t.Errorf("Line 0 after clear: got %q, want %q", display[0], "Li")
	}

	// Rest should be empty
	for i := 1; i < 3; i++ {
		if strings.TrimSpace(display[i]) != "" {
			t.Errorf("Line %d should be empty, got %q", i, display[i])
		}
	}
}

func TestNativeScreenReset(t *testing.T) {
	screen := gopyte.NewNativeScreen(80, 24)
	stream := gopyte.NewStream(screen, false)

	// Add some content
	stream.Feed("Some text\n\x1b[31mColored\x1b[0m")
	stream.Feed("\x1b[5;10H") // Move cursor

	// Reset
	stream.Feed("\x1bc")

	// Check everything is cleared
	display := screen.GetDisplay()
	for i, line := range display {
		if strings.TrimSpace(line) != "" {
			t.Errorf("Line %d not cleared after reset: %q", i, line)
		}
	}

	// Check cursor is at home
	x, y := screen.GetCursor()
	if x != 0 || y != 0 {
		t.Errorf("Cursor not at home after reset: (%d,%d)", x, y)
	}
}

func TestNativeScreenWithRealSequences(t *testing.T) {
	screen := gopyte.NewNativeScreen(80, 24)
	stream := gopyte.NewStream(screen, false)

	// A more realistic sequence
	stream.Feed("\x1b[2J") // Clear screen
	stream.Feed("\x1b[H")  // Home
	stream.Feed("Terminal Test\r\n")
	stream.Feed("\x1b[32m") // Green (stub for now)
	stream.Feed("Green text\x1b[0m\r\n")
	stream.Feed("\x1b[3;1H") // Position
	stream.Feed("Line 3")

	display := screen.GetDisplay()

	// Verify content
	if display[0] != "Terminal Test" {
		t.Errorf("Line 0: got %q, want %q", display[0], "Terminal Test")
	}

	if !strings.Contains(display[1], "Green text") {
		t.Errorf("Line 1: got %q, want to contain %q", display[1], "Green text")
	}

	if display[2] != "Line 3" {
		t.Errorf("Line 2: got %q, want %q", display[2], "Line 3")
	}
}

// Test that our native screen satisfies the Screen interface
func TestNativeScreenInterface(t *testing.T) {
	var _ gopyte.Screen = (*gopyte.NativeScreen)(nil)
}

// Benchmark native screen performance
func BenchmarkNativeScreen(b *testing.B) {
	screen := gopyte.NewNativeScreen(80, 24)
	stream := gopyte.NewStream(screen, false)

	// Typical terminal sequence
	sequence := "\x1b[2J\x1b[HHello World\r\nLine 2\x1b[3;1HLine 3\x1b[K"

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		stream.Feed(sequence)
		screen.Reset()
	}
}
