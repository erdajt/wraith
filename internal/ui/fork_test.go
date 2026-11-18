package ui

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/erdajt/wraith/internal/session"
)

func TestUserEditForksOnlyAfterConfirmation(t *testing.T) {
	a := fixture(t)
	a.Session.Messages = []session.Message{{Role: "user", Content: "first"}, {Role: "assistant", Content: "reply"}, {Role: "user", Content: "revise me"}, {Role: "assistant", Content: "newer reply"}}
	a.Session.Draft = "unrelated saved draft"
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
	if err != nil || len(saved.Messages) != 4 || saved.Draft != "unrelated saved draft" {
		t.Fatal("original branch lost")
	}

	branch, err := a.Store.Load(a.Session.ID)
	if err != nil || branch.Draft != "revise me" {
		t.Fatal("fork did not persist its own editable draft")
	}
	a.Prompt.Set("")
	a.resume(branch.ID)
	if a.Prompt.String() != "revise me" {
		t.Fatal("resuming fork lost the selected prompt")
	}

	if a.busy {
		t.Fatal("fork sent the prompt before editing")
	}
}
