package ui

import (
	"encoding/json"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/erdajt/wraith/internal/config"
	"github.com/erdajt/wraith/internal/keymap"
)

func TestNamedActionsShareCommandsAndWindowOperations(t *testing.T) {
	a := fixture(t)
	a.width, a.height, a.focus = 120, 40, "chat"
	a.runAction("window.split_vertical", "")
	if len(a.windows) != 2 {
		t.Fatal("named split did not change the workspace")
	}
	a.runAction("command.settings.appearance", "")
	if a.popup == nil || a.popup.Kind != "settings" || a.popup.Title != "settings · appearance" {
		t.Fatal("named command did not use normal command dispatch")
	}
	a.runAction("window.close", "")
	if len(a.windows) != 2 {
		t.Fatal("window action bypassed popup focus")
	}
	a.popup = nil
	a.runAction("window.close", "")
	if len(a.windows) != 1 {
		t.Fatal("named close did not remove the focused window")
	}
}

func TestSettingsValidateAndActivateMappingsBeforePersistence(t *testing.T) {
	a := fixture(t)
	a.width, a.height, a.focus = 120, 40, "chat"
	bindings := []keymap.Binding{{Mode: "chat.normal", Keys: "<C-w>v", Action: "window.split_horizontal"}}
	encoded, err := json.Marshal(bindings)
	if err != nil {
		t.Fatal(err)
	}
	a.set("keymaps", string(encoded))
	a.key(tea.KeyMsg{Type: tea.KeyCtrlW})
	a.key(key("v"))
	if len(a.windows) != 2 || a.windowTree.vertical {
		t.Fatal("setting did not replace the default chord")
	}
	before := a.keymaps
	a.set("keymaps", `[{"mode":"chat.normal","keys":"x","action":"missing"}]`)
	if a.keymaps != before {
		t.Fatal("invalid setting replaced active bindings")
	}
	saved, err := config.Load(config.File(a.Paths.Config))
	if err != nil || len(saved.Keymaps) != 1 || saved.Keymaps[0].Action != "window.split_horizontal" {
		t.Fatalf("invalid setting replaced persisted bindings: %+v, %v", saved.Keymaps, err)
	}
}

func TestInvalidActionCandidateKeepsExistingRegistry(t *testing.T) {
	a := fixture(t)
	before := a.keymaps
	candidate := a.Config
	candidate.Keymaps = []keymap.Binding{{Mode: "chat.normal", Keys: "x", Action: "made.up"}}
	if _, err := a.buildKeymaps(candidate); err == nil || a.keymaps != before {
		t.Fatal("invalid action replaced working mappings")
	}
	candidate.Keymaps[0].Action = "window.left"
	registry, err := a.buildKeymaps(candidate)
	if err != nil || registry.Lookup("chat.normal", []string{"x"}).Action != "window.left" {
		t.Fatalf("valid custom mapping failed: %v", err)
	}
}
