package ui

import (
	tea "github.com/charmbracelet/bubbletea"
	"github.com/erdajt/wraith/internal/session"
	"testing"
)

func TestUserEditForksOnlyAfterConfirmation(t *testing.T) {
	a := fixture(t)
	a.Session.Messages = []session.Message{{Role: "user", Content: "first"}, {Role: "assistant", Content: "reply"}, {Role: "user", Content: "revise me"}, {Role: "assistant", Content: "newer reply"}}
	a.selected = 2
	original := a.Session.ID
	a.editResponse()
	if a.popup == nil || a.popup.Kind != "fork" || a.Session.ID != original {
		t.Fatal("edit skipped confirmation")
	}
	a.key(tea.KeyMsg{Type: tea.KeyEsc})
	if len(a.Session.Messages) != 4 || a.Prompt.String() != "" {
		t.Fatal("decline changed conversation")
	}
	a.editResponse()
	a.key(tea.KeyMsg{Type: tea.KeyEnter})
	if a.Session.ID == original || len(a.Session.Messages) != 2 || a.Prompt.String() != "revise me" || a.focus != "prompt" || a.Prompt.Mode != "normal" {
		t.Fatal("confirmed edit did not restore draft at fork point")
	}
	saved, err := a.Store.Load(original)
	if err != nil || len(saved.Messages) != 4 {
		t.Fatal("original branch lost")
	}
	if a.busy {
		t.Fatal("fork sent the prompt before editing")
	}
}
