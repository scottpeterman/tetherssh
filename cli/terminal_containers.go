package main

import (
	"fmt"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/driver/desktop"
)

// HybridScrollContainer shows scroll bars but forwards mouse events to terminal
type HybridScrollContainer struct {
	*container.Scroll
	terminal *NativeTerminalWidget
}

// NewHybridScrollContainer creates a new hybrid scroll container
func NewHybridScrollContainer(terminal *NativeTerminalWidget) *HybridScrollContainer {
	baseScroll := container.NewScroll(terminal.textGrid)
	baseScroll.SetMinSize(fyne.NewSize(600, 400))
	baseScroll.Direction = container.ScrollVerticalOnly

	h := &HybridScrollContainer{
		Scroll:   baseScroll,
		terminal: terminal,
	}

	fmt.Printf("NewHybridScrollContainer: Created with terminal %p\n", terminal)
	return h
}

// Forward mouse events to terminal for selection
func (h *HybridScrollContainer) MouseDown(event *desktop.MouseEvent) {
	fmt.Printf("HybridScrollContainer.MouseDown: Forwarding to terminal\n")
	if h.terminal != nil {
		// Forward directly to terminal's selection manager
		h.terminal.isSelecting = true
		if h.terminal.selection != nil {
			h.terminal.selection.HandleMouseDown(event)
		}
		h.terminal.updatePending = true
	}
}

func (h *HybridScrollContainer) MouseUp(event *desktop.MouseEvent) {
	fmt.Printf("HybridScrollContainer.MouseUp: Forwarding to terminal\n")
	if h.terminal != nil {
		h.terminal.isSelecting = false
		if h.terminal.selection != nil {
			h.terminal.selection.HandleMouseUp(event)
		}
	}
}

func (h *HybridScrollContainer) Dragged(event *fyne.DragEvent) {
	fmt.Printf("HybridScrollContainer.Dragged: Forwarding to terminal\n")
	if h.terminal != nil && h.terminal.isSelecting {
		if h.terminal.selection != nil {
			h.terminal.selection.HandleDrag(event.Position)
		}
		h.terminal.updatePending = true
	}
}

func (h *HybridScrollContainer) DragEnd() {
	fmt.Printf("HybridScrollContainer.DragEnd: Forwarding to terminal\n")
	if h.terminal != nil {
		h.terminal.isSelecting = false
		if h.terminal.selection != nil {
			h.terminal.selection.HandleMouseUp(&desktop.MouseEvent{
				Button: desktop.MouseButtonPrimary,
			})
		}
	}
}

// Handle scroll wheel events
func (h *HybridScrollContainer) Scrolled(event *fyne.ScrollEvent) {
	fmt.Printf("HybridScrollContainer.Scrolled: DY=%.2f\n", event.Scrolled.DY)

	// Let terminal handle scroll events first
	handled := h.terminal.handleScrollEvent(event)

	if !handled {
		// Terminal didn't handle it, let scroll container handle it
		fmt.Printf("HybridScrollContainer.Scrolled: Terminal didn't handle, passing to container\n")
		h.Scroll.Scrolled(event)
	} else {
		fmt.Printf("HybridScrollContainer.Scrolled: Terminal handled scroll event\n")
	}
}

// SCROLL BAR POSITION MANAGEMENT

func (h *HybridScrollContainer) UpdateScrollPosition() {
	if h.terminal.screen.IsUsingAlternate() {
		// In alternate screen, no virtual scrolling - position at top
		h.ScrollToTop()
		fmt.Printf("UpdateScrollPosition: Alternate screen, scrolled to top\n")
		return
	}

	if h.terminal.IsInHistoryMode() {
		// In history mode - position scroll bar based on history position
		var scrollPercentage float32 = 0.5 // Mid-position as placeholder

		h.setScrollBarPosition(scrollPercentage)
		fmt.Printf("UpdateScrollPosition: History mode, percentage=%.2f\n", scrollPercentage)
	} else {
		// Normal mode - scroll to bottom
		h.ScrollToBottom()
		fmt.Printf("UpdateScrollPosition: Normal mode, scrolled to bottom\n")
	}
}

func (h *HybridScrollContainer) setScrollBarPosition(percentage float32) {
	if percentage < 0 {
		percentage = 0
	}
	if percentage > 1 {
		percentage = 1
	}

	// Get the actual content size from the TextGrid
	contentSize := h.terminal.textGrid.Size()
	containerSize := h.Scroll.Size()

	if contentSize.Height > containerSize.Height {
		maxScrollY := contentSize.Height - containerSize.Height
		scrollY := maxScrollY * percentage
		h.Scroll.Offset = fyne.NewPos(0, scrollY)
		h.Scroll.Refresh()

		fmt.Printf("setScrollBarPosition: Set scroll to %.1f (%.1f%%), maxScroll=%.1f\n",
			scrollY, percentage*100, maxScrollY)
	} else {
		// Content fits in container, no scrolling needed
		h.Scroll.Offset = fyne.NewPos(0, 0)
		h.Scroll.Refresh()
	}
}

func (h *HybridScrollContainer) ScrollToBottom() {
	contentSize := h.terminal.textGrid.Size()
	containerSize := h.Scroll.Size()

	if contentSize.Height > containerSize.Height {
		maxScrollY := contentSize.Height - containerSize.Height
		h.Scroll.Offset = fyne.NewPos(0, maxScrollY)
		h.Scroll.Refresh()
		fmt.Printf("ScrollToBottom: Scrolled to bottom (offset=%.1f)\n", maxScrollY)
	} else {
		h.Scroll.Offset = fyne.NewPos(0, 0)
		h.Scroll.Refresh()
		fmt.Printf("ScrollToBottom: Content fits, no scroll needed\n")
	}
}

func (h *HybridScrollContainer) ScrollToTop() {
	h.Scroll.Offset = fyne.NewPos(0, 0)
	h.Scroll.Refresh()
	fmt.Printf("ScrollToTop: Scrolled to top\n")
}

func (h *HybridScrollContainer) GetScrollPosition() float32 {
	contentSize := h.terminal.textGrid.Size()
	containerSize := h.Scroll.Size()

	if contentSize.Height <= containerSize.Height {
		return 0.0 // No scrolling possible
	}

	maxScrollY := contentSize.Height - containerSize.Height
	currentScrollY := h.Scroll.Offset.Y

	if maxScrollY <= 0 {
		return 0.0
	}

	percentage := currentScrollY / maxScrollY
	if percentage < 0 {
		percentage = 0
	}
	if percentage > 1 {
		percentage = 1
	}

	return percentage
}

// VIRTUAL SCROLLING INTEGRATION

func (h *HybridScrollContainer) SyncWithVirtualScroll(viewport VirtualScrollState) {
	// Only sync if in normal mode with virtual scrolling
	if h.terminal.screen.IsUsingAlternate() {
		return
	}

	// Calculate scroll position based on viewport
	var scrollPercentage float32 = 0
	if viewport.maxScroll > 0 {
		scrollPercentage = float32(viewport.scrollOffset) / float32(viewport.maxScroll)
	}

	h.setScrollBarPosition(scrollPercentage)
	fmt.Printf("SyncWithVirtualScroll: offset=%d/%d, percentage=%.2f\n",
		viewport.scrollOffset, viewport.maxScroll, scrollPercentage)
}

func (h *HybridScrollContainer) HandleVirtualScrollUpdate(viewport VirtualScrollState) {
	// Use a small delay to ensure UI updates are complete
	go func() {
		time.Sleep(5 * time.Millisecond)
		fyne.Do(func() {
			h.SyncWithVirtualScroll(viewport)
		})
	}()
}

// CONFIGURATION

func (h *HybridScrollContainer) SetScrollBarVisibility(horizontal, vertical bool) {
	if horizontal && vertical {
		h.Scroll.Direction = container.ScrollBoth
	} else if horizontal {
		h.Scroll.Direction = container.ScrollHorizontalOnly
	} else if vertical {
		h.Scroll.Direction = container.ScrollVerticalOnly
	} else {
		h.Scroll.Direction = container.ScrollNone
	}

	h.Scroll.Refresh()
	fmt.Printf("SetScrollBarVisibility: horizontal=%v, vertical=%v\n", horizontal, vertical)
}

func (h *HybridScrollContainer) IsScrollBarVisible() (horizontal, vertical bool) {
	switch h.Scroll.Direction {
	case container.ScrollBoth:
		return true, true
	case container.ScrollHorizontalOnly:
		return true, false
	case container.ScrollVerticalOnly:
		return false, true
	case container.ScrollNone:
		return false, false
	default:
		return false, false
	}
}

func (h *HybridScrollContainer) OptimizeForVirtualScrolling() {
	// Set scroll bars for vertical scrolling only
	h.SetScrollBarVisibility(false, true)

	// Set reasonable minimum size
	h.Scroll.SetMinSize(fyne.NewSize(400, 300))

	fmt.Printf("OptimizeForVirtualScrolling: Configured for virtual scrolling\n")
}

func (h *HybridScrollContainer) UpdateContentSize(width, height float32) {
	newSize := fyne.NewSize(width, height)

	// Only update if size actually changed
	currentSize := h.terminal.textGrid.Size()
	if currentSize.Width != width || currentSize.Height != height {
		h.terminal.textGrid.Resize(newSize)
		h.Scroll.Refresh()

		fmt.Printf("UpdateContentSize: Updated to %.1fx%.1f\n", width, height)
	}
}

// DEBUGGING AND MONITORING

func (h *HybridScrollContainer) DebugScrollState() {
	fmt.Printf("=== SCROLL CONTAINER DEBUG STATE ===\n")

	contentSize := h.terminal.textGrid.Size()
	containerSize := h.Scroll.Size()
	scrollPos := h.GetScrollPosition()
	hVis, vVis := h.IsScrollBarVisible()

	fmt.Printf("Content size: %.1fx%.1f\n", contentSize.Width, contentSize.Height)
	fmt.Printf("Container size: %.1fx%.1f\n", containerSize.Width, containerSize.Height)
	fmt.Printf("Scroll position: %.1f%% (%.1f, %.1f)\n", scrollPos*100, h.Scroll.Offset.X, h.Scroll.Offset.Y)
	fmt.Printf("Scroll bars visible: H=%v, V=%v\n", hVis, vVis)
	fmt.Printf("Terminal mode: alternate=%v, history=%v\n",
		h.terminal.screen.IsUsingAlternate(), h.terminal.IsInHistoryMode())
	fmt.Printf("Terminal history size: %d\n", h.terminal.GetHistorySize())
	fmt.Printf("===================================\n")
}

func (h *HybridScrollContainer) MonitorScrollHealth() {
	ticker := time.NewTicker(5 * time.Second)
	go func() {
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				// Check for common issues
				contentSize := h.terminal.textGrid.Size()
				if contentSize.Width <= 0 || contentSize.Height <= 0 {
					fmt.Printf("WARNING: Invalid content size: %.1fx%.1f\n",
						contentSize.Width, contentSize.Height)
				}

				// Check scroll position sanity
				scrollPos := h.GetScrollPosition()
				if scrollPos < 0 || scrollPos > 1 {
					fmt.Printf("WARNING: Invalid scroll position: %.2f\n", scrollPos)
				}

			case <-h.terminal.GetContext().Done():
				return
			}
		}
	}()
}
