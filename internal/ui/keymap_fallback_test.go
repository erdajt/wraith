package ui

import (
	"net/http"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/erdajt/wraith/internal/keymap"
	"github.com/erdajt/wraith/internal/session"
)

func TestFailedPrefixRetainsConfiguredWorkspaceActions(t *testing.T) {
	for _, test := range []struct {
		name string
		key  tea.KeyType
		alt  rune
	}{
		{"focus", tea.KeyTab, 0}, {"send", tea.KeyCtrlS, 0}, {"editor", tea.KeyCtrlE, 0},
		{"overlay", 0, 'e'}, {"side", 0, 's'}, {"configured-focus", tea.KeyCtrlF, 0},
	} {
		t.Run(test.name, func(t *testing.T) {
			a := fixture(t)
			a.Config.Thinking = "off"
			if test.name == "configured-focus" {
				a.Config.Keys["focus"] = "ctrl+f"
			}
			a.Config.Keymaps = []keymap.Binding{{Mode: "prompt.insert", Keys: "jk", Action: "noop"}}
			var err error
			a.keymaps, err = a.buildKeymaps(a.Config)
			if err != nil {
				t.Fatal(err)
			}
			a.Client.HTTP.Transport = transport(func(*http.Request) (*http.Response, error) {
				return response(`{"done":true,"message":{"content":"ready"}}`), nil
			})
			a.key(key("j"))
			input := tea.KeyMsg{Type: test.key}
			if test.alt != 0 {
				input = tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{test.alt}, Alt: true}
			}
			a.key(input)
			switch test.name {
			case "focus", "configured-focus":
				if a.focus != "chat" || a.Prompt.String() != "j" {
					t.Fatalf("focus swallowed: %s %q", a.focus, a.Prompt.String())
				}
			case "send":
				if !a.busy || len(a.Session.Messages) != 1 || a.Session.Messages[0].Content != "j" {
					t.Fatal("send swallowed or duplicated")
				}
			case "editor":
				if !a.editing || a.Prompt.String() != "j" {
					t.Fatal("editor swallowed")
				}
			case "overlay":
				if a.panes[0] == nil || a.focus != "terminal0" {
					t.Fatal("overlay swallowed")
				}
			case "side":
				if a.panes[1] == nil || a.focus != "terminal1" {
					t.Fatal("side shell swallowed")
				}
			}
		})
	}
}

func TestDefaultWindowPrefixSurvivesReplayAndExpiresOnce(t *testing.T) {
	for _, test := range []string{"separate", "batched", "timeout"} {
		t.Run(test, func(t *testing.T) {
			a := fixture(t)
			a.width, a.height, a.focus = 120, 40, "chat"
			a.Session.Messages = []session.Message{{Role: "assistant", Content: "first\nsecond\nthird"}}
			a.Config.Keymaps = []keymap.Binding{{Mode: "chat.normal", Keys: "jk", Action: "noop"}}
			var err error
			a.keymaps, err = a.buildKeymaps(a.Config)
			if err != nil {
				t.Fatal(err)
			}
			keys := []tea.KeyMsg{key("j"), {Type: tea.KeyCtrlW}, key("v")}
			if test == "batched" {
				a.dispatchKeys(queuedKeysMsg{Keys: keys, Owner: a.keyOwner(), Mapped: true})
			} else {
				a.key(keys[0])
				a.key(keys[1])
				if len(a.keyPending) == 0 || !a.keyPendingDefault {
					t.Fatal("fallback chord lost ownership")
				}
				if test == "timeout" {
					a.keymapTimeout(a.keySerial)
					if len(a.keyPending) > 0 {
						t.Fatal("expired fallback started another timer")
					}
					return
				}
				a.key(keys[2])
			}
			if len(a.windows) != 2 || len(a.keyPending) != 0 {
				t.Fatal("fallback split did not complete immediately")
			}
		})
	}
}

func TestFallbackPrefixDoesNotOverrideNewScopeMapping(t *testing.T) {
	a := fixture(t)
	a.focus = "chat"
	a.Config.Keymaps = []keymap.Binding{
		{Mode: "chat.normal", Keys: "jk", Action: "noop"},
		{Mode: "prompt.insert", Keys: "x", Action: "focus.next"},
	}
	var err error
	a.keymaps, err = a.buildKeymaps(a.Config)
	if err != nil {
		t.Fatal(err)
	}
	a.key(key("j"))
	a.key(tea.KeyMsg{Type: tea.KeyCtrlW})
	a.focus = "prompt"
	a.key(key("x"))
	if a.focus != "chat" || a.Prompt.String() != "" {
		t.Fatal("stale fallback bypassed the new scope mapping")
	}
}

func TestExpiredOverrideRetainsCompleteDefaultChord(t *testing.T) {
	a := fixture(t)
	a.focus = "chat"
	a.width = 120
	a.height = 40
	a.Config.Keymaps = []keymap.Binding{{Mode: "chat.normal", Keys: "j<C-w>vx", Action: "noop"}}
	var err error
	a.keymaps, err = a.buildKeymaps(a.Config)
	if err != nil {
		t.Fatal(err)
	}
	a.key(key("j"))
	a.key(tea.KeyMsg{Type: tea.KeyCtrlW})
	a.key(key("v"))
	a.keymapTimeout(a.keySerial)
	if len(a.windows) != 2 {
		t.Fatalf("timeout discarded complete default chord: windows=%d visual=%q", len(a.windows), a.visualMode)
	}
}

func TestPasteEndsPendingDefaultChord(t *testing.T) {
	a := fixture(t)
	a.focus = "chat"
	a.width = 120
	a.height = 40
	a.key(tea.KeyMsg{Type: tea.KeyCtrlW})
	a.key(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("literal paste"), Paste: true})
	if len(a.keyPending) != 0 {
		t.Fatalf("paste left old default prefix live: %+v", a.keyPending)
	}
}

func TestAmbiguousTabRetainsSlashCompletion(t *testing.T) {
	a := fixture(t)
	a.Config.Keymaps = []keymap.Binding{{Mode: "prompt.insert", Keys: "<Tab>x", Action: "noop"}}
	var err error
	a.keymaps, err = a.buildKeymaps(a.Config)
	if err != nil {
		t.Fatal(err)
	}
	a.Prompt.Set("/settings")
	a.key(tea.KeyMsg{Type: tea.KeyTab})
	a.keymapTimeout(a.keySerial)
	if a.focus != "prompt" || a.Prompt.String() != "/settings." {
		t.Fatalf("timed tab bypassed slash completion: focus=%s draft=%q", a.focus, a.Prompt.String())
	}
}

func TestAsyncSendDoesNotReorderFollowingInput(t *testing.T) {
	a := fixture(t)
	a.Config.Thinking = "off"
	a.Config.Keymaps = []keymap.Binding{{Mode: "prompt.insert", Keys: "x", Action: "prompt.send"}}
	var err error
	a.keymaps, err = a.buildKeymaps(a.Config)
	if err != nil {
		t.Fatal(err)
	}
	a.Client.HTTP.Transport = transport(func(*http.Request) (*http.Response, error) {
		return response(`{"done":true,"message":{"content":"ready"}}`), nil
	})
	a.Prompt.Set("question")
	a.key(key("xy"))
	a.key(key("z"))
	if a.Prompt.String() != "yz" {
		t.Fatalf("background send reordered typing: %q", a.Prompt.String())
	}
}

func TestBatchedQuitDoesNotOpenAnotherShell(t *testing.T) {
	a := fixture(t)
	a.Config.Keymaps = []keymap.Binding{
		{Mode: "prompt.insert", Keys: "x", Action: "app.quit"},
		{Mode: "prompt.insert", Keys: "z", Action: "terminal.overlay"},
	}
	var err error
	a.keymaps, err = a.buildKeymaps(a.Config)
	if err != nil {
		t.Fatal(err)
	}
	a.key(key("xz"))
	if a.panes[0] != nil {
		t.Fatal("batched input started a shell after shutdown")
	}
}
