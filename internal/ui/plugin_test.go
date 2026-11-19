package ui

import (
	"os"
	"path/filepath"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/erdajt/wraith/internal/plugin"
)

// pluginFixture attaches a real local lua runtime to an isolated workspace.
func pluginFixture(t *testing.T, source string, packages ...plugin.PackageRoot) *App {
	t.Helper()
	a := fixture(t)
	if err := os.WriteFile(filepath.Join(a.Paths.Config, "init.lua"), []byte(source), 0600); err != nil {
		t.Fatal(err)
	}
	r, err := plugin.OpenWithOptions(t.Context(), a.Paths.Config, a.Config, PluginActions(), plugin.Options{Paths: a.Paths, Packages: packages})
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

func TestLuaSlashCompletionRunsCommandsAndOpensTheirChildren(t *testing.T) {
	a := pluginFixture(t, `wraith.commands.create("tools.greet", function(ctx) wraith.prompt.set("hello " .. ctx.argument) end)`)
	a.Prompt.Set("/to")
	a.key(tea.KeyMsg{Type: tea.KeyEnter})
	if a.Prompt.String() != "/tools." {
		t.Fatalf("custom parent did not expand: %q", a.Prompt.String())
	}
	a.key(tea.KeyMsg{Type: tea.KeyTab})
	if a.Prompt.String() != "/tools.greet " {
		t.Fatalf("custom child did not complete: %q", a.Prompt.String())
	}
	a.Prompt.Set("/tools.greet world")
	cmd := a.key(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd == nil {
		t.Fatal("custom slash command was rejected")
	}
	a.Update(cmd())
	if a.Prompt.String() != "hello world" {
		t.Fatalf("slash callback failed: %q", a.Prompt.String())
	}
}

func TestLuaOverridesExAndSlashCommandsExplicitly(t *testing.T) {
	for _, name := range []string{"q", "q!", "qa", "qa!", "w", "w!", "wq", "settings.appearance"} {
		t.Run(name, func(t *testing.T) {
			a := pluginFixture(t, `wraith.commands.create("`+name+`", function(ctx) wraith.prompt.set(ctx.argument) end, "custom behavior", {replace=true, arguments=true})`)
			choices := a.pluginCommands(name)
			if len(choices) == 0 || choices[0].Name != name || choices[0].Description != "custom behavior" || !choices[0].NeedsArgument {
				t.Fatal("override lost completion metadata")
			}
			a.Prompt.Set("/" + name)
			a.key(tea.KeyMsg{Type: tea.KeyEnter})
			if a.Prompt.String() != "/"+name+" " {
				t.Fatal("argument command ran before receiving input")
			}
			for _, prefix := range []string{":", "/"} {
				cmd := a.runCommand(prefix + name + " preserve spaces here")
				if cmd == nil {
					t.Fatalf("override rejected: %s", a.notice)
				}
				a.Update(cmd())
				if a.Prompt.String() != "preserve spaces here" {
					t.Fatal("override lost arguments")
				}
			}
		})
	}
}

func TestLuaDeletingCommandOverrideRestoresBuiltin(t *testing.T) {
	a := pluginFixture(t, `
wraith.commands.create("q", function() error("deleted") end, "override", {replace=true})
wraith.commands.delete("q")
wraith.keymap.set("chat.normal", "Q", "command.q")
`)
	a.width, a.height = 120, 40
	a.windowAction("v")
	if cmd := a.runCommand("q"); cmd != nil || len(a.windows) != 1 {
		t.Fatal("deleted q override did not restore window close")
	}
}
