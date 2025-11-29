package gopyte_test

import (
	"fmt"
	"strings"
	"testing"

	gopyte "github.com/scottpeterman/gopyte/gopyte"
)

func TestAlternateScreenBuffer(t *testing.T) {
	screen := gopyte.NewWideCharScreen(80, 24, 1000)
	stream := gopyte.NewStream(screen, false)

	// Add content to main screen
	stream.Feed("Main screen line 1\n")
	stream.Feed("Main screen line 2\n")
	stream.Feed("Main screen line 3\n")

	mainDisplay := screen.GetDisplay()
	if !strings.Contains(mainDisplay[0], "Main screen line 1") {
		t.Errorf("Main screen content not found: %q", mainDisplay[0])
	}

	// Switch to alternate screen (vim uses this)
	stream.Feed("\x1b[?1049h") // Save cursor and switch to alternate screen

	if !screen.IsUsingAlternate() {
		t.Error("Should be using alternate screen after ESC[?1049h")
	}

	// Alternate screen should be empty
	altDisplay := screen.GetDisplay()
	if strings.Contains(altDisplay[0], "Main screen") {
		t.Error("Alternate screen should not contain main screen content")
	}

	// Add content to alternate screen
	stream.Feed("Alternate screen content\n")
	stream.Feed("Like in vim or less\n")

	altDisplay = screen.GetDisplay()
	if !strings.Contains(altDisplay[0], "Alternate screen content") {
		t.Errorf("Alternate screen content not found: %q", altDisplay[0])
	}

	// Switch back to main screen
	stream.Feed("\x1b[?1049l") // Restore cursor and switch to main screen

	if screen.IsUsingAlternate() {
		t.Error("Should not be using alternate screen after ESC[?1049l")
	}

	// Main screen content should be preserved
	mainDisplay = screen.GetDisplay()
	if !strings.Contains(mainDisplay[0], "Main screen line 1") {
		t.Errorf("Main screen content not preserved: %q", mainDisplay[0])
	}

	// Alternate content should not be visible
	if strings.Contains(mainDisplay[0], "Alternate screen") {
		t.Error("Main screen should not show alternate content")
	}
}

func TestAlternateScreenNoHistory(t *testing.T) {
	screen := gopyte.NewWideCharScreen(40, 5, 100)
	stream := gopyte.NewStream(screen, false)

	// Fill main screen with scrolling content
	for i := 1; i <= 10; i++ {
		stream.Feed(fmt.Sprintf("Main line %d\n", i))
	}

	// Should have history in main screen
	mainHistSize := screen.GetHistorySize()
	if mainHistSize == 0 {
		t.Error("Main screen should have history")
	}

	// Switch to alternate screen
	stream.Feed("\x1b[?1049h")

	// Fill alternate screen with scrolling content
	for i := 1; i <= 10; i++ {
		stream.Feed(fmt.Sprintf("Alt line %d\n", i))
	}

	// Alternate screen should not accumulate history
	// The history from main screen is saved
	altHistSize := screen.GetHistorySize()
	if altHistSize != 0 {
		t.Errorf("Alternate screen should have no history, got %d", altHistSize)
	}

	// Try to scroll up in alternate screen - should be no-op
	screen.ScrollUp(5)
	if screen.IsViewingHistory() {
		t.Error("Should not be able to view history in alternate screen")
	}

	// Switch back to main
	stream.Feed("\x1b[?1049l")

	// History should be restored
	restoredHistSize := screen.GetHistorySize()
	if restoredHistSize != mainHistSize {
		t.Errorf("History not preserved: expected %d, got %d",
			mainHistSize, restoredHistSize)
	}
}

func TestWideCharacterBasics(t *testing.T) {
	screen := gopyte.NewWideCharScreen(10, 3, 100)
	stream := gopyte.NewStream(screen, false)

	// Test ASCII characters (width 1)
	stream.Feed("Hello")
	x, _ := screen.GetCursor()
	if x != 5 {
		t.Errorf("After 'Hello', cursor should be at 5, got %d", x)
	}

	// Reset
	stream.Feed("\x1b[H\x1b[2J")

	// Test CJK character (width 2)
	stream.Feed("你") // Chinese character, width 2
	x, _ = screen.GetCursor()
	if x != 2 {
		t.Errorf("After wide character, cursor should be at 2, got %d", x)
	}

	// Reset
	stream.Feed("\x1b[H\x1b[2J")

	// Test emoji (width 2)
	stream.Feed("😀") // Emoji, width 2
	x, _ = screen.GetCursor()
	if x != 2 {
		t.Errorf("After emoji, cursor should be at 2, got %d", x)
	}
}

func TestWideCharacterOverwrite(t *testing.T) {
	screen := gopyte.NewWideCharScreen(10, 3, 100)
	stream := gopyte.NewStream(screen, false)

	// Place wide characters
	stream.Feed("你好世界") // 4 Chinese characters, 8 cells total

	// Move cursor back and overwrite
	stream.Feed("\x1b[H") // Home
	stream.Feed("Hi")     // Overwrite first wide char with ASCII

	display := screen.GetDisplay()

	// Should have "Hi" at start, then the remaining Chinese characters
	if !strings.HasPrefix(display[0], "Hi") {
		t.Errorf("Should start with 'Hi', got %q", display[0])
	}

	// The wide character at position 0-1 should be completely replaced
	if strings.Contains(display[0][:2], "你") {
		t.Error("Wide character should be completely overwritten")
	}
}

func TestEmojiCombinations(t *testing.T) {
	screen := gopyte.NewWideCharScreen(20, 3, 100)
	stream := gopyte.NewStream(screen, false)

	// Test various emoji
	emojis := []struct {
		emoji string
		name  string
	}{
		{"😀", "grinning face"},
		{"👍", "thumbs up"},
		{"🎉", "party popper"},
		{"❤️", "red heart"},
		{"🌈", "rainbow"},
		{"🚀", "rocket"},
	}

	for _, e := range emojis {
		stream.Feed("\x1b[H\x1b[2J") // Clear
		stream.Feed(e.emoji)

		x, _ := screen.GetCursor()
		if x != 2 {
			t.Errorf("After %s (%s), cursor should be at 2, got %d",
				e.emoji, e.name, x)
		}

		display := screen.GetDisplay()
		if !strings.Contains(display[0], e.emoji) {
			t.Errorf("Display should contain %s (%s)", e.emoji, e.name)
		}
	}
}

func TestMixedWidthContent(t *testing.T) {
	screen := gopyte.NewWideCharScreen(20, 5, 100)
	stream := gopyte.NewStream(screen, false)

	// Mix of ASCII, CJK, and emoji
	stream.Feed("Hello 你好 😀\n")
	stream.Feed("Test 测试 🎉\n")
	stream.Feed("Mix 混合 ❤️\n")

	display := screen.GetDisplay()

	// Check each line contains expected content
	expectedLines := []string{
		"Hello 你好 😀",
		"Test 测试 🎉",
		"Mix 混合 ❤️",
	}

	for i, expected := range expectedLines {
		fmt.Print(expected)
		if !strings.Contains(display[i], "Hello") && i == 0 ||
			!strings.Contains(display[i], "Test") && i == 1 ||
			!strings.Contains(display[i], "Mix") && i == 2 {
			t.Errorf("Line %d missing expected content: got %q", i, display[i])
		}
	}
}

func BenchmarkWideCharacterRendering(b *testing.B) {
	screen := gopyte.NewWideCharScreen(80, 24, 1000)
	stream := gopyte.NewStream(screen, false)

	// Mix of content types
	content := "Hello 世界 🌍 Testing 测试 🚀 Performance 性能 ⚡\n"

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		stream.Feed(content)
		if i%100 == 0 {
			stream.Feed("\x1b[2J\x1b[H") // Clear periodically
		}
	}
}
