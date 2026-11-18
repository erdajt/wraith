package ui

import (
	tea "github.com/charmbracelet/bubbletea"
	"github.com/erdajt/wraith/internal/keymap"
	"testing"
)

func TestPendingInsertMappingPreservesTextBeforePaste(t *testing.T) {
	a := fixture(t)
	a.Config.Keymaps = []keymap.Binding{{Mode: "prompt.insert", Keys: "jk", Action: "noop"}}
	var err error
	a.keymaps, err = a.buildKeymaps(a.Config)
	if err != nil {
		t.Fatal(err)
	}
	a.key(key("j"))
	a.key(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("pasted"), Paste: true})
	if got := a.Prompt.String(); got != "jpasted" {
		t.Fatalf("typed prefix lost: %q", got)
	}
}

func TestPromptSearchModeCanBeMapped(t *testing.T) {
	a := fixture(t)
	a.Config.Keymaps = []keymap.Binding{{Mode: "search", Keys: "x", Action: "noop"}}
	var err error
	a.keymaps, err = a.buildKeymaps(a.Config)
	if err != nil {
		t.Fatal(err)
	}
	a.Prompt.Key("esc", "")
	a.Prompt.Key("/", "")
	if mode := a.inputMode(); mode != "search" {
		t.Fatalf("search input has unregistrable mode %q", mode)
	}
}

func TestApprovalCannotBeReplacedByUnrelatedMappedCommand(t *testing.T) {
	a := fixture(t)
	a.Config.Keymaps = []keymap.Binding{{Mode: "*", Keys: "<F1>", Action: "command.settings"}}
	var err error
	a.keymaps, err = a.buildKeymaps(a.Config)
	if err != nil {
		t.Fatal(err)
	}
	a.proposal = &proposal{Command: "printf test", Resolved: "printf test"}
	a.popup = &menu{Kind: "approval", Title: "approve"}
	a.key(tea.KeyMsg{Type: tea.KeyF1})
	if a.popup == nil || a.popup.Kind != "approval" {
		t.Fatalf("approval overwritten by %+v with pending proposal %v", a.popup, a.proposal != nil)
	}
}

func TestBatchedKeysWaitForEditorHandoff(t *testing.T) {
	a := fixture(t)
	a.Config.Keymaps = []keymap.Binding{{Mode: "prompt.insert", Keys: "x", Action: "editor.open"}}
	var err error
	a.keymaps, err = a.buildKeymaps(a.Config)
	if err != nil {
		t.Fatal(err)
	}
	a.Prompt.Set("draft")
	cmd := a.key(key("xy"))
	if cmd == nil {
		t.Fatal("editor handoff was not scheduled")
	}
	if a.editing && a.Prompt.String() != "draft" {
		t.Fatalf("remaining batched key changed draft during queued editor handoff: %q", a.Prompt.String())
	}
}

func TestLegacyBindingCollisionIsDeterministic(t *testing.T) {
	a := fixture(t)
	a.Config.Keys["send"] = a.Config.Keys["editor"]
	seen := map[string]bool{}
	for i := 0; i < 100; i++ {
		r, err := a.buildKeymaps(a.Config)
		if err != nil {
			return
		}
		seen[r.Lookup("prompt.insert", []string{a.Config.Keys["editor"]}).Action] = true
	}
	if len(seen) > 1 {
		t.Fatalf("same configured chord selects different actions across reloads: %v", seen)
	}
}

func TestUnmatchedLeaderPrefixRetainsDefaultBinding(t *testing.T) {
	a := fixture(t)
	a.Prompt.Mode = "normal"
	a.Config.Keymaps = []keymap.Binding{{Mode: "prompt.normal", Keys: "<Space>sa", Action: "command.settings.appearance"}}
	var err error
	a.keymaps, err = a.buildKeymaps(a.Config)
	if err != nil {
		t.Fatal(err)
	}
	a.key(key(" ca"))
	if a.popup == nil || a.popup.Kind != "archives" {
		t.Fatalf("default leader binding lost after custom prefix: %+v", a.popup)
	}
}
