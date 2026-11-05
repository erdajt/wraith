package ui

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/erdajt/wraith/internal/vim"
)

func (a *App) openModification() {
	a.popup = &menu{Kind: "modify-command", Title: "modify command · enter sends · empty defers · esc returns", Input: vim.New()}
}

func (a *App) modificationKey(k tea.KeyMsg) tea.Cmd {
	if a.proposal == nil {
		a.popup = nil
		return nil
	}
	if k.String() == "esc" && a.popup.Input.Mode == "normal" {
		return a.approve(*a.proposal)
	}
	if k.String() == "enter" || k.String() == a.Config.Keys["send"] {
		instructions := strings.TrimSpace(a.popup.Input.String())
		p := *a.proposal
		draft := a.Prompt.String()
		text := "command not executed:\n" + p.Command + "\nreason: " + p.Reason + "\n\nmodify: " + instructions
		if draft != "" {
			text += "\n" + draft
		}
		a.popup, a.proposal = nil, nil
		a.Prompt.Set(text)
		a.focus = "prompt"
		if instructions == "" {
			a.notice = "describe the command changes after modify:"
			return nil
		}
		return a.submit()
	}
	a.popup.Input.Key(k.String(), string(k.Runes))
	return nil
}
