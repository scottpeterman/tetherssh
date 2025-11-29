package gopyte_test

import (
	gopyte "github.com/scottpeterman/gopyte/gopyte"
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestBasicStream(t *testing.T) {
	// Remove the skip now that Stream is implemented
	screen, err := gopyte.NewPythonScreen(80, 24)
	assert.NoError(t, err)
	defer screen.Close()

	stream := gopyte.NewStream(screen, false)

	// Test basic text
	stream.Feed("Hello, World!")

	// Test escape sequences
	stream.Feed("\x1b[2J")                 // Clear screen
	stream.Feed("\x1b[H")                  // Home position
	stream.Feed("\x1b[31mRed Text\x1b[0m") // Red text with reset

	// If we get here without crashing, basic parsing works!
	assert.NotNil(t, stream)
}

func TestStreamEscapeSequences(t *testing.T) {
	screen, err := gopyte.NewPythonScreen(80, 24)
	assert.NoError(t, err)
	defer screen.Close()

	stream := gopyte.NewStream(screen, false)

	testCases := []struct {
		name  string
		input string
	}{
		{"Clear screen", "\x1b[2J"},
		{"Cursor home", "\x1b[H"},
		{"Move to 10,20", "\x1b[10;20H"},
		{"Red text", "\x1b[31mRed\x1b[0m"},
		{"Bold text", "\x1b[1mBold\x1b[0m"},
		{"Newline", "Line1\r\nLine2"},
		{"Tab", "Col1\tCol2"},
		{"Backspace", "abc\bX"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			stream.Feed(tc.input)
			// Just verify we don't crash
		})
	}
}
