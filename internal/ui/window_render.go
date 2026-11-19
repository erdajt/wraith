package ui

import (
	"strings"

	"github.com/charmbracelet/x/ansi"
)

// drawConversations renders each leaf with its own viewport, search and selection.
func (a *App) drawConversations(rows []string, p palette) {
	places := a.windowPlaces()
	for _, place := range places {
		view := a.windows[place.id]
		focused := place.id == a.windowID && a.focus == "chat"
		if view.surface != nil {
			a.overlay(rows, a.surfaceFrame(view.surface, place.width, place.height, p, focused), place.x, place.y)
			continue
		}
		lines := a.windowTranscript(view, max(10, place.width-5), focused)
		start := view.viewportStart(place.height)
		frame := make([]string, place.height)
		for y := range frame {
			content := ""
			if start+y < len(lines) {
				query := view.search
				if focused && a.searchMode {
					query = a.command
				}
				content = highlightMatches(lines[start+y].Text, query, p)
				content = view.selectionLine(content, start+y, p, lines)
				if focused && start+y == view.chatCursor {
					content = cursorText(content, view.chatColumn, p)
				}
			}
			frame[y] = base(p, "  "+content, place.width)
		}
		if len(a.Session.Messages) == 0 && !a.busy {
			welcome := a.welcome(place.width, place.height, p)
			y := max(0, (place.height-len(welcome))/2)
			for _, line := range welcome {
				if y >= len(frame) {
					break
				}
				pad := max(0, (place.width-ansi.StringWidth(line))/2)
				frame[y] = base(p, strings.Repeat(" ", pad)+line, place.width)
				y++
			}
		}
		a.overlay(rows, frame, place.x, place.y)

		border := p.Border
		if focused {
			border = p.Accent
		}
		if place.x > 0 {
			for y := place.y; y < place.y+place.height; y++ {
				a.overlay(rows, []string{ink(border, "│")}, place.x-1, y)
			}
		}
		if place.y > 3 {
			a.overlay(rows, []string{ink(border, strings.Repeat("─", place.width))}, place.x, place.y-1)
		}
	}
}
