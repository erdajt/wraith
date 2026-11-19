package ui

import "strings"

// surfaceFrame draws a plugin pane using the same border, cursor and theme as native views.
func (a *App) surfaceFrame(view *surfaceView, width, height int, p palette, focused bool) []string {
	if width <= 0 || height <= 0 {
		return nil
	}
	buffer := a.surfaceBuffers[view.window.BufferID]
	border := p.Border
	if focused {
		border = p.Accent
	}
	title := view.window.Title
	if title == "" {
		title = buffer.Name
	}
	if title == "" {
		title = "buffer"
	}
	frame := []string{base(p, ink(border, fit("╭─ "+title+" "+strings.Repeat("─", width), max(0, width-1))+"╮"), width)}
	if height == 1 {
		return frame
	}
	for _, line := range renderSurfaceBuffer(buffer, view.reader, max(0, width-2), max(0, height-2), p, focused) {
		frame = append(frame, base(p, ink(border, "│")+line+ink(border, "│"), width))
	}
	frame = append(frame, base(p, ink(border, "╰"+strings.Repeat("─", max(0, width-2))+"╯"), width))
	return frame
}
