package ui

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/erdajt/wraith/internal/keymap"
	"github.com/erdajt/wraith/internal/session"
)

func TestMappingsConsumeChordsAndPreserveVimCounts(t *testing.T) {
	a := fixture(t)
	a.width, a.height, a.focus = 120, 40, "chat"
	a.Session.Messages = []session.Message{{Role: "assistant", Content: "one\ntwo\nthree\nfour\nfive"}}
	a.Config.Keymaps = []keymap.Binding{{Mode: "chat.normal", Keys: "j", Input: "j"}, {Mode: "chat.normal", Keys: "s", Action: "window.split_vertical"}, {Mode: "chat.normal", Keys: "<C-h>", Action: "window.left"}}
	var err error
	a.keymaps, err = a.buildKeymaps(a.Config)
	if err != nil {
		t.Fatal(err)
	}
	a.ensureChatCursor()
	a.chatCursor, a.offset = 0, 0
	a.key(key("2j"))
	if len(a.windows) != 1 || a.chatCursor != 2 {
		t.Fatal("normal mapping intercepted a counted vim motion")
	}
	a.key(key("s"))
	if len(a.windows) != 2 {
		t.Fatal("normal mapping did not run")
	}
	a.key(tea.KeyMsg{Type: tea.KeyCtrlH})
	if a.windowID != 1 {
		t.Fatal("control navigation remap did not focus left")
	}
}

func TestAmbiguousMappingTimeoutAndFailedPrefixReplayOnce(t *testing.T) {
	a := fixture(t)
	a.Config.Keymaps = []keymap.Binding{{Mode: "prompt.insert", Keys: "ab", Action: "command.help"}, {Mode: "prompt.insert", Keys: "x", Action: "command.settings"}, {Mode: "prompt.insert", Keys: "xy", Action: "command.help"}}
	var err error
	a.keymaps, err = a.buildKeymaps(a.Config)
	if err != nil {
		t.Fatal(err)
	}
	a.key(key("a"))
	a.key(key("c"))
	if a.Prompt.String() != "ac" {
		t.Fatalf("failed prefix replayed incorrectly: %q", a.Prompt.String())
	}
	a.keymapTimeout(a.keySerial)
	if a.Prompt.String() != "ac" {
		t.Fatal("stale timer replayed keys twice")
	}
	a.key(key("a"))
	a.key(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("pasted"), Paste: true})
	if a.Prompt.String() != "acapasted" {
		t.Fatalf("paste lost a pending prefix: %q", a.Prompt.String())
	}
	a.key(key("x"))
	if a.popup != nil {
		t.Fatal("ambiguous exact match ran before timeout")
	}
	a.keymapTimeout(a.keySerial)
	if a.popup == nil || a.popup.Kind != "settings" {
		t.Fatal("timeout did not run exact action")
	}
}

func TestPendingMappingCannotCrossFocusOrInterceptTerminalInput(t *testing.T) {
	a := fixture(t)
	a.focus = "chat"
	a.key(tea.KeyMsg{Type: tea.KeyCtrlW})
	a.focus = "prompt"
	a.key(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("draft"), Paste: true})
	a.keymapTimeout(a.keySerial)
	if a.Prompt.String() != "draft" || len(a.keyPending) != 0 {
		t.Fatal("pending chord entered another buffer")
	}
	a.focus = "terminal0"
	a.key(tea.KeyMsg{Type: tea.KeyCtrlW})
	if len(a.keyPending) != 0 {
		t.Fatal("chat mapping delayed raw terminal input")
	}
	a.focus = "chat"
	a.key(tea.KeyMsg{Type: tea.KeyCtrlW})
	a.key(tea.KeyMsg{Type: tea.KeyEsc})
	a.keymapTimeout(a.keySerial)
	if len(a.keyPending) != 0 || len(a.windows) != 1 {
		t.Fatal("escape did not cancel chord")
	}
}
