package ui

import (
	tea "github.com/charmbracelet/bubbletea"
	"github.com/erdajt/wraith/internal/session"
)

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
		a.showText("command-preview", "full command · esc returns to approval", a.proposal.Command+"\n\nreason: "+a.proposal.Reason)
	case "modify":
		a.openModification()
	case "cancel":
		a.declineCommand()
	}
	return nil
}

func (a *App) declineCommand() {
	if a.proposal != nil && a.proposal.Continue {
		a.Session.Messages = append(a.Session.Messages, session.Message{Role: "user", Content: "the requested command was declined and was not executed. do not repeat it unless asked."})
		a.persist("command declined")
	}
	a.proposal = nil
	a.notice = "command declined"
}
