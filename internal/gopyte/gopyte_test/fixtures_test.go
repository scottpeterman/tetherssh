package gopyte_test

import (
	gopyte "github.com/scottpeterman/gopyte/gopyte"
	// "github.com/stretchr/testify/assert"
	"os"
	"testing"
)

func TestRealTerminalOutput(t *testing.T) {
	// Test with actual captured terminal output
	testFiles := map[string]string{
		"ls":       "../tests/captured/ls.input",
		"cat-gpl3": "../tests/captured/cat-gpl3.input",
		"top":      "../tests/captured/top.input",
		"htop":     "../tests/captured/htop.input",
		"vi":       "../tests/captured/vi.input",
		"mc":       "../tests/captured/mc.input",
	}

	for name, path := range testFiles {
		t.Run(name, func(t *testing.T) {
			data, err := os.ReadFile(path)
			if err != nil {
				t.Skipf("Could not read %s: %v", path, err)
				return
			}

			// Use Python screen for validation
			screen, err := gopyte.NewPythonScreen(80, 24)
			if err != nil {
				t.Skipf("Could not create Python screen: %v", err)
				return
			}
			defer screen.Close()

			stream := gopyte.NewStream(screen, false)

			// Feed the data
			stream.Feed(string(data))

			t.Logf("Successfully parsed %s (%d bytes)", name, len(data))
		})
	}
}

func BenchmarkStreamParsing(b *testing.B) {
	// Benchmark with mock screen to measure pure parsing performance
	screen := gopyte.NewMockScreen()
	stream := gopyte.NewStream(screen, false)

	// Create some test data with mixed content
	testData := "\x1b[2J\x1b[H" + // Clear and home
		"\x1b[31mRed text\x1b[0m Normal text\r\n" +
		"\x1b[1mBold\x1b[0m \x1b[4mUnderline\x1b[0m\r\n" +
		"Plain text line\r\n" +
		"\x1b[10;20HPositioned text" +
		"\x1b[2K" // Clear line

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		screen.Calls = nil // Reset
		stream.Feed(testData)
	}

	b.ReportMetric(float64(len(testData)), "bytes/op")
}
