package ui

import (
	tea "github.com/charmbracelet/bubbletea"
)

func (a *App) key(k tea.KeyMsg) tea.Cmd {
	if k.String() == a.Config.Keys["quit"] {
		a.Close()
		return tea.Quit
	}
	text := ""
	if k.Type == tea.KeyRunes {
		text = string(k.Runes)
	}
	a.Prompt.Key(k.String(), text)
	return nil
}
