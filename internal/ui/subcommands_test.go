package ui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func TestDottedCommandsRequireCompleteParent(t *testing.T) {
	for _, query := range []string{"", "sett", "settings"} {
		for _, cmd := range matchingCommands(query) {
			if strings.Contains(cmd.Name, ".") {
				t.Fatalf("early subcommand for %q", query)
			}
		}
	}
	if len(matchingCommands("sett.")) != 0 || len(matchingCommands("models.")) != 0 {
		t.Fatal("invalid parent exposed children")
	}
	choices := matchingCommands("settings.a")
	if len(choices) != 1 || choices[0].Name != "settings.appearance" {
		t.Fatal("category completion missing")
	}
	a := fixture(t)
	a.Prompt.Set("/settings.a")
	a.key(tea.KeyMsg{Type: tea.KeyEnter})
	if a.popup == nil || a.popup.Title != "settings · appearance" {
		t.Fatal("dotted command did not open category")
	}
}

func TestTabOpensAvailableSubcommands(t *testing.T) {
	a := fixture(t)
	a.Prompt.Set("/settings")
	a.key(tea.KeyMsg{Type: tea.KeyTab})
	choices, _, visible := a.completions()
	if a.Prompt.String() != "/settings." || !visible || len(choices) != 6 {
		t.Fatal("tab did not expose categories")
	}
}
