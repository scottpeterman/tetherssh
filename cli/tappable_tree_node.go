// tappable_tree_node.go - Right-click support for tree nodes
package main

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/widget"
)

// TappableBox is a container that supports secondary (right-click) tap
type TappableBox struct {
	widget.BaseWidget
	content          fyne.CanvasObject
	onSecondaryTap   func(pos fyne.Position)
}

// NewTappableBox creates a new tappable container
func NewTappableBox(content fyne.CanvasObject, onSecondaryTap func(pos fyne.Position)) *TappableBox {
	t := &TappableBox{
		content:        content,
		onSecondaryTap: onSecondaryTap,
	}
	t.ExtendBaseWidget(t)
	return t
}

// CreateRenderer implements fyne.Widget
func (t *TappableBox) CreateRenderer() fyne.WidgetRenderer {
	return &tappableBoxRenderer{box: t}
}

// TappedSecondary handles right-click
func (t *TappableBox) TappedSecondary(e *fyne.PointEvent) {
	if t.onSecondaryTap != nil {
		t.onSecondaryTap(e.AbsolutePosition)
	}
}

// Tapped required by Tappable interface but we let clicks pass through
func (t *TappableBox) Tapped(*fyne.PointEvent) {
	// Let the tree handle normal clicks
}

type tappableBoxRenderer struct {
	box *TappableBox
}

func (r *tappableBoxRenderer) Layout(size fyne.Size) {
	r.box.content.Resize(size)
	r.box.content.Move(fyne.NewPos(0, 0))
}

func (r *tappableBoxRenderer) MinSize() fyne.Size {
	return r.box.content.MinSize()
}

func (r *tappableBoxRenderer) Refresh() {
	r.box.content.Refresh()
}

func (r *tappableBoxRenderer) Objects() []fyne.CanvasObject {
	return []fyne.CanvasObject{r.box.content}
}

func (r *tappableBoxRenderer) Destroy() {}