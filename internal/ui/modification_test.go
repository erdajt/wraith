package ui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func TestEmptyModificationPreparesNextPrompt(t *testing.T) {
	a := fixture(t)
	a.approve(proposal{Command: "printf original", Text: "printf original", Reason: "inspect output", Continue: true})
	a.approvalAction("modify")
	a.menuKey(tea.KeyMsg{Type: tea.KeyEnter})
	if a.busy || a.popup != nil || a.proposal != nil || a.focus != "prompt" {
		t.Fatal("empty modification sent or retained approval")
	}
	if !strings.HasSuffix(a.Prompt.String(), "modify: ") || !strings.Contains(a.Prompt.String(), "printf original") || !strings.Contains(a.Prompt.String(), "reason: inspect output") {
		t.Fatal("modification draft lost command or reason")
	}
}

func TestInlineModificationEntersPromptWithoutRunningCommand(t *testing.T) {
	a := fixture(t)
	a.loading = true
	a.Prompt.Set("retain existing draft")
	a.approve(proposal{Command: "printf original", Reason: "inspect output", Continue: true})
	a.approvalAction("modify")
	a.menuKey(key("use json output"))
	a.menuKey(tea.KeyMsg{Type: tea.KeyEnter})
	if a.executing || !strings.Contains(a.Prompt.String(), "modify: use json output") || !strings.Contains(a.Prompt.String(), "retain existing draft") {
		t.Fatal("modification did not preserve draft")
	}
	if len(a.Session.Messages) != 0 {
		t.Fatal("loading modification was sent early")
	}
}
