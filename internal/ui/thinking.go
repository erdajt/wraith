package ui

import (
	"fmt"

	tea "github.com/charmbracelet/bubbletea"
)

func (a *App) thinkingKey(index int) string { return fmt.Sprintf("%s:%d", a.Session.ID, index) }
func (a *App) thinkingOpen(index int) bool {
	if expanded, ok := a.thinkingExpanded[a.thinkingKey(index)]; ok {
		return expanded
	}
	return a.showThinking
}

func (a *App) toggleThinking(index int) {
	lines := a.transcript(max(10, a.mainWidth()-5))
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

func (a *App) mouse(m tea.MouseMsg) tea.Cmd {
	if !a.Config.Appearance.Mouse || a.popup != nil || m.X < 0 || m.X >= a.mainWidth() || m.Y < 3 || m.Y >= 3+a.bodyHeight() {
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
	lines := a.transcript(max(10, a.mainWidth()-5))
	start := min(a.offset, max(0, len(lines)-a.bodyHeight()))
	if a.follow {
		start = max(0, len(lines)-a.bodyHeight())
	}
	index := start + m.Y - 3
	if index >= 0 && index < len(lines) && lines[index].Thinking {
		a.selected = lines[index].Message
		a.focus = "chat"
		a.toggleThinking(a.selected)
	}
	return nil
}
