package ui

import (
	"strconv"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/erdajt/wraith/internal/vim"
)

// openContextInput opens an editable context-capacity field initialized to the current limit.
func (a *App) openContextInput() {
	b := vim.New()
	b.Set(strconv.Itoa(a.contextLimit()))
	a.popup = &menu{Kind: "context-input", Title: "context capacity", Input: b}
}

// contextInput handles editing and validation in the context-capacity popup.
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
		if value != "auto" && (err != nil || n < 1) {
			a.notice = "context must be a positive integer or auto"
			return nil
		}
		a.popup = nil
		return a.set("context", value)
	}

	a.popup.Input.Key(k.String(), string(k.Runes))
	return nil
}
