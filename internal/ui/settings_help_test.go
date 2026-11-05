package ui

import (
	tea "github.com/charmbracelet/bubbletea"
	"strings"
	"testing"
)

func TestSettingsHelpReturnsToSelection(t *testing.T) {
	a := fixture(t)
	a.settingsCategory("execution")
	a.popup.Index = 2
	previous := a.popup
	a.key(key("?"))
	if a.popup.Kind != "setting-help" || !strings.Contains(a.View(), "Maximum approved") {
		t.Fatal("detailed help missing")
	}
	a.key(tea.KeyMsg{Type: tea.KeyEsc})
	if a.popup != previous || a.popup.Index != 2 {
		t.Fatal("help lost selection")
	}
	a.Config.Keys["setting_help"] = "K"
	a.key(key("K"))
	if a.popup.Kind != "setting-help" {
		t.Fatal("custom help key ignored")
	}
}

func TestEverySettingsRowHasDetailedHelp(t *testing.T) {
	a := fixture(t)
	for _, category := range []string{"model", "execution", "context", "appearance", "terminals", "keys"} {
		a.settingsCategory(category)
		for _, it := range a.popup.Items {
			if len(settingDescriptions[it.Value]) < 60 {
				t.Errorf("missing detailed help for %s", it.Value)
			}
		}
	}
}
