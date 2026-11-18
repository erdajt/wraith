package ui

import (
	"testing"

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
