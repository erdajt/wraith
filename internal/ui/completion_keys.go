package ui

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/erdajt/wraith/internal/vim"
)

func (a *App) slashKey(k tea.KeyMsg) (tea.Cmd, bool) {
	text, active := a.slashDraft()
	if !active {
		return nil, false
	}
	choices, index, visible := a.completions()
	switch k.String() {
	case "up", "down":
		if !visible || len(choices) == 0 {
			return nil, false
		}
		if k.String() == "up" {
			index = max(0, index-1)
		} else {
			index = min(len(choices)-1, index+1)
		}
		a.completionQuery = text
		a.completionIndex = index
		return nil, true
	case "enter", "tab":
		if visible {
			if len(choices) == 0 {
				a.notice = "no matching command · esc returns to normal mode"
				return nil, true
			}
			choice := choices[index]
			if choice.NeedsArgument || k.String() == "tab" {
				a.Prompt.Set("/" + choice.Name + " ")
				a.completionIndex = 0
				a.notice = choice.Description + " · enter runs the command"
				return nil, true
			}
			text = choice.Name
		} else if k.String() != "enter" {
			return nil, false
		}
		name, _, _ := strings.Cut(strings.TrimSpace(text), " ")
		matches := matchingCommands(name)
		if len(matches) == 0 || matches[0].Name != name {
			a.notice = "unknown command · edit the command name or press esc"
			return nil, true
		}
		a.Prompt = vim.New()
		a.completionQuery = ""
		a.completionIndex = 0
		return a.runCommand(text), true
	}
	return nil, false
}
