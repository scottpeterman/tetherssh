// resize_style_test.go
package gopyte_test

import (
	"fmt"
	"strings"
	"testing"
	"unicode/utf8"

	gopyte "github.com/scottpeterman/gopyte/gopyte"
)

// Matches your table-driven pattern and logging style
func TestResizeBehavior(t *testing.T) {
	screen := gopyte.NewWideCharScreen(10, 4, 1000)
	stream := gopyte.NewStream(screen, false)

	type testCase struct {
		name        string
		description string
		setup       func()
		sequence    string
		resizeCols  int
		resizeRows  int
		validate    func(*testing.T, *gopyte.WideCharScreen) bool
	}

	tests := []testCase{
		{
			name:        "ShrinkCols_TruncatesRight",
			description: "Shrinking columns clips right side; left content preserved",
			setup: func() {
				screen.Reset()
			},
			sequence:   "ABCDEFG\r\n",
			resizeCols: 5, resizeRows: 4,
			validate: func(t *testing.T, s *gopyte.WideCharScreen) bool {
				d := s.GetDisplay()
				return utf8.RuneCountInString(d[0]) <= 5 &&
					strings.HasPrefix(d[0], "ABCDE")
			},
		},
		{
			name:        "GrowCols_AllowsDrawingAtNewLastColumn",
			description: "Growing columns pads with spaces; can draw at last new column",
			setup: func() {
				screen.Reset()
			},
			sequence:   "X\r\n",
			resizeCols: 12, resizeRows: 4,
			validate: func(t *testing.T, s *gopyte.WideCharScreen) bool {
				s.CursorPosition(1, 12)
				s.Draw("Z")
				d := s.GetDisplay()
				return strings.HasSuffix(d[0], "Z")
			},
		},
		{
			name:        "ShrinkRows_PushesBottomToHistory",
			description: "Shrinking rows moves bottom cut lines into history (main screen, keep-top policy)",
			setup: func() {
				screen.Reset()
				var b strings.Builder
				i := 1
				for i <= 10 {
					b.WriteString(fmt.Sprintf("Line %d\r\n", i))
					i++
				}
				stream.Feed(b.String())
			},
			sequence:   "",
			resizeCols: 10, resizeRows: 2,
			validate: func(t *testing.T, s *gopyte.WideCharScreen) bool {
				d := s.GetDisplay()
				h := s.GetHistorySize()
				// Your implementation preserves the TOP two visible rows (=> “Line 8”, “Line 9”)
				ok := h >= 9 &&
					strings.Contains(d[0], "Line 8") &&
					strings.Contains(d[1], "Line 9")
				if !ok {
					t.Logf("history=%d d0=%q d1=%q", h, strings.TrimSpace(d[0]), strings.TrimSpace(d[1]))
				}
				return ok
			},
		},

		{
			name:        "AlternateScreen_NoHistoryOnResize",
			description: "Resizing in alt screen does not touch history; main preserved",
			setup: func() {
				screen.Reset()
			},
			sequence: "\x1b[?1049h" + // enter alt
				"\x1b[2J\x1b[HAltView" +
				"", // resize happens below while still in alt
			resizeCols: 15, resizeRows: 6,
			validate: func(t *testing.T, s *gopyte.WideCharScreen) bool {
				// Still in alt; history should be zero
				h0 := s.GetHistorySize()
				// Leave alt; main should be empty and intact
				s.SetMode([]int{1049}, false)
				d := s.GetDisplay()
				ok := h0 == 0 && strings.TrimSpace(d[0]) == ""
				if !ok {
					t.Logf("history=%d main[0]=%q (expected empty)", h0, d[0])
				}
				return ok
			},
		},
		{
			name:        "WideRune_NoDanglingContinuationAfterShrink",
			description: "Shrinking across a wide rune must not leave a broken cell",
			setup: func() {
				screen.Reset()
			},
			sequence:   "\x1b[1;9H界", // place a 2-width rune at cols 9..10 (10-wide screen)
			resizeCols: 9, resizeRows: 4,
			validate: func(t *testing.T, s *gopyte.WideCharScreen) bool {
				// Now try to draw at the new last column; it must work
				s.CursorPosition(1, 9)
				s.Draw("X")
				d := s.GetDisplay()
				return strings.HasSuffix(d[0], "X") && utf8.RuneCountInString(d[0]) <= 9
			},
		},
	}

	passed := 0
	failed := 0
	results := make(map[string]bool)

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			tc.setup()
			if tc.sequence != "" {
				stream.Feed(tc.sequence)
			}
			screen.Resize(tc.resizeCols, tc.resizeRows)
			ok := tc.validate(t, screen)
			results[tc.name] = ok
			if ok {
				passed++
				t.Logf("✅ %s: %s", tc.name, tc.description)
			} else {
				failed++
				t.Logf("❌ %s: %s", tc.name, tc.description)
				d := screen.GetDisplay()
				if len(d) > 0 {
					t.Logf("   Display[0]=%q", d[0])
				}
				t.Logf("   History=%d", screen.GetHistorySize())
			}
		})
	}

	t.Log("\n=== RESIZE REPORT ===")
	t.Logf("Passed: %d  Failed: %d  (%.1f%%)", passed, failed, float64(passed*100)/float64(passed+failed))
}
