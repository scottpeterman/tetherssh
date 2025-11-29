package main

import (
	"fmt"
	"image/color"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/driver/desktop"
	"fyne.io/fyne/v2/widget"
)

type SelectionManager struct {
	terminal *NativeTerminalWidget

	// Selection positions (matching your existing fields)
	startPos fyne.Position
	endPos   fyne.Position

	// Selection state
	isSelecting  bool
	hasSelection bool

	// Cached selection text
	selectedText string
}

func NewSelectionManager(terminal *NativeTerminalWidget) *SelectionManager {
	return &SelectionManager{
		terminal: terminal,
	}
}

func (sm *SelectionManager) HandleMouseDown(event *desktop.MouseEvent) bool {
	if event.Button != desktop.MouseButtonPrimary {
		return false
	}

	sm.Clear()
	sm.startPos = event.Position
	sm.endPos = event.Position
	sm.isSelecting = true
	sm.hasSelection = false

	fmt.Printf("Selection started at %.1f,%.1f\n", event.Position.X, event.Position.Y)
	sm.terminal.updatePending = true
	return true
}

func (sm *SelectionManager) HandleMouseUp(event *desktop.MouseEvent) bool {
	if event.Button != desktop.MouseButtonPrimary || !sm.isSelecting {
		return false
	}

	sm.isSelecting = false

	// Check if we selected something (must have moved the mouse)
	if sm.startPos.X != sm.endPos.X || sm.startPos.Y != sm.endPos.Y {
		sm.hasSelection = true

		// Debug output
		startCol, startRow := sm.positionToCell(sm.startPos)
		endCol, endRow := sm.positionToCell(sm.endPos)
		fmt.Printf("Selection completed: (%d,%d) to (%d,%d)\n",
			startCol, startRow, endCol, endRow)

		// Get and display selected text
		text := sm.GetSelectedText()
		fmt.Printf("Selected text: %q\n", text)

		// Copy to clipboard
		if text != "" {
			sm.CopyToClipboard()
		}

		// Keep selection visible after mouse up
		sm.terminal.updatePending = true
	} else {
		// Just a click, clear any existing selection
		sm.Clear()
	}

	return true
}

func (sm *SelectionManager) HandleDrag(pos fyne.Position) bool {
	if !sm.isSelecting {
		return false
	}

	sm.endPos = pos
	sm.hasSelection = true
	sm.terminal.updatePending = true
	return true
}

func (sm *SelectionManager) GetSelectedText() string {
	if !sm.hasSelection && !sm.isSelecting {
		return ""
	}

	// Convert positions to cell coordinates
	startCol, startRow := sm.positionToCell(sm.startPos)
	endCol, endRow := sm.positionToCell(sm.endPos)

	// Normalize (ensure start is before end)
	if startRow > endRow || (startRow == endRow && startCol > endCol) {
		startRow, endRow = endRow, startRow
		startCol, endCol = endCol, startCol
	}

	allLines := sm.terminal.screen.GetDisplay()
	viewport := sm.terminal.calculateUnifiedViewport(allLines)

	// Adjust for viewport offset
	actualStartRow := viewport.scrollOffset + startRow
	actualEndRow := viewport.scrollOffset + endRow

	var selectedText strings.Builder

	if actualStartRow == actualEndRow {
		// Single line
		if actualStartRow < len(allLines) {
			line := allLines[actualStartRow]
			runes := []rune(line)
			if startCol < len(runes) {
				endIdx := endCol
				if endIdx > len(runes) {
					endIdx = len(runes)
				}
				text := string(runes[startCol:endIdx])
				selectedText.WriteString(strings.TrimRight(text, " "))
			}
		}
	} else {
		// Multi-line
		for row := actualStartRow; row <= actualEndRow && row < len(allLines); row++ {
			line := allLines[row]
			runes := []rune(line)

			if row == actualStartRow {
				if startCol < len(runes) {
					selectedText.WriteString(strings.TrimRight(string(runes[startCol:]), " "))
				}
				selectedText.WriteString("\r\n")
			} else if row == actualEndRow {
				if endCol > 0 && len(runes) > 0 {
					endIdx := endCol
					if endIdx > len(runes) {
						endIdx = len(runes)
					}
					selectedText.WriteString(strings.TrimRight(string(runes[:endIdx]), " "))
				}
			} else {
				selectedText.WriteString(strings.TrimRight(line, " "))
				selectedText.WriteString("\r\n")
			}
		}
	}

	return selectedText.String()
}

func (sm *SelectionManager) CopyToClipboard() {
	text := sm.GetSelectedText()
	if text == "" {
		return
	}

	sm.selectedText = text

	window := fyne.CurrentApp().Driver().AllWindows()[0]
	clipboard := window.Clipboard()
	clipboard.SetContent(text)

	fmt.Printf("Copied %d chars to clipboard\n", len(text))
}

func (sm *SelectionManager) Clear() {
	sm.hasSelection = false
	sm.isSelecting = false
	sm.selectedText = ""
	sm.terminal.updatePending = true
}

func (sm *SelectionManager) HasSelection() bool {
	return sm.hasSelection
}

func (sm *SelectionManager) IsSelecting() bool {
	return sm.isSelecting
}

// Helper to convert position to cell coordinates
func (sm *SelectionManager) positionToCell(pos fyne.Position) (col, row int) {
	col = int(pos.X / sm.terminal.charWidth)
	row = int(pos.Y / sm.terminal.charHeight)

	if col < 0 {
		col = 0
	}
	if col >= sm.terminal.cols {
		col = sm.terminal.cols - 1
	}
	if row < 0 {
		row = 0
	}
	if row >= sm.terminal.rows {
		row = sm.terminal.rows - 1
	}

	return col, row
}
func (sm *SelectionManager) ApplyHighlight(rows []widget.TextGridRow, viewport VirtualScrollState) {
	if !sm.hasSelection && !sm.isSelecting {
		fmt.Printf("ApplyHighlight: No selection active\n")
		return
	}

	startCol, startRow := sm.positionToCell(sm.startPos)
	endCol, endRow := sm.positionToCell(sm.endPos)
	fmt.Printf("ApplyHighlight: startPos=(%.1f,%.1f) endPos=(%.1f,%.1f)\n",
		sm.startPos.X, sm.startPos.Y, sm.endPos.X, sm.endPos.Y)
	fmt.Printf("ApplyHighlight: cells from (%d,%d) to (%d,%d)\n",
		startCol, startRow, endCol, endRow)

	// If start and end are the same, nothing to highlight
	if startCol == endCol && startRow == endRow {
		fmt.Printf("ApplyHighlight: Start and end are the same - no area to highlight\n")
		return
	}
	// Normalize
	if startRow > endRow || (startRow == endRow && startCol > endCol) {
		startRow, endRow = endRow, startRow
		startCol, endCol = endCol, startCol
	}

	for rowIdx := 0; rowIdx < len(rows) && rowIdx < viewport.visibleLines; rowIdx++ {
		if rowIdx < startRow || rowIdx > endRow {
			continue
		}

		// Access row directly (not as pointer)
		row := &rows[rowIdx]

		for colIdx := 0; colIdx < len(row.Cells); colIdx++ {
			shouldHighlight := false

			if rowIdx == startRow && rowIdx == endRow {
				shouldHighlight = colIdx >= startCol && colIdx < endCol
			} else if rowIdx == startRow {
				shouldHighlight = colIdx >= startCol
			} else if rowIdx == endRow {
				shouldHighlight = colIdx < endCol
			} else {
				shouldHighlight = true
			}

			if shouldHighlight {
				if row.Cells[colIdx].Style == nil {
					row.Cells[colIdx].Style = &widget.CustomTextGridStyle{}
				}
				style := row.Cells[colIdx].Style.(*widget.CustomTextGridStyle)
				style.BGColor = color.RGBA{0x0C, 0x7A, 0xCC, 0xFF}
				style.FGColor = color.White
			}
		}
	}
}
