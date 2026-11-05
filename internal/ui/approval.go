package ui

import tea "github.com/charmbracelet/bubbletea"

func (a *App) approvalAction(action string) tea.Cmd {
	if a.proposal == nil {
		return nil
	}
	switch action {
	case "run":
		return a.runApproved()
	case "edit":
		return a.edit("run", a.proposal.Language, a.proposal.Text, -1)
	case "view":
		a.showText("command-preview", "full command · esc returns to approval", a.proposal.Command)
	case "cancel":
		a.proposal = nil
		a.notice = "command declined"
	}
	return nil
}
