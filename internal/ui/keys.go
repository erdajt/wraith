package ui

import (
	tea "github.com/charmbracelet/bubbletea"
	"strings"
)

func (a *App) key(k tea.KeyMsg) tea.Cmd {
	key := k.String()
	if key == a.Config.Keys["quit"] {
		a.Close()
		return tea.Quit
	}
	if key == a.Config.Keys["terminal_overlay"] {
		return a.toggleTerminal(0)
	}
	if key == a.Config.Keys["terminal_split"] {
		return a.toggleTerminal(1)
	}
	if strings.HasPrefix(a.focus, "terminal") {
		return a.terminalKey(k)
	}
	text := ""
	if k.Type == tea.KeyRunes {
		text = string(k.Runes)
	}
	a.Prompt.Key(key, text)
	return nil
}
