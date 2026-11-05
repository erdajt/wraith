package ui

import (
	"strconv"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/erdajt/wraith/internal/vim"
)

func (a *App) openContextInput() {
	b := vim.New()
	b.Set(strconv.Itoa(a.Config.Context))
	a.popup = &menu{Kind: "context-input", Title: "context capacity · enter saves · esc normal/cancel · ctrl+u clears", Input: b}
}

func (a *App) contextInput(k tea.KeyMsg) tea.Cmd {
	if k.String() == "esc" && a.popup.Input.Mode == "normal" {
		a.settingsCategory("context")
		return nil
	}
	if k.String() == "ctrl+u" {
		a.popup.Input.Set("")
		return nil
	}
	if k.String() == "enter" || k.String() == a.Config.Keys["send"] {
		value := strings.TrimSpace(a.popup.Input.String())
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
