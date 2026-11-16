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
	case "run-hidden":
		a.proposal.HideOutput = true
		return a.runApproved()
	case "edit":
		return a.edit("run", "sh", a.proposal.Resolved, -1)
	case "view":
		reason, _ := a.proposal.Variables.Expand(a.proposal.Reason)
		a.showText("command-preview", "full command · esc returns to approval", a.proposal.Resolved+"\n\nreason: "+reason)
	case "modify":
		a.openModification()
	case "cancel":
		a.declineCommand()
	}
	return nil
}

func (a *App) declineCommand() {
	if a.proposal != nil && a.proposal.Continue {
		a.stopAgent()
		a.Session.Messages = append(a.Session.Messages, session.Message{Role: "user", Content: "the requested command was declined and was not executed. do not repeat it unless asked."})
		a.persist("command declined")
	}
	a.proposal = nil
	a.notice = "command declined"
}

func (a *App) stopAgent() {
	a.streamStopped = true
	a.pendingTool = nil
	a.agentStopped = true
	a.pendingAgent = false
	if a.jobs != nil {
		a.jobs.CancelTurn(a.Session.ID, a.generation)
	}
}
