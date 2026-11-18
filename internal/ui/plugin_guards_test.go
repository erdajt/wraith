package ui

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/erdajt/wraith/internal/config"
	"github.com/erdajt/wraith/internal/editor"
	"github.com/erdajt/wraith/internal/session"
)

func TestYamlEditorRetainsUneditedLuaLayers(t *testing.T) {
	a := pluginFixture(t, `wraith.config.set("theme","nord"); wraith.keymap.set("chat.normal","x",function() end)`)
	a.demo = true
	base := config.Default()
	base.Theme = "rose-pine"
	if err := config.Save(config.File(a.Paths.Config), base); err != nil {
		t.Fatal(err)
	}
	original, err := config.Encode(base, "config.yaml")
	if err != nil {
		t.Fatal(err)
	}
	f, err := editor.Prepare(filepath.Join(a.Paths.Cache, "edits"), ".yaml", string(original))
	if err != nil {
		t.Fatal(err)
	}
	base.Appearance.Background = "inherit"
	changed, err := config.Encode(base, "config.yaml")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(f.Path, changed, 0600); err != nil {
		t.Fatal(err)
	}
	a.finishEdit(editedMsg{Kind: "config", File: f})
	if a.Config.Theme != "nord" || len(a.Config.Keymaps) != 1 {
		t.Fatalf("unrelated yaml edit removed lua layer: theme=%q mappings=%d", a.Config.Theme, len(a.Config.Keymaps))
	}
}

func TestCallbackDoesNotEditNewlySelectedMessage(t *testing.T) {
	a := pluginFixture(t, `wraith.commands.create("edit-selected",function() wraith.action("editor.open") end)`)
	a.focus = "chat"
	a.Session.Messages = []session.Message{{Role: "assistant", Content: "first"}, {Role: "assistant", Content: "second"}}
	a.selected = 0
	a.jumpSelected()
	cmd := a.callPlugin(a.pluginManifest.Commands[0].Callback, "", "")
	a.selected = 1
	a.jumpSelected()
	a.Update(cmd())
	if a.editing {
		t.Fatal("callback opened editor for a message selected after invocation")
	}
}

func TestAdvertisedAliasActionDispatches(t *testing.T) {
	a := pluginFixture(t, `wraith.commands.create("check",function() wraith.action("command.h") end)`)
	a.Update(a.callPlugin(a.pluginManifest.Commands[0].Callback, "", "")())
	if a.popup == nil || a.popup.Kind != "help" {
		t.Fatalf("advertised alias action failed: %s", a.notice)
	}
}

func TestSetupOnlyMarksProvidedLeaves(t *testing.T) {
	a := pluginFixture(t, `wraith.config.setup({appearance={background="inherit"}})`)
	a.settingsCategory("appearance")
	for _, item := range a.popup.Items {
		if item.Value == "appearance.mouse" && strings.Contains(item.Detail, "init.lua on reload") {
			t.Fatal("yaml-owned mouse setting marked as lua override")
		}
	}
}

func TestLuaEffectsWaitForExternalEditorCompletion(t *testing.T) {
	a := pluginFixture(t, `wraith.commands.create("edit-then-set", function() wraith.action("editor.open"); wraith.prompt.set("after editor") end)`)
	a.Prompt.Set("before editor")
	_, cmd := a.Update(a.callPlugin(a.pluginManifest.Commands[0].Callback, "", "")())
	if cmd == nil || !a.editing || len(a.editorDeferred) != 1 || a.Prompt.String() != "before editor" {
		t.Fatal("editor continuation was not held")
	}
	f := a.editorFile
	if err := os.WriteFile(f.Path, []byte("saved edit"), 0600); err != nil {
		t.Fatal(err)
	}
	_, next := a.Update(editedMsg{Kind: "prompt", File: f})
	if a.editing || a.Prompt.String() != "saved edit" || next == nil {
		t.Fatal("editor result was not applied first")
	}
	a.Update(next())
	if a.Prompt.String() != "after editor" || len(a.editorDeferred) != 0 {
		t.Fatal("ordered effects did not resume after editor completion")
	}
}
