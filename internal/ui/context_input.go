package ui

import (
	"strconv"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/erdajt/wraith/internal/vim"
)

func (a *App) openContextInput() {
	b := vim.New()
	b.Set(strconv.Itoa(a.Config.Context))
	a.popup = &menu{Kind: "context-input", Title: "context capacity · enter saves · esc cancels", Input: b}
}

func (a *App) contextInput(k tea.KeyMsg) tea.Cmd {
	if k.String() == "esc" {
		a.settingsCategory("model")
		return nil
	}
	if k.String() == "enter" {
		value := a.popup.Input.String()
		n, err := strconv.Atoi(value)
		if err != nil || n < 1 {
			a.notice = "context must be a positive integer"
			return nil
		}
		a.popup = nil
		return a.set("context", value)
	}
	a.popup.Input.Key(k.String(), string(k.Runes))
	return nil
}
