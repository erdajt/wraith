package ui

import (
	"fmt"

	tea "github.com/charmbracelet/bubbletea"
)

// thinkingKey identifies per-message thinking expansion state within a session.
func (a *App) thinkingKey(index int) string { return fmt.Sprintf("%s:%d", a.Session.ID, index) }

// thinkingOpen resolves a message's expansion override or the default thinking visibility.
func (a *App) thinkingOpen(index int) bool {
	if expanded, ok := a.thinkingExpanded[a.thinkingKey(index)]; ok {
		return expanded
	}
	return a.showThinking
}

// toggleThinking toggles thinking for a message and keeps its heading in view.
func (a *App) toggleThinking(index int) {
	lines := a.transcript(a.chatWidth())
	for n, line := range lines {
		if line.Message == index && line.Thinking {
			if a.thinkingExpanded == nil {
				a.thinkingExpanded = map[string]bool{}
			}
			a.thinkingExpanded[a.thinkingKey(index)] = !a.thinkingOpen(index)
			a.offset = max(0, n-1)
			a.follow = false
			return
		}
	}
	a.notice = "selected response has no thinking output"
}

// mouse toggles a clicked thinking heading when no popup or terminal covers it.
func (a *App) mouse(m tea.MouseMsg) tea.Cmd {
	if !a.Config.Appearance.Mouse || a.popup != nil || a.pluginDialog != nil || m.X < 0 || m.X >= a.mainWidth() || m.Y < 3 || m.Y >= 3+a.bodyHeight() {
		return nil
	}
	if p := a.panes[0]; p != nil && p.Visible {
		w, h := a.paneSize(0)
		x, y := (a.width-w)/2, (a.height-h)/2
		if m.X >= x && m.X < x+w && m.Y >= y && m.Y < y+h {
			return nil
		}
	}

	if m.Action != tea.MouseActionPress || m.Button != tea.MouseButtonLeft {
		return nil
	}

	places := a.windowPlaces()
	for i := len(places) - 1; i >= 0; i-- {
		place := places[i]
		if m.X < place.x || m.X >= place.x+place.width || m.Y < place.y || m.Y >= place.y+place.height {
			continue
		}
		a.focusWindow(place.id)
		if a.windows[place.id].surface != nil {
			a.surfaceMouse(place, m.X, m.Y)
			return nil
		}
		lines := a.transcript(a.chatWidth())
		start := a.viewportStart(place.height)
		index := start + m.Y - place.y
		if index < len(lines) {
			a.chatCursor, a.chatColumn = index, max(0, m.X-place.x-2)
			a.selected = lines[index].Message
			a.revealCursor(len(lines))
			if lines[index].Thinking {
				a.toggleThinking(a.selected)
			}
		}
		break
	}
	return nil
}
