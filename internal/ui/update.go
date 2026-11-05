package ui

import (
	"fmt"
	tea "github.com/charmbracelet/bubbletea"
)

func (a *App) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch m := msg.(type) {
	case tea.WindowSizeMsg:
		a.width = m.Width
		a.height = m.Height
		a.resizePanes()
	case tick:
		a.frame++
		return a, a.ticker()
	case infoMsg:
		if m.Model == a.Session.Model {
			a.Info = m.Info
			a.connected = m.Err == nil
			if m.Err != nil {
				a.notice = m.Err.Error()
			}
		}
	case tea.KeyMsg:
		return a, a.key(m)
	case terminalMsg:

		p := a.panes[m.Index]
		if p == nil || p.PTY != m.PTY {
			break
		}
		if !m.Open || m.Event.Err != nil {
			p.Err = "shell exited · /terminal-restart " + fmt.Sprint(m.Index+1)
			break
		}
		if reply := p.Screen.Feed(m.Event.Data); len(reply) > 0 {
			_ = p.PTY.Write(reply)
		}
		return a, a.readTerminal(m.Index)

	case streamMsg:
		return a, a.receive(m)
	case progressMsg:
		a.phase = string(m)
		return a, a.next()
	case compactMsg:
		return a, a.finishCompact(m)
	case editedMsg:
		return a, a.finishEdit(m)
	}
	return a, nil
}
