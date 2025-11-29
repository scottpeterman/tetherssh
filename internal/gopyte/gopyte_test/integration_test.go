package gopyte_test

import (
	gopyte "github.com/scottpeterman/gopyte/gopyte"
	"github.com/stretchr/testify/assert"
	"os"
	_ "path/filepath"
	"testing"
)

func TestPyteFixtures(t *testing.T) {
	// Test with actual pyte test fixtures
	testCases := []struct {
		name      string
		inputFile string
	}{
		{"ls command", "tests/captured/ls.input"},
		{"cat GPL3", "tests/captured/cat-gpl3.input"},
		// Start with simpler tests first
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Read the input file
			input, err := os.ReadFile(tc.inputFile)
			if err != nil {
				t.Skipf("Could not read test file %s: %v", tc.inputFile, err)
				return
			}

			// Create screen and stream
			screen, err := gopyte.NewPythonScreen(80, 24)
			assert.NoError(t, err)
			defer screen.Close()

			stream := gopyte.NewStream(screen, false)

			// Feed the entire input
			stream.Feed(string(input))

			// If we get here without crashing, it's a success!
			t.Logf("Successfully processed %d bytes from %s", len(input), tc.inputFile)
		})
	}
}

func TestBasicVT100Sequences(t *testing.T) {
	screen, err := gopyte.NewPythonScreen(80, 24)
	assert.NoError(t, err)
	defer screen.Close()

	stream := gopyte.NewStream(screen, false)

	// Test a variety of VT100/ANSI sequences
	sequences := []string{
		"\x1b[2J",          // Clear screen
		"\x1b[H",           // Home
		"\x1b[10;20H",      // Position
		"\x1b[1m",          // Bold
		"\x1b[31m",         // Red foreground
		"\x1b[44m",         // Blue background
		"\x1b[0m",          // Reset
		"\x1b[K",           // Erase to end of line
		"\x1b[1K",          // Erase to beginning of line
		"\x1b[2K",          // Erase entire line
		"\x1b[3A",          // Cursor up 3
		"\x1b[2B",          // Cursor down 2
		"\x1b[4C",          // Cursor forward 4
		"\x1b[5D",          // Cursor back 5
		"\x1b7",            // Save cursor
		"\x1b8",            // Restore cursor
		"\x1b[?25h",        // Show cursor
		"\x1b[?25l",        // Hide cursor
		"\x1b]0;Title\x07", // Set title
		"\x1b[1;24r",       // Set scroll region
	}

	for _, seq := range sequences {
		stream.Feed(seq)
		// Just verify no crash
	}

	// Test some real text with escapes
	stream.Feed("\x1b[31mRed \x1b[32mGreen \x1b[34mBlue\x1b[0m Normal\r\n")
	stream.Feed("\x1b[1mBold \x1b[4mUnderline \x1b[7mReverse\x1b[0m\r\n")
}
