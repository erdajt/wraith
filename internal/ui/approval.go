package ui

import (
	"fmt"
	"strconv"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/erdajt/wraith/internal/session"
)

// approvalAction dispatches an explicit command decision, including hidden-output execution.
func (a *App) approvalAction(action string) tea.Cmd {
	if a.proposal == nil {
		return nil
	}

	if a.proposal.Extension != nil {
		return a.extensionAction(action)
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
		var review strings.Builder
		fmt.Fprintf(&review, "%s\n\nreason: %s\ndirectory: %s\nshell: %s\ntimeout: %s\nbackground: %t\n\nenvironment (frozen):\n", a.proposal.Resolved, reason, a.proposal.launch.Directory, a.proposal.launch.Shell, a.proposal.timeout, a.proposal.Background)
		for _, entry := range a.proposal.launch.Environment {
			review.WriteString(strconv.Quote(entry) + "\n")
		}
		a.showText("command-preview", "full command", review.String())
		a.popup.Hints = "esc returns to approval"
	case "modify":
		a.openModification()
	case "cancel":
		a.declineCommand()
	}
	return nil
}

// declineCommand records denial and stops the requesting agent turn when applicable.
func (a *App) declineCommand() {
	if a.policyCancel != nil {
		a.policyCancel()
		a.policyCancel = nil
	}
	if a.proposal != nil && a.proposal.Continue {
		a.stopAgent()
		a.Session.Messages = append(a.Session.Messages, session.Message{Role: "user", Content: "the requested command was declined and was not executed. do not repeat it unless asked."})
		a.persist("command declined")
	}

	a.proposal = nil
	a.notice = "command declined"
}

// stopAgent clears pending actions and cancels background jobs requested by the current turn.
func (a *App) stopAgent() {
	a.cancelOutputRead()
	if a.policyCancel != nil {
		a.policyCancel()
		a.policyCancel = nil
	}
	a.proposal = nil
	if a.popup != nil {
		switch a.popup.Kind {
		case "approval", "command-preview", "modify-command", "command-policy":
			a.popup = nil
		}
	}

	a.streamStopped = true
	a.pendingTool = nil
	a.agentStopped = true
	a.pendingAgent = false
	if a.jobs != nil {
		a.jobs.CancelTurn(a.Session.ID, a.generation)
	}
}
