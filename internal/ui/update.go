package ui

import (
	tea "github.com/charmbracelet/bubbletea"
)

func (a *App) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch m := msg.(type) {
	case tea.WindowSizeMsg:
		a.width = m.Width
		a.height = m.Height
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
	}
	return a, nil
}
