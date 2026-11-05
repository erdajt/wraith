package ui

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func TestInputHistoryBrowsesPromptsCommandsAndRestoresDraft(t *testing.T) {
	a := fixture(t)
	a.rememberInput("earlier prompt")
	a.runCommand("help")
	a.popup = nil
	a.Prompt.Set("unfinished draft")
	a.Prompt.Pos = 4
	a.key(tea.KeyMsg{Type: tea.KeyUp})
	if a.Prompt.String() != "/help" {
		t.Fatal("last command missing")
	}
	a.key(tea.KeyMsg{Type: tea.KeyUp})
	if a.Prompt.String() != "earlier prompt" {
		t.Fatal("prompt history lost to slash completion")
	}
	a.key(tea.KeyMsg{Type: tea.KeyDown})
	a.key(tea.KeyMsg{Type: tea.KeyDown})
	if a.Prompt.String() != "unfinished draft" || a.Prompt.Pos != 4 {
		t.Fatal("history discarded draft")
	}
	entries, err := a.Store.Inputs(a.Directory)
	if err != nil || len(entries) != 2 {
		t.Fatalf("history persistence: %v %v", entries, err)
	}
	other, err := a.Store.Inputs(t.TempDir())
	if err != nil || len(other) != 0 {
		t.Fatal("history leaked across directories")
	}
}
