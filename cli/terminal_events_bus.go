package main

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/driver/desktop"
)

// EventBus forwards events between components
type EventBus struct {
	onMouseDown []func(*desktop.MouseEvent)
	onMouseUp   []func(*desktop.MouseEvent)
	onDragged   []func(*fyne.DragEvent)
}

func NewEventBus() *EventBus {
	return &EventBus{
		onMouseDown: make([]func(*desktop.MouseEvent), 0),
		onMouseUp:   make([]func(*desktop.MouseEvent), 0),
		onDragged:   make([]func(*fyne.DragEvent), 0),
	}
}

func (e *EventBus) ConnectMouseDown(handler func(*desktop.MouseEvent)) {
	e.onMouseDown = append(e.onMouseDown, handler)
}

func (e *EventBus) ConnectMouseUp(handler func(*desktop.MouseEvent)) {
	e.onMouseUp = append(e.onMouseUp, handler)
}

func (e *EventBus) ConnectDragged(handler func(*fyne.DragEvent)) {
	e.onDragged = append(e.onDragged, handler)
}

func (e *EventBus) EmitMouseDown(event *desktop.MouseEvent) {
	for _, handler := range e.onMouseDown {
		handler(event)
	}
}

func (e *EventBus) EmitMouseUp(event *desktop.MouseEvent) {
	for _, handler := range e.onMouseUp {
		handler(event)
	}
}

func (e *EventBus) EmitDragged(event *fyne.DragEvent) {
	for _, handler := range e.onDragged {
		handler(event)
	}
}
