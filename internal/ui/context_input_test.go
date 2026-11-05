package ui

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/erdajt/wraith/internal/config"
)

func TestContextEditorSupportsVimAndPersistsArbitraryCapacity(t *testing.T) {
	a := fixture(t)
	a.settingsCategory("context")
	a.key(tea.KeyMsg{Type: tea.KeyEnter})
	if a.popup == nil || a.popup.Kind != "context-input" {
		t.Fatal("capacity selection did not open the editor directly")
	}
	a.key(tea.KeyMsg{Type: tea.KeyEsc})
	if a.popup == nil || a.popup.Input.Mode != "normal" {
		t.Fatal("escape closed the editor instead of entering normal mode")
	}
	for _, k := range []string{"c", "c", "1000000"} {
		a.key(key(k))
	}
	a.key(tea.KeyMsg{Type: tea.KeyEnter})
	if a.Config.Context != 1000000 || a.popup != nil {
		t.Fatalf("context edit failed: %s", a.notice)
	}
	c, err := config.LoadDirectory(a.Paths.Config)
	if err != nil || c.Context != 1000000 {
		t.Fatalf("capacity was not saved: %v", err)
	}
}

func TestContextEditorRejectsInvalidInputAndCancelsWithoutSaving(t *testing.T) {
	a := fixture(t)
	original := a.Config.Context
	a.openContextInput()
	a.key(tea.KeyMsg{Type: tea.KeyCtrlU})
	a.key(key("0"))
	a.key(tea.KeyMsg{Type: tea.KeyEnter})
	if a.popup == nil || a.Config.Context != original {
		t.Fatal("invalid capacity was accepted")
	}
	a.key(tea.KeyMsg{Type: tea.KeyEsc})
	a.key(tea.KeyMsg{Type: tea.KeyEsc})
	if a.popup.Title != "settings · context" || a.Config.Context != original {
		t.Fatal("cancel changed the capacity or lost the context category")
	}
}
