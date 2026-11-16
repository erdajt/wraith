package ui

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func TestCancelInvalidatesPendingApproval(t *testing.T) {
	for _, action := range []string{"approval", "view", "modify", "command"} {
		t.Run(action, func(t *testing.T) {
			a := fixture(t)
			a.approve(proposal{Command: "printf cancelled", Continue: true})
			if action == "view" || action == "modify" {
				a.approvalAction(action)
			}
			if action == "command" {
				a.runCommand("cancel")
			} else {
				a.key(tea.KeyMsg{Type: tea.KeyCtrlC})
			}
			if a.proposal != nil || a.popup != nil || !a.agentStopped {
				t.Fatal("cancel left an actionable approval")
			}
			a.runApproved()
			if len(a.jobs.Snapshot()) != 0 {
				t.Fatal("cancelled approval launched")
			}
		})
	}
}

func TestCancelReachesActiveOperationFromOutputPopup(t *testing.T) {
	a := fixture(t)
	cancelled := false
	a.busy = true
	a.cancel = func() { cancelled = true }
	a.popup = &menu{Kind: "output"}
	a.key(tea.KeyMsg{Type: tea.KeyCtrlC})
	if !cancelled || !a.agentStopped {
		t.Fatal("output popup swallowed cancellation")
	}
	if a.popup == nil {
		t.Fatal("cancel discarded output inspection")
	}
}

func TestCancelInScratchShellDoesNotStopAgent(t *testing.T) {
	a := fixture(t)
	a.focus = "terminal0"
	a.agentStopped = false
	a.key(tea.KeyMsg{Type: tea.KeyCtrlC})
	if a.agentStopped {
		t.Fatal("shell interrupt cancelled agent turn")
	}
}
