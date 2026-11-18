package ui

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/erdajt/wraith/internal/plugin"
)

// pluginFixture attaches a real local lua runtime to an isolated workspace.
func pluginFixture(t *testing.T, source string) *App {
	t.Helper()
	a := fixture(t)
	if err := os.WriteFile(filepath.Join(a.Paths.Config, "init.lua"), []byte(source), 0600); err != nil {
		t.Fatal(err)
	}
	r, err := plugin.Open(t.Context(), a.Paths.Config, a.Config, PluginActions())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(r.Close)
	a.plugins, a.pluginManifest = r, r.Manifest()
	a.Config = a.pluginManifest.Config
	a.actions = a.pluginActions(a.pluginManifest)
	a.keymaps, err = a.buildKeymaps(a.Config)
	if err != nil {
		t.Fatal(err)
	}
	return a
}

func TestLuaCommandCompletesAndChangesSharedDraft(t *testing.T) {
	a := pluginFixture(t, `wraith.commands.create("greet", function(ctx) wraith.prompt.set("hello " .. ctx.argument) end, "greet someone")`)
	a.Prompt.Set("/gr")
	matches, _, _ := a.completions()
	if len(matches) != 1 || matches[0].Name != "greet" {
		t.Fatalf("missing custom command: %+v", matches)
	}
	cmd := a.runCommand("greet world")
	if cmd == nil {
		t.Fatal("custom command was not queued")
	}
	a.Update(cmd())
	if a.Prompt.String() != "hello world" {
		t.Fatalf("callback did not update draft: %q", a.Prompt.String())
	}
}

func TestLuaResultCannotOverwriteNewInputOrApproval(t *testing.T) {
	a := pluginFixture(t, `wraith.keymap.set("*", "<F1>", function() wraith.prompt.set("replacement") end)`)
	id := a.pluginManifest.Callbacks[0]
	cmd := a.callPlugin(id, "", "")
	a.Prompt.Set("new input")
	a.Update(cmd())
	if a.Prompt.String() != "new input" {
		t.Fatal("stale callback overwrote user input")
	}
	a.proposal, a.popup = &proposal{Command: "printf test"}, &menu{Kind: "approval"}
	a.Update(a.callPlugin(id, "", "")())
	if a.Prompt.String() != "new input" || a.popup.Kind != "approval" || a.proposal == nil {
		t.Fatal("callback bypassed approval ownership")
	}
}
