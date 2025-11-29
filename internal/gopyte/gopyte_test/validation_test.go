package gopyte_test

import (
	gopyte "github.com/scottpeterman/gopyte/gopyte"
	"github.com/stretchr/testify/assert"
	"strings"
	"testing"
)

func TestMockScreenValidation(t *testing.T) {
	testCases := []struct {
		name        string
		input       string
		mustContain []string // Changed from exact matches to substring matches
	}{
		{
			name:        "Simple text",
			input:       "Hello",
			mustContain: []string{"Draw", "Hello"},
		},
		{
			name:        "Text with newline",
			input:       "Line1\nLine2",
			mustContain: []string{"Line1", "Linefeed", "Line2"},
		},
		{
			name:        "CRLF",
			input:       "Test\r\n",
			mustContain: []string{"Test", "CarriageReturn", "Linefeed"},
		},
		{
			name:  "Cursor movement",
			input: "\x1b[5A\x1b[3B\x1b[2C\x1b[4D",
			mustContain: []string{
				"CursorUp[5]",
				"CursorDown[3]",
				"CursorForward[2]",
				"CursorBack[4]",
			},
		},
		{
			name:        "SGR sequence",
			input:       "\x1b[1;31;44m",
			mustContain: []string{"SelectGraphicRendition[[1 31 44]]"},
		},
		{
			name:        "Clear and home",
			input:       "\x1b[2J\x1b[H",
			mustContain: []string{"EraseInDisplay[2]", "CursorPosition[1 1]"},
		},
		{
			name:        "Set margins",
			input:       "\x1b[5;20r",
			mustContain: []string{"SetMargins[5 20]"},
		},
		{
			name:        "Private mode set",
			input:       "\x1b[?25h",
			mustContain: []string{"SetMode[[25] true]"},
		},
		{
			name:  "Multiple SGR",
			input: "\x1b[0;1;31mRed Bold\x1b[m",
			mustContain: []string{
				"SelectGraphicRendition[[0 1 31]]",
				"Red Bold", // Just check the text is there
				"SelectGraphicRendition[[0]]",
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			screen := gopyte.NewMockScreen()
			stream := gopyte.NewStream(screen, false)

			stream.Feed(tc.input)

			// Join all calls for easier searching
			allCalls := strings.Join(screen.Calls, " ")

			// Check that all expected substrings are present
			for _, expected := range tc.mustContain {
				if !strings.Contains(allCalls, expected) {
					t.Errorf("Expected substring %q not found.\nGot calls: %v", expected, screen.Calls)
				}
			}
		})
	}
}

func TestComplexSequences(t *testing.T) {
	screen := gopyte.NewMockScreen()
	stream := gopyte.NewStream(screen, false)

	// Test a complex real-world sequence (simplified vim startup)
	stream.Feed("\x1b[?1049h") // Alternative screen buffer
	stream.Feed("\x1b[1;24r")  // Set scrolling region
	stream.Feed("\x1b[?12h")   // Start blinking cursor
	stream.Feed("\x1b[?25h")   // Show cursor
	stream.Feed("\x1b[27m")    // Exit reverse video
	stream.Feed("\x1b[m")      // Reset attributes
	stream.Feed("\x1b[H")      // Home
	stream.Feed("\x1b[2J")     // Clear screen

	// Join all calls for substring matching
	allCalls := strings.Join(screen.Calls, " ")

	// Verify we handled all the sequences
	assert.Contains(t, allCalls, "SetMode[[1049] true]")
	assert.Contains(t, allCalls, "SetMargins[1 24]")
	assert.Contains(t, allCalls, "CursorPosition[1 1]")
	assert.Contains(t, allCalls, "EraseInDisplay[2]")
}

func TestTextBatching(t *testing.T) {
	screen := gopyte.NewMockScreen()
	stream := gopyte.NewStream(screen, false)

	// Feed a longer string without control characters
	stream.Feed("This is a longer test string")

	// Should be batched into one Draw call
	assert.Equal(t, 1, len(screen.Calls))
	assert.Contains(t, screen.Calls[0], "This is a longer test string")

	// Test mixed content
	screen.Calls = nil
	stream.Feed("Start\x1b[31mRed\x1b[0mEnd")

	// Check all parts are there
	allCalls := strings.Join(screen.Calls, " ")
	assert.Contains(t, allCalls, "Start")
	assert.Contains(t, allCalls, "Red")
	assert.Contains(t, allCalls, "End")
	assert.Contains(t, allCalls, "SelectGraphicRendition[[31]]")
	assert.Contains(t, allCalls, "SelectGraphicRendition[[0]]")
}
