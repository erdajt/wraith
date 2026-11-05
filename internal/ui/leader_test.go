package ui

import (
	tea "github.com/charmbracelet/bubbletea"
	"testing"
)

func TestLeaderSequencesAndSearch(t *testing.T) {
	a := fixture(t)
	a.Prompt.Mode = "normal"
	a.key(tea.KeyMsg{Type: tea.KeySpace, Runes: []rune{' '}})
	if a.popup == nil || a.popup.Kind != "leader" {
		t.Fatal("leader did not open")
	}
	a.key(key("c"))
	if a.popup.Prefix != "c" {
		t.Fatal("prefix not selected")
	}
	a.key(key("s"))
	if a.popup.Kind != "sessions" {
		t.Fatal("nested binding not invoked")
	}
	a.popup = nil
	a.Config.Leader = ","
	a.Config.Bindings = map[string]string{"x y": "settings", "b": "blocks"}
	a.key(key(",xy"))
	if a.popup == nil || a.popup.Kind != "settings" {
		t.Fatal("custom batched sequence failed")
	}
	a.key(key("/"))
	a.key(key("scratch"))
	a.key(tea.KeyMsg{Type: tea.KeyEnter})
	if len(a.popup.Items) != 2 {
		t.Fatalf("search did not filter settings: %v", a.popup.Items)
	}
	a.key(tea.KeyMsg{Type: tea.KeyEsc})
	if len(a.popup.Items) <= 2 {
		t.Fatal("search reset did not restore settings")
	}
}

func TestLeaderDoesNotConsumeInsertSpaces(t *testing.T) {
	a := fixture(t)
	a.key(tea.KeyMsg{Type: tea.KeySpace, Runes: []rune{' '}})
	if a.popup != nil || a.Prompt.String() != " " {
		t.Fatal("leader consumed insert space")
	}
}
