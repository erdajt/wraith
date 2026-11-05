package ui

import (
	tea "github.com/charmbracelet/bubbletea"
	"github.com/erdajt/wraith/internal/session"
	"testing"
)

func TestResumePickerSupportsArrowsVimAndSearch(t *testing.T) {
	a := fixture(t)
	for _, title := range []string{"alpha review", "beta review", "gamma notes"} {
		s := session.New("test-model")
		s.Title = title
		if err := a.Store.Save(s); err != nil {
			t.Fatal(err)
		}
	}
	a.OpenSessions()
	a.key(tea.KeyMsg{Type: tea.KeyDown})
	if a.popup.Index != 1 {
		t.Fatal("down arrow ignored")
	}
	a.key(key("j"))
	a.key(key("k"))
	a.key(tea.KeyMsg{Type: tea.KeyUp})
	if a.popup.Index != 0 {
		t.Fatal("vim or arrow movement failed")
	}
	a.key(key("/"))
	a.key(key("beta"))
	a.key(tea.KeyMsg{Type: tea.KeyEnter})
	if len(a.popup.Items) != 1 {
		t.Fatal("session filter failed")
	}
	a.key(tea.KeyMsg{Type: tea.KeyEnter})
	if a.Session.Title != "beta review" {
		t.Fatal("wrong session resumed")
	}
}

func TestApprovalEscapeRecordsDeclineWithoutExecution(t *testing.T) {
	a := fixture(t)
	a.approve(proposal{Command: "printf x", Text: "printf x", Continue: true})
	a.key(tea.KeyMsg{Type: tea.KeyEsc})
	if len(a.Session.Messages) != 1 || a.proposal != nil {
		t.Fatal("decline was not recorded")
	}
	if a.busy {
		t.Fatal("decline executed a command")
	}
}
