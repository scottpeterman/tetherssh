package main

import (
	"context"
	"fmt"
	"image/color"
	"log"
	"runtime"
	"strings"
	"sync"
	"time"

	"tetherssh/internal/gopyte"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/driver/desktop"
	"fyne.io/fyne/v2/widget"
)

var (
	_ fyne.Focusable    = (*NativeTerminalWidget)(nil)
	_ fyne.Tappable     = (*NativeTerminalWidget)(nil)
	_ fyne.Draggable    = (*NativeTerminalWidget)(nil)
	_ desktop.Hoverable = (*NativeTerminalWidget)(nil)
	_ fyne.Shortcutable = (*NativeTerminalWidget)(nil)
	// _ desktop.Keyable   = (*NativeTerminalWidget)(nil)
)

// NativeTerminalWidget - Enhanced with cross-platform PTY and history support
type NativeTerminalWidget struct {
	widget.BaseWidget
	fyne.ShortcutHandler // ADD: Embed the shortcut handler for control keys
	lineSelection        struct {
		startLine int
		endLine   int
		active    bool
	}
	// Core components - Using WideCharScreen with enhanced history
	screen    *gopyte.WideCharScreen
	stream    *gopyte.Stream
	textGrid  *widget.TextGrid
	scroll    *HybridScrollContainer
	selection *SelectionManager

	// UNIFIED PTY MANAGEMENT - Works on Windows and Unix
	ptyManager *PTYManager

	// State management
	title string

	// Font and sizing
	fontSize   float32
	charWidth  float32
	charHeight float32
	cols       int
	rows       int

	// Enhanced virtual scrolling state
	virtualScroll VirtualScrollState

	// Thread safety and performance
	mutex         sync.RWMutex
	updateChannel chan []byte
	updatePending bool

	// Context for cancellation
	ctx    context.Context
	cancel context.CancelFunc

	// Size change detection
	lastWidth   float32
	lastHeight  float32
	resizeTimer *time.Timer
	resizeMutex sync.Mutex

	// Selection support
	selectionStart fyne.Position
	selectionEnd   fyne.Position
	isSelecting    bool

	// Theme
	theme *NativeTheme

	// Default colors
	fgColor string
	bgColor string

	// Performance optimization
	cachedLines []string

	// Event handling state
	hasFocus       bool
	debugEvents    bool
	lastScrollTime time.Time

	// Cross-platform compatibility flags
	isWindows bool
	isUnix    bool

	// SSH write override - routes output to SSH instead of local PTY
	writeOverride func([]byte)

	// Resize callback - allows SSH sessions to receive resize events
	onResizeCallback func(cols, rows int)
}

// NewNativeTerminalWidget creates a new cross-platform terminal with history support
func NewNativeTerminalWidget(darkMode bool) *NativeTerminalWidget {
	ctx, cancel := context.WithCancel(context.Background())

	t := &NativeTerminalWidget{
		updateChannel: make(chan []byte, 1000),
		ctx:           ctx,
		cancel:        cancel,
		fontSize:      13.0,
		cols:          80,
		rows:          24,
		title:         "Terminal",
		theme:         NewNativeTheme(darkMode),
		fgColor:       "white",
		bgColor:       "black",
		cachedLines:   make([]string, 0, 150),

		// Platform detection
		isWindows: runtime.GOOS == "windows",
		isUnix:    runtime.GOOS != "windows",

		// Event handling
		hasFocus:       false,
		debugEvents:    true,
		lastScrollTime: time.Now(),

		// Initialize virtual scroll state
		virtualScroll: VirtualScrollState{
			visibleLines: 24,
		},
	}

	t.selection = NewSelectionManager(t)

	t.calculateCharDimensions()

	// Enhanced WideCharScreen with more history for better scrolling
	historyLines := 1000
	if runtime.GOOS == "windows" {
		// ConPTY can handle more history efficiently
		historyLines = 2000
	}

	log.Printf("Creating WideCharScreen with enhanced history support (%d lines)", historyLines)
	t.screen = gopyte.NewWideCharScreen(t.cols, t.rows, historyLines)
	t.stream = gopyte.NewStream(t.screen, false)

	// Create TextGrid
	t.textGrid = widget.NewTextGrid()
	t.textGrid.ShowLineNumbers = false
	t.textGrid.ShowWhitespace = false

	// Initialize TextGrid size
	t.initializeTextGridSize()

	// Start background processing
	go t.dataProcessor()
	go t.updateProcessor()

	t.ExtendBaseWidget(t)
	log.Printf("NewNativeTerminalWidget: Created %s terminal widget", runtime.GOOS)
	return t
}

// ENHANCED DATA PROCESSOR - Handles both ConPTY and PTY data
func (t *NativeTerminalWidget) dataProcessor() {
	log.Printf("Unified data processor started for %s", runtime.GOOS)

	for {
		select {
		case data := <-t.updateChannel:
			// Use unified processing method
			t.processTerminalDataUnified(data)
		case <-t.ctx.Done():
			log.Printf("Unified data processor stopping")
			return
		}
	}
}

// INTERFACE IMPLEMENTATIONS - Enhanced with unified system
func (t *NativeTerminalWidget) Focusable() bool {
	return true
}

func (t *NativeTerminalWidget) FocusLost() {
	fmt.Printf("FocusLost: Unified terminal widget lost focus (%s)\n", runtime.GOOS)
	t.hasFocus = false
}

func (t *NativeTerminalWidget) TypedShortcut(shortcut fyne.Shortcut) {
	fmt.Printf("TypedShortcut received: %T\n", shortcut)

	if !t.isPTYAvailable() {
		fmt.Printf("TypedShortcut: No PTY available, ignoring\n")
		return
	}

	// Handle desktop custom shortcuts (Ctrl+key and Alt+key combinations)
	if customShortcut, ok := shortcut.(*desktop.CustomShortcut); ok {
		fmt.Printf("Custom shortcut detected: Key=%s, Modifier=%d\n",
			customShortcut.KeyName, customShortcut.Modifier)
		// In TypedShortcut method:
		if customShortcut.Modifier&fyne.KeyModifierControl != 0 {
			if customShortcut.KeyName == fyne.KeyC {
				if t.selection != nil && t.selection.HasSelection() {
					// Copy selection and clear
					t.selection.CopyToClipboard()
					t.selection.Clear()
					fmt.Printf("Copied selection and cleared\n")
					return
				} else {
					// No selection - send interrupt
					t.WriteToPTY([]byte{0x03})
					return
				}
			}
		}
		// Handle Alt modifier combinations FIRST (before Control)
		if customShortcut.Modifier&fyne.KeyModifierAlt != 0 {
			// Alt+key sends ESC followed by the key
			var sequence []byte

			// Check if it's Alt+Control combo (handle differently)
			if customShortcut.Modifier&fyne.KeyModifierControl != 0 {
				// Alt+Ctrl combinations - less common but some apps use them
				fmt.Printf("Alt+Ctrl combo detected\n")
				// You can implement specific handling here if needed
				return
			}

			// Pure Alt+key combinations
			if keyChar := t.keyNameToChar(customShortcut.KeyName); keyChar != 0 {
				sequence = []byte{0x1B, keyChar} // ESC + character
				fmt.Printf("Sending Alt+%c sequence: ESC+%c (0x1B 0x%02X)\n",
					keyChar, keyChar, keyChar)
			} else {
				// Handle special keys with Alt
				switch customShortcut.KeyName {
				case fyne.KeyLeft:
					sequence = []byte{0x1B, 0x5B, 0x44} // ESC[D
				case fyne.KeyRight:
					sequence = []byte{0x1B, 0x5B, 0x43} // ESC[C
				case fyne.KeyUp:
					sequence = []byte{0x1B, 0x5B, 0x41} // ESC[A
				case fyne.KeyDown:
					sequence = []byte{0x1B, 0x5B, 0x42} // ESC[B
				case fyne.KeyBackspace:
					sequence = []byte{0x1B, 0x7F} // ESC + DEL
				case fyne.KeyReturn:
					sequence = []byte{0x1B, 0x0D} // ESC + CR
				}
			}

			if len(sequence) > 0 {
				t.WriteToPTY(sequence)
				t.updatePending = true
				if t.screen != nil {
					t.screen.InvalidateCache()
				}
				t.triggerImmediateRedraw()
				return
			}
		}

		// Handle Control modifier combinations (existing code)
		if customShortcut.Modifier&fyne.KeyModifierControl != 0 {
			controlByte := t.keyNameToControlByte(customShortcut.KeyName)
			if controlByte != 0 {
				fmt.Printf("Sending control sequence: Ctrl+%c (0x%02X)\n",
					controlByte+64, controlByte)

				t.WriteToPTY([]byte{controlByte})
				t.updatePending = true

				if t.screen != nil {
					t.screen.InvalidateCache()
				}

				t.triggerImmediateRedraw()
				return
			}
		}
	}

	// Handle standard shortcuts (Copy, Paste, Cut, etc.)
	switch shortcut := shortcut.(type) {
	case *fyne.ShortcutCopy:
		fmt.Printf("Copy shortcut (Ctrl+C) - sending interrupt\n")
		t.WriteToPTY([]byte{0x03}) // Ctrl+C

	case *fyne.ShortcutPaste:
		fmt.Printf("Paste shortcut (Ctrl+V) detected\n")
		if shortcut.Clipboard != nil {
			content := shortcut.Clipboard.Content()
			if content != "" {
				fmt.Printf("Pasting content: %q\n", content)
				t.WriteToPTY([]byte(content))
			}
		}

	case *fyne.ShortcutCut:
		fmt.Printf("Cut shortcut (Ctrl+X) detected\n")
		t.WriteToPTY([]byte{0x18}) // Ctrl+X

	default:
		fmt.Printf("Unhandled shortcut type: %T\n", shortcut)
		// Call the embedded shortcut handler for other shortcuts
		t.ShortcutHandler.TypedShortcut(shortcut)
	}

	t.updatePending = true
}

// Add this helper method for Alt key character mapping
func (t *NativeTerminalWidget) keyNameToChar(keyName fyne.KeyName) byte {
	switch keyName {
	case fyne.KeyA:
		return 'a'
	case fyne.KeyB:
		return 'b'
	case fyne.KeyC:
		return 'c'
	case fyne.KeyD:
		return 'd'
	case fyne.KeyE:
		return 'e'
	case fyne.KeyF:
		return 'f'
	case fyne.KeyG:
		return 'g'
	case fyne.KeyH:
		return 'h'
	case fyne.KeyI:
		return 'i'
	case fyne.KeyJ:
		return 'j'
	case fyne.KeyK:
		return 'k'
	case fyne.KeyL:
		return 'l'
	case fyne.KeyM:
		return 'm'
	case fyne.KeyN:
		return 'n'
	case fyne.KeyO:
		return 'o'
	case fyne.KeyP:
		return 'p'
	case fyne.KeyQ:
		return 'q'
	case fyne.KeyR:
		return 'r'
	case fyne.KeyS:
		return 's'
	case fyne.KeyT:
		return 't'
	case fyne.KeyU:
		return 'u'
	case fyne.KeyV:
		return 'v'
	case fyne.KeyW:
		return 'w'
	case fyne.KeyX:
		return 'x'
	case fyne.KeyY:
		return 'y'
	case fyne.KeyZ:
		return 'z'
	case fyne.Key0:
		return '0'
	case fyne.Key1:
		return '1'
	case fyne.Key2:
		return '2'
	case fyne.Key3:
		return '3'
	case fyne.Key4:
		return '4'
	case fyne.Key5:
		return '5'
	case fyne.Key6:
		return '6'
	case fyne.Key7:
		return '7'
	case fyne.Key8:
		return '8'
	case fyne.Key9:
		return '9'
	default:
		return 0
	}
}

// ADD: keyNameToControlByte method
func (t *NativeTerminalWidget) keyNameToControlByte(keyName fyne.KeyName) byte {
	switch keyName {
	case fyne.KeyA:
		return 0x01 // Ctrl+A
	case fyne.KeyB:
		return 0x02 // Ctrl+B
	case fyne.KeyC:
		return 0x03 // Ctrl+C
	case fyne.KeyD:
		return 0x04 // Ctrl+D
	case fyne.KeyE:
		return 0x05 // Ctrl+E
	case fyne.KeyF:
		return 0x06 // Ctrl+F
	case fyne.KeyG:
		return 0x07 // Ctrl+G
	case fyne.KeyH:
		return 0x08 // Ctrl+H
	case fyne.KeyI:
		return 0x09 // Ctrl+I
	case fyne.KeyJ:
		return 0x0A // Ctrl+J
	case fyne.KeyK:
		return 0x0B // Ctrl+K
	case fyne.KeyL:
		return 0x0C // Ctrl+L
	case fyne.KeyM:
		return 0x0D // Ctrl+M
	case fyne.KeyN:
		return 0x0E // Ctrl+N
	case fyne.KeyO:
		return 0x0F // Ctrl+O
	case fyne.KeyP:
		return 0x10 // Ctrl+P
	case fyne.KeyQ:
		return 0x11 // Ctrl+Q
	case fyne.KeyR:
		return 0x12 // Ctrl+R
	case fyne.KeyS:
		return 0x13 // Ctrl+S
	case fyne.KeyT:
		return 0x14 // Ctrl+T
	case fyne.KeyU:
		return 0x15 // Ctrl+U
	case fyne.KeyV:
		return 0x16 // Ctrl+V
	case fyne.KeyW:
		return 0x17 // Ctrl+W
	case fyne.KeyX:
		return 0x18 // Ctrl+X
	case fyne.KeyY:
		return 0x19 // Ctrl+Y
	case fyne.KeyZ:
		return 0x1A // Ctrl+Z
	default:
		return 0
	}
}

// ADD: triggerImmediateRedraw method (removed - already exists in terminal_events.go)

// ADD: Missing unified history methods (removed - already exist in other files)

// CreateRenderer with unified scroll container
func (t *NativeTerminalWidget) CreateRenderer() fyne.WidgetRenderer {
	log.Printf("CreateRenderer: Creating unified renderer with enhanced scroll container")

	// Create enhanced hybrid scroll container
	t.scroll = NewHybridScrollContainer(t)
	t.scroll.OptimizeForVirtualScrolling()

	return &unifiedTerminalRenderer{
		widget:  t,
		scroll:  t.scroll,
		content: t.scroll,
	}
}

type unifiedTerminalRenderer struct {
	widget  *NativeTerminalWidget
	scroll  *HybridScrollContainer
	content fyne.CanvasObject
}

// Ensure we implement all required fyne.WidgetRenderer methods
func (r *unifiedTerminalRenderer) Layout(size fyne.Size) {
	r.content.Resize(size)

	widget := r.widget
	cols, rows := widget.CalculateTerminalSize(size.Width, size.Height)

	widget.mutex.RLock()
	currentCols, currentRows := widget.cols, widget.rows
	needsUpdate := cols != currentCols || rows != currentRows
	widget.mutex.RUnlock()

	if needsUpdate {
		log.Printf("Layout: Unified terminal resize from %dx%d to %dx%d",
			currentCols, currentRows, cols, rows)

		// Use unified resize handling
		widget.handleResizeUnified(size.Width, size.Height)
	}
}

func (r *unifiedTerminalRenderer) MinSize() fyne.Size {
	return fyne.NewSize(400, 300)
}

func (r *unifiedTerminalRenderer) Refresh() {
	if r.content != nil {
		r.content.Refresh()
	}
}

func (r *unifiedTerminalRenderer) Objects() []fyne.CanvasObject {
	if r.content != nil {
		return []fyne.CanvasObject{r.content}
	}
	return []fyne.CanvasObject{}
}

func (r *unifiedTerminalRenderer) Destroy() {
	// Clean up any resources if needed
}

// ENHANCED DISPLAY PROCESSING - Works with unified history system
func (t *NativeTerminalWidget) performRedrawUnified() {
	t.mutex.RLock()

	// Get display data
	allLines := t.screen.GetDisplay()
	allAttrs := t.screen.GetAttributes()
	isUsingAlternate := t.screen.IsUsingAlternate()
	isViewingHistory := t.IsInHistoryModeUnified()

	t.mutex.RUnlock()

	// Route to appropriate renderer
	if isUsingAlternate {
		// Full screen applications (vim, htop, etc.)
		go func() {
			fyne.Do(func() {
				t.renderAlternateScreenUnified(allLines, allAttrs)
			})
		}()
	} else {
		// Normal shell mode with unified history
		shouldAutoScroll := !isViewingHistory
		go func() {
			fyne.Do(func() {
				t.renderNormalModeUnified(allLines, allAttrs, shouldAutoScroll)
			})
		}()
	}
}

func (t *NativeTerminalWidget) renderNormalModeUnified(allLines []string, allAttrs [][]gopyte.Attributes, shouldAutoScroll bool) {
	log.Printf("NORMAL (%s): Rendering with unified virtual scrolling, lines=%d, autoScroll=%v",
		runtime.GOOS, len(allLines), shouldAutoScroll)

	if len(allLines) == 0 {
		t.textGrid.SetText("")
		return
	}

	// Calculate viewport using unified system
	viewport := t.calculateUnifiedViewport(allLines)

	// Size TextGrid to viewport
	viewportSize := fyne.NewSize(
		float32(t.cols)*t.charWidth,
		float32(viewport.visibleLines)*t.charHeight,
	)

	currentSize := t.textGrid.Size()
	if currentSize.Width != viewportSize.Width || currentSize.Height != viewportSize.Height {
		t.textGrid.Resize(viewportSize)
		log.Printf("NORMAL (%s): Resized viewport to %dx%d",
			runtime.GOOS, t.cols, viewport.visibleLines)
	}

	// Extract visible content from unified system
	visibleLines := t.extractUnifiedVisibleContent(allLines, viewport)
	visibleAttrs := t.extractUnifiedVisibleAttributes(allAttrs, viewport)

	// Handle cursor positioning
	cursorX, cursorY := t.screen.GetCursor()
	adjustedCursorY := t.adjustUnifiedCursor(cursorX, cursorY, viewport, len(allLines))

	// Place cursor if visible and not in history mode
	if adjustedCursorY >= 0 && adjustedCursorY < len(visibleLines) &&
		cursorX >= 0 && cursorX < t.cols && !t.IsInHistoryModeUnified() {
		t.placeCursorInLineFast(&visibleLines[adjustedCursorY], cursorX)
		log.Printf("NORMAL (%s): Cursor at (%d,%d) in viewport", runtime.GOOS, cursorX, adjustedCursorY)
	}

	// Set visible content
	fullText := strings.Join(visibleLines, "\n")
	t.textGrid.SetText(fullText)

	// Apply colors
	if len(visibleAttrs) > 0 {
		t.applyColorsToTextGrid(visibleLines, visibleAttrs)
	} else {
		t.textGrid.Refresh()
	}
	if t.selection != nil && (t.selection.HasSelection() || t.selection.IsSelecting()) {
		fmt.Printf("Applying selection highlight\n")
		t.selection.ApplyHighlight(t.textGrid.Rows, viewport)
		t.textGrid.Refresh() // Ensure refresh after highlight
	}
	// Update scroll bar position
	t.updateUnifiedScrollBar(viewport)

	log.Printf("NORMAL (%s): Rendered viewport lines %d-%d of %d total",
		runtime.GOOS, viewport.scrollOffset, viewport.scrollOffset+viewport.visibleLines-1, len(allLines))
}

// UNIFIED VIEWPORT CALCULATIONS
func (t *NativeTerminalWidget) calculateUnifiedViewport(allLines []string) VirtualScrollState {
	totalLines := len(allLines)
	visibleLines := t.rows
	if visibleLines <= 0 {
		visibleLines = 24
	}

	var scrollOffset int

	if t.IsInHistoryModeUnified() {
		// UNIFIED HISTORY MODE CALCULATION
		pos, total := t.GetHistoryPosition()

		log.Printf("calculateUnifiedViewport (%s): HISTORY MODE - pos=%d, total=%d, totalLines=%d",
			runtime.GOOS, pos, total, totalLines)

		if totalLines <= visibleLines {
			scrollOffset = 0
		} else {
			maxScrollOffset := totalLines - visibleLines

			if total > 0 {
				// Map history position to scroll offset
				scrollOffset = maxScrollOffset - ((pos * maxScrollOffset) / total)
			} else {
				scrollOffset = maxScrollOffset
			}

			// Handle maximum position (top of history)
			if pos >= total {
				scrollOffset = 0
			}

			// Ensure bounds
			if scrollOffset < 0 {
				scrollOffset = 0
			}
			if scrollOffset > maxScrollOffset {
				scrollOffset = maxScrollOffset
			}
		}

		log.Printf("calculateUnifiedViewport (%s): HISTORY - pos=%d/%d -> scrollOffset=%d",
			runtime.GOOS, pos, total, scrollOffset)
	} else {
		// Normal mode: show bottom (latest output)
		if totalLines <= visibleLines {
			scrollOffset = 0
		} else {
			scrollOffset = totalLines - visibleLines
		}

		log.Printf("calculateUnifiedViewport (%s): NORMAL MODE - scrollOffset=%d",
			runtime.GOOS, scrollOffset)
	}

	// Calculate maximum scroll
	maxScroll := totalLines - visibleLines
	if maxScroll < 0 {
		maxScroll = 0
	}

	// Final bounds check
	if scrollOffset > maxScroll {
		scrollOffset = maxScroll
	}
	if scrollOffset < 0 {
		scrollOffset = 0
	}

	return VirtualScrollState{
		totalLines:    totalLines,
		visibleLines:  visibleLines,
		scrollOffset:  scrollOffset,
		maxScroll:     maxScroll,
		contentHeight: float32(totalLines) * t.charHeight,
	}
}

func (t *NativeTerminalWidget) extractUnifiedVisibleContent(allLines []string, viewport VirtualScrollState) []string {
	visibleContent := make([]string, viewport.visibleLines)

	for i := 0; i < viewport.visibleLines; i++ {
		lineIndex := viewport.scrollOffset + i
		if lineIndex < len(allLines) {
			visibleContent[i] = t.padLineToWidth(allLines[lineIndex])
		} else {
			visibleContent[i] = t.padLineToWidth("")
		}
	}

	return visibleContent
}

func (t *NativeTerminalWidget) extractUnifiedVisibleAttributes(allAttrs [][]gopyte.Attributes, viewport VirtualScrollState) [][]gopyte.Attributes {
	if len(allAttrs) == 0 {
		return [][]gopyte.Attributes{}
	}

	visibleAttrs := make([][]gopyte.Attributes, viewport.visibleLines)

	for i := 0; i < viewport.visibleLines; i++ {
		lineIndex := viewport.scrollOffset + i
		if lineIndex < len(allAttrs) {
			visibleAttrs[i] = allAttrs[lineIndex]
		}
	}

	return visibleAttrs
}

func (t *NativeTerminalWidget) adjustUnifiedCursor(cursorX, cursorY int, viewport VirtualScrollState, totalLines int) int {
	// Don't show cursor in history mode
	if t.IsInHistoryModeUnified() {
		return -1
	}

	// Calculate cursor position in viewport
	if totalLines <= viewport.visibleLines {
		return cursorY
	} else {
		actualCursorLine := totalLines - viewport.visibleLines + cursorY
		if actualCursorLine >= viewport.scrollOffset && actualCursorLine < viewport.scrollOffset+viewport.visibleLines {
			return actualCursorLine - viewport.scrollOffset
		}
		return -1 // Cursor not visible
	}
}

// UTILITY METHODS
func (t *NativeTerminalWidget) padLineToWidth(line string) string {
	runes := []rune(line)
	if len(runes) < t.cols {
		padding := strings.Repeat(" ", t.cols-len(runes))
		return line + padding
	} else if len(runes) > t.cols {
		return string(runes[:t.cols])
	}
	return line
}

func (t *NativeTerminalWidget) updateUnifiedScrollBar(viewport VirtualScrollState) {
	if t.scroll == nil {
		return
	}

	go func() {
		time.Sleep(5 * time.Millisecond)
		fyne.Do(func() {
			if viewport.maxScroll > 0 {
				scrollPercentage := float32(viewport.scrollOffset) / float32(viewport.maxScroll)

				// Update HybridScrollContainer position
				containerSize := t.scroll.Scroll.Size()
				if viewport.contentHeight > containerSize.Height {
					maxScrollY := viewport.contentHeight - containerSize.Height
					scrollY := maxScrollY * scrollPercentage
					t.scroll.Scroll.Offset = fyne.NewPos(0, scrollY)
					t.scroll.Scroll.Refresh()
				}
			} else {
				t.scroll.Scroll.Offset = fyne.NewPos(0, 0)
				t.scroll.Scroll.Refresh()
			}
		})
	}()
}

// PUBLIC API - Enhanced unified methods
func (t *NativeTerminalWidget) GetTitle() string {
	t.mutex.RLock()
	defer t.mutex.RUnlock()
	return t.title
}

func (t *NativeTerminalWidget) GetContext() context.Context {
	return t.ctx
}

func (t *NativeTerminalWidget) Clear() {
	// Cross-platform clear screen command
	t.WriteToPTY([]byte("\x1b[2J\x1b[H"))
}

func (t *NativeTerminalWidget) GetHistorySize() int {
	if t.screen != nil {
		return t.screen.GetHistorySize()
	}
	return 0
}

func (t *NativeTerminalWidget) IsInHistoryMode() bool {
	if t.screen != nil {
		return t.screen.IsViewingHistory()
	}
	return false
}

func (t *NativeTerminalWidget) ScrollToTop() {
	if t.screen != nil {
		t.screen.ScrollToTop()
		t.updatePending = true
	}
}

func (t *NativeTerminalWidget) ScrollToBottom() {
	if t.screen != nil {
		t.screen.ScrollToBottom()
		t.updatePending = true
	}
}

func (t *NativeTerminalWidget) SetMaxHistoryLines(maxLines int) {
	// ONLY set WideCharScreen history - remove PTYManager part entirely
	if t.screen != nil {
		t.screen.SetMaxHistoryLines(maxLines)
		log.Printf("Set max history lines to %d", maxLines)
	}
}

func (t *NativeTerminalWidget) TestColors() {
	// Enhanced color test that works on both Windows and Unix
	var colorTest string

	if t.isWindows {
		// Windows-specific color test
		colorTest = "\x1b[31mRed (Win)\x1b[0m \x1b[32mGreen (Win)\x1b[0m \x1b[33mYellow (Win)\x1b[0m \x1b[34mBlue (Win)\x1b[0m\n"
	} else {
		// Unix color test
		colorTest = "\x1b[31mRed (Unix)\x1b[0m \x1b[32mGreen (Unix)\x1b[0m \x1b[33mYellow (Unix)\x1b[0m \x1b[34mBlue (Unix)\x1b[0m\n"
	}

	brightTest := "\x1b[91mBright Red\x1b[0m \x1b[92mBright Green\x1b[0m \x1b[93mBright Yellow\x1b[0m \x1b[94mBright Blue\x1b[0m\n"
	formatTest := "\x1b[1mBold\x1b[0m \x1b[3mItalic\x1b[0m \x1b[4mUnderline\x1b[0m Normal\n"
	platformTest := fmt.Sprintf("Platform: %s | ConPTY: %v | PTY: %v\n", runtime.GOOS, t.isWindows, t.isUnix)

	fullTest := colorTest + brightTest + formatTest + platformTest + "Color test completed!\n"
	t.WriteToPTY([]byte(fullTest))
}

func (t *NativeTerminalWidget) Close() {
	log.Printf("Closing unified terminal widget (%s)", runtime.GOOS)

	// Cancel context
	t.cancel()

	// Stop resize timer
	t.resizeMutex.Lock()
	if t.resizeTimer != nil {
		t.resizeTimer.Stop()
	}
	t.resizeMutex.Unlock()

	// Close unified PTY
	t.CloseUnified()
}

// ENHANCED DEBUG METHODS
func (t *NativeTerminalWidget) DebugUnifiedState() {
	fmt.Printf("=== UNIFIED TERMINAL DEBUG STATE ===\n")
	fmt.Printf("Platform: %s (Windows: %v, Unix: %v)\n", runtime.GOOS, t.isWindows, t.isUnix)
	fmt.Printf("Terminal size: %dx%d\n", t.cols, t.rows)
	fmt.Printf("Character dimensions: %.2fx%.2f\n", t.charWidth, t.charHeight)
	fmt.Printf("Widget focus: %v\n", t.hasFocus)

	if t.ptyManager != nil {
		fmt.Printf("PTY Manager initialized: %v\n", t.ptyManager.pty != nil)
	} else {
		fmt.Printf("PTY Manager: NOT INITIALIZED\n")
	}

	if t.screen != nil {
		fmt.Printf("Screen initialized: %v\n", true)
		fmt.Printf("Using alternate screen: %v\n", t.screen.IsUsingAlternate())
		fmt.Printf("Screen history size: %d\n", t.screen.GetHistorySize())
	} else {
		fmt.Printf("Screen: NOT INITIALIZED\n")
	}

	fmt.Printf("Virtual scroll state: offset=%d, visible=%d, total=%d, max=%d\n",
		t.virtualScroll.scrollOffset, t.virtualScroll.visibleLines,
		t.virtualScroll.totalLines, t.virtualScroll.maxScroll)

	fmt.Printf("=====================================\n")
}

func (t *NativeTerminalWidget) TestUnifiedFeatures() {
	fmt.Printf("=== TESTING UNIFIED TERMINAL FEATURES ===\n")

	// Test platform detection
	fmt.Printf("Platform detection: %s\n", runtime.GOOS)

	// Test PTY initialization
	if t.ptyManager != nil && t.ptyManager.pty != nil {
		fmt.Printf("PTY system: WORKING (%T)\n", t.ptyManager.pty)
	} else {
		fmt.Printf("PTY system: NOT INITIALIZED\n")
		return
	}

	// Test writing to PTY
	testCmd := "echo \"Unified terminal test successful\"\n"
	err := t.WriteToPTY([]byte(testCmd))
	if err != nil {
		fmt.Printf("PTY write test: FAILED (%v)\n", err)
	} else {
		fmt.Printf("PTY write test: PASSED\n")
	}

	fmt.Printf("==========================================\n")
}

// Implement the Tabbable interface to capture Tab key events
func (t *NativeTerminalWidget) AcceptsTab() bool {
	return true // This tells Fyne to send Tab key events to this widget
}

func (t *NativeTerminalWidget) TestControlKeyShortcuts() {
	fmt.Printf("=== TESTING CONTROL KEY SHORTCUTS ===\n")

	// Test various control key shortcuts
	testShortcuts := []*desktop.CustomShortcut{
		{KeyName: fyne.KeyC, Modifier: fyne.KeyModifierControl}, // Ctrl+C
		{KeyName: fyne.KeyD, Modifier: fyne.KeyModifierControl}, // Ctrl+D
		{KeyName: fyne.KeyL, Modifier: fyne.KeyModifierControl}, // Ctrl+L
		{KeyName: fyne.KeyZ, Modifier: fyne.KeyModifierControl}, // Ctrl+Z
	}

	for _, shortcut := range testShortcuts {
		fmt.Printf("Testing Ctrl+%s...\n", shortcut.KeyName)
		t.TypedShortcut(shortcut)
		time.Sleep(200 * time.Millisecond)
	}

	fmt.Printf("Control key shortcut test completed\n")
}

// PERFORMANCE MONITORING
func (t *NativeTerminalWidget) StartPerformanceMonitoring() {
	go func() {
		ticker := time.NewTicker(10 * time.Second)
		defer ticker.Stop()

		startTime := time.Now()

		for {
			select {
			case <-ticker.C:
				currentTime := time.Now()
				elapsed := currentTime.Sub(startTime)

				// Monitor update channel
				channelLen := len(t.updateChannel)
				channelCap := cap(t.updateChannel)

				// Monitor gopyte history
				gopyteHistory := 0
				if t.screen != nil {
					gopyteHistory = t.screen.GetHistorySize()
				}

				log.Printf("PERF (%s): Runtime=%.1fs | UpdateChan=%d/%d | Gopyte=%d | Focus=%v",
					runtime.GOOS, elapsed.Seconds(), channelLen, channelCap,
					gopyteHistory, t.hasFocus)

				// Warn about potential issues
				if channelLen > channelCap/2 {
					log.Printf("WARNING: Update channel is %d%% full", (channelLen*100)/channelCap)
				}

			case <-t.ctx.Done():
				log.Printf("Performance monitoring stopped")
				return
			}
		}
	}()

	log.Printf("Started performance monitoring for %s terminal", runtime.GOOS)
}

// ENHANCED UPDATE PROCESSOR - Same as before but with logging
func (t *NativeTerminalWidget) updateProcessor() {
	ticker := time.NewTicker(33 * time.Millisecond) // ~30 FPS
	defer ticker.Stop()

	var lastUpdateTime time.Time
	updateCooldown := 16 * time.Millisecond
	updateCount := 0

	log.Printf("Unified update processor started for %s", runtime.GOOS)

	for {
		select {
		case <-ticker.C:
			now := time.Now()
			if t.updatePending && now.Sub(lastUpdateTime) >= updateCooldown {
				t.performRedrawUnified()
				t.updatePending = false
				lastUpdateTime = now
				updateCount++

				// Log occasionally for monitoring
				if updateCount%100 == 0 {
					log.Printf("Update processor (%s): %d redraws completed", runtime.GOOS, updateCount)
				}
			}
		case <-t.ctx.Done():
			log.Printf("Unified update processor stopping after %d updates", updateCount)
			return
		}
	}
}

// HELPER METHODS
func (t *NativeTerminalWidget) calculateCharDimensions() {
	// Platform-specific character dimension calculations
	switch runtime.GOOS {
	case "windows":
		// Windows tends to have slightly different font rendering
		t.charWidth = t.fontSize * 0.58
		t.charHeight = t.fontSize * 1.25
	case "darwin":
		// macOS font rendering
		t.charWidth = t.fontSize * 0.55
		t.charHeight = t.fontSize * 1.15
	default:
		// Linux and other Unix systems
		t.charWidth = t.fontSize * 0.56
		t.charHeight = t.fontSize * 1.22
	}

	log.Printf("Character dimensions (%s): %.2fx%.2f for fontSize %.1f",
		runtime.GOOS, t.charWidth, t.charHeight, t.fontSize)
}

func (t *NativeTerminalWidget) initializeTextGridSize() {
	initialSize := fyne.NewSize(
		float32(t.cols)*t.charWidth,
		float32(t.rows)*t.charHeight,
	)
	t.textGrid.Resize(initialSize)

	log.Printf("Initial TextGrid size (%s): %.1fx%.1f for %dx%d terminal",
		runtime.GOOS, initialSize.Width, initialSize.Height, t.cols, t.rows)
}

func (t *NativeTerminalWidget) placeCursorInLineFast(line *string, cursorX int) {
	if line == nil || cursorX < 0 {
		return
	}

	currentLine := *line
	runes := []rune(currentLine)

	if cursorX < len(runes) {
		// Replace character at cursor position with block cursor
		runes[cursorX] = '█' // Block cursor character
		*line = string(runes)
	} else if cursorX < t.cols {
		// Extend line with spaces and place cursor
		padLen := cursorX - len(runes)
		if padLen > 0 {
			padding := strings.Repeat(" ", padLen)
			*line = currentLine + padding + "█"
		} else {
			*line = currentLine + "█"
		}
	}
}

// Utility function for minimum
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// Helper functions
func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

// MISSING METHOD 2: extractWindowTitle
func (t *NativeTerminalWidget) extractWindowTitle(data string) {
	start := strings.Index(data, "\x1b]0;")
	if start >= 0 {
		start += 4 // Skip past "\x1b]0;"

		// Find end of title (BEL or ESC\)
		end := strings.IndexAny(data[start:], "\x07\x1b")
		if end >= 0 {
			newTitle := data[start : start+end]
			if newTitle != t.title {
				t.title = newTitle
				log.Printf("Window title changed to: %s", newTitle)
			}
		}
	}
}

// MISSING METHOD 3: handleResizeUnified (alias to existing method)
func (t *NativeTerminalWidget) handleResizeUnified(width, height float32) {
	// Call your existing handleResize method
	t.handleResize(width, height)
}

// MISSING METHOD 4: applyColorsToTextGrid
func (t *NativeTerminalWidget) applyColorsToTextGrid(lines []string, attrs [][]gopyte.Attributes) {
	if len(t.textGrid.Rows) == 0 {
		return
	}

	if len(attrs) == 0 || len(attrs) != len(lines) {
		t.textGrid.Refresh()
		return
	}

	for rowIdx, line := range lines {
		if rowIdx >= len(t.textGrid.Rows) || rowIdx >= len(attrs) {
			break
		}

		t.applyLineColorsFromAttributes(rowIdx, line, attrs[rowIdx])
	}

	t.textGrid.Refresh()
}

// HELPER METHOD: applyLineColorsFromAttributes
func (t *NativeTerminalWidget) applyLineColorsFromAttributes(rowIdx int, line string, lineAttrs []gopyte.Attributes) {
	if rowIdx >= len(t.textGrid.Rows) {
		return
	}

	row := t.textGrid.Rows[rowIdx]
	runes := []rune(line)

	for charIdx, char := range runes {
		if charIdx >= len(row.Cells) || charIdx >= len(lineAttrs) {
			break
		}

		attr := lineAttrs[charIdx]

		if row.Cells[charIdx].Style == nil {
			row.Cells[charIdx].Style = &widget.CustomTextGridStyle{}
		}

		style := row.Cells[charIdx].Style.(*widget.CustomTextGridStyle)

		if fgColor := t.mapGopyteColorToFyne(attr.Fg); fgColor != nil {
			style.FGColor = fgColor
		}

		if bgColor := t.mapGopyteColorToFyne(attr.Bg); bgColor != nil {
			style.BGColor = bgColor
		}

		if attr.Bold {
			if brightColor := t.makeBrighter(style.FGColor); brightColor != nil {
				style.FGColor = brightColor
			}
		}

		row.Cells[charIdx].Rune = char
	}
}

// HELPER METHOD: mapGopyteColorToFyne
func (t *NativeTerminalWidget) mapGopyteColorToFyne(colorName string) color.Color {
	if colorName == "" || colorName == "default" {
		return nil
	}

	if fyneColor, exists := colorMappings[colorName]; exists {
		return fyneColor
	}

	if strings.HasPrefix(colorName, "color") {
		return colorMappings["white"]
	}

	switch colorName {
	case "brown":
		return colorMappings["yellow"]
	default:
		return colorMappings["white"]
	}
}

// HELPER METHOD: makeBrighter
func (t *NativeTerminalWidget) makeBrighter(c color.Color) color.Color {
	if c == nil {
		return nil
	}

	r, g, b, a := c.RGBA()
	r8, g8, b8, a8 := uint8(r>>8), uint8(g>>8), uint8(b>>8), uint8(a>>8)

	brighten := uint8(40)

	newR := r8
	if r8 < 255-brighten {
		newR = r8 + brighten
	} else {
		newR = 255
	}

	newG := g8
	if g8 < 255-brighten {
		newG = g8 + brighten
	} else {
		newG = 255
	}

	newB := b8
	if b8 < 255-brighten {
		newB = b8 + brighten
	} else {
		newB = 255
	}

	return color.RGBA{newR, newG, newB, a8}
}

func (t *NativeTerminalWidget) renderAlternateScreenUnified(allLines []string, allAttrs [][]gopyte.Attributes) {
	log.Printf("ALTERNATE (%s): Rendering full screen mode", runtime.GOOS)

	// Size TextGrid to exact screen dimensions
	screenSize := fyne.NewSize(
		float32(t.cols)*t.charWidth,
		float32(t.rows)*t.charHeight,
	)

	currentSize := t.textGrid.Size()
	if currentSize.Width != screenSize.Width || currentSize.Height != screenSize.Height {
		t.textGrid.Resize(screenSize)
		log.Printf("ALTERNATE (%s): Resized to %dx%d", runtime.GOOS, t.cols, t.rows)
	}

	// Display handling for alternate screen
	var displayLines []string
	var displayAttrs [][]gopyte.Attributes

	if len(allLines) == t.rows {
		// Perfect match - use exactly what app provides
		displayLines = make([]string, len(allLines))
		for i, line := range allLines {
			displayLines[i] = t.padLineToWidth(line)
		}
		displayAttrs = allAttrs
		log.Printf("ALTERNATE: Using app's exact %d lines", len(allLines))

	} else if len(allLines) < t.rows {
		// App provided fewer lines - pad to screen height
		displayLines = make([]string, t.rows)
		displayAttrs = make([][]gopyte.Attributes, t.rows)

		for i := 0; i < t.rows; i++ {
			if i < len(allLines) {
				displayLines[i] = t.padLineToWidth(allLines[i])
				if i < len(allAttrs) {
					displayAttrs[i] = allAttrs[i]
				}
			} else {
				displayLines[i] = t.padLineToWidth("")
			}
		}
		log.Printf("ALTERNATE: Using %d app lines, padded to %d", len(allLines), t.rows)

	} else {
		// App provided MORE lines than screen height
		startIdx := len(allLines) - t.rows
		displayLines = make([]string, t.rows)
		displayAttrs = make([][]gopyte.Attributes, t.rows)

		for i := 0; i < t.rows; i++ {
			displayLines[i] = t.padLineToWidth(allLines[startIdx+i])
			if startIdx+i < len(allAttrs) {
				displayAttrs[i] = allAttrs[startIdx+i]
			}
		}
		log.Printf("ALTERNATE: Used last %d lines from %d total", t.rows, len(allLines))
	}

	// Get cursor position
	cursorX, cursorY := t.screen.GetCursor()

	// Place cursor exactly where app says it should be
	if cursorY >= 0 && cursorY < len(displayLines) && cursorX >= 0 && cursorX < t.cols {
		t.placeCursorInLineFast(&displayLines[cursorY], cursorX)
	}

	// Set content exactly as app wants it
	fullText := strings.Join(displayLines, "\n")
	t.textGrid.SetText(fullText)

	// Apply colors
	if len(displayAttrs) > 0 {
		t.applyColorsToTextGrid(displayLines, displayAttrs)
	} else {
		t.textGrid.Refresh()
	}

	log.Printf("ALTERNATE: Rendered %d lines", len(displayLines))
}

// Add the missing DragEnd method
func (t *NativeTerminalWidget) DragEnd() {
	fmt.Printf("DragEnd called\n")

	if t.selection != nil && t.selection.IsSelecting() {
		// Finalize selection
		t.selection.HandleMouseUp(&desktop.MouseEvent{
			Button: desktop.MouseButtonPrimary,
		})
	}
}

// Make sure you have Dragged defined correctly
func (t *NativeTerminalWidget) Dragged(event *fyne.DragEvent) {
	fmt.Printf("Dragged to %.1f,%.1f\n", event.Position.X, event.Position.Y)

	if t.isSelecting {
		t.selectionEnd = event.Position

		if t.selection != nil {
			t.selection.HandleDrag(event.Position)
		}
	}
}

func (t *NativeTerminalWidget) Tapped(event *fyne.PointEvent) {
	fmt.Printf("Tapped at %.1f,%.1f (was selecting: %v)\n",
		event.Position.X, event.Position.Y, t.isSelecting)

	if t.isSelecting {
		// Complete the selection
		t.isSelecting = false
		if t.selection != nil {
			t.selection.HandleMouseUp(&desktop.MouseEvent{
				Button: desktop.MouseButtonPrimary,
			})
		}
	}
}
func (t *NativeTerminalWidget) findWordBoundaries(line string, col int) (start, end int) {
	runes := []rune(line)
	if col >= len(runes) {
		return col, col
	}

	// Word delimiters
	isDelimiter := func(r rune) bool {
		return r == ' ' || r == '\t' || r == '\\' || r == '/' ||
			r == '(' || r == ')' || r == '[' || r == ']' ||
			r == '{' || r == '}' || r == '<' || r == '>' ||
			r == '"' || r == '\'' || r == ',' || r == ';' ||
			r == ':' || r == '|' || r == '.' || r == '-'
	}

	// Find start of word
	start = col
	for start > 0 && !isDelimiter(runes[start-1]) {
		start--
	}

	// Find end of word
	end = col
	for end < len(runes) && !isDelimiter(runes[end]) {
		end++
	}

	return start, end
}

func (t *NativeTerminalWidget) DoubleTapped(event *fyne.PointEvent) {
	fmt.Printf("DoubleTapped at %.1f,%.1f\n", event.Position.X, event.Position.Y)

	col := int(event.Position.X / t.charWidth)
	row := int(event.Position.Y / t.charHeight)

	// Get display lines
	allLines := t.screen.GetDisplay()
	viewport := t.calculateUnifiedViewport(allLines)

	// Calculate actual line index with viewport offset
	actualRow := viewport.scrollOffset + row

	if actualRow < len(allLines) {
		line := allLines[actualRow]
		start, end := t.findWordBoundaries(line, col)

		fmt.Printf("Word boundaries: col %d -> start=%d, end=%d\n", col, start, end)

		// Set selection positions
		t.selection.startPos = fyne.NewPos(float32(start)*t.charWidth, float32(row)*t.charHeight)
		t.selection.endPos = fyne.NewPos(float32(end)*t.charWidth, float32(row)*t.charHeight)
		t.selection.hasSelection = true
		t.selection.isSelecting = false

		// Get and copy text
		selectedText := t.selection.GetSelectedText()
		fmt.Printf("Selected text: %q\n", selectedText)

		if selectedText != "" {
			t.selection.CopyToClipboard()
		}

		// Force redraw to show selection
		t.updatePending = true
	}
}

// Add to terminal_events.go

func (t *NativeTerminalWidget) TripleTapped(event *fyne.PointEvent) {
	fmt.Printf("TripleTapped at %.1f,%.1f - selecting entire line\n", event.Position.X, event.Position.Y)

	row := int(event.Position.Y / t.charHeight)

	// Get display lines
	allLines := t.screen.GetDisplay()
	viewport := t.calculateUnifiedViewport(allLines)

	// Calculate actual line index with viewport offset
	actualRow := viewport.scrollOffset + row

	if actualRow < len(allLines) {
		line := allLines[actualRow]

		// Select entire line
		t.selection.startPos = fyne.NewPos(0, float32(row)*t.charHeight)
		t.selection.endPos = fyne.NewPos(float32(len([]rune(line)))*t.charWidth, float32(row)*t.charHeight)
		t.selection.hasSelection = true
		t.selection.isSelecting = false

		// Get and copy text
		selectedText := strings.TrimSpace(line)
		fmt.Printf("Selected line: %q\n", selectedText)

		if selectedText != "" {
			window := fyne.CurrentApp().Driver().AllWindows()[0]
			clipboard := window.Clipboard()
			clipboard.SetContent(selectedText)
			fmt.Printf("Copied line to clipboard\n")
		}

		// Force redraw to show selection
		t.updatePending = true
	}
}

// ============================================================================
// desktop.Hoverable interface implementation
// ============================================================================

// MouseIn implements desktop.Hoverable
func (t *NativeTerminalWidget) MouseIn(event *desktop.MouseEvent) {
	// Optional: handle mouse enter - could show cursor or change state
}

// MouseOut implements desktop.Hoverable
func (t *NativeTerminalWidget) MouseOut() {
	// Optional: handle mouse leave
}

// MouseMoved implements desktop.Hoverable
func (t *NativeTerminalWidget) MouseMoved(event *desktop.MouseEvent) {
	// Optional: handle mouse movement for hover effects
}
