package ui

import (
	"context"
	"io"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

func TestLuaLiveRegistrationsReachCompletionAndInput(t *testing.T) {
	a := pluginFixture(t, `wraith.commands.create("install", function()
  wraith.commands.create("late", function() wraith.prompt.set("installed") end, "late command")
  wraith.keymap.set("prompt.insert", "<F4>", "command.late")
end)`)
	a.Update(a.runCommand("install")())
	matches := a.pluginCommands("late")
	if len(matches) != 1 || matches[0].Description != "late command" {
		t.Fatalf("live command missing from completion: %+v; %s", matches, a.notice)
	}
	cmd := a.key(tea.KeyMsg{Type: tea.KeyF4})
	if cmd == nil {
		t.Fatal("live mapping was not installed")
	}
	a.Update(cmd())
	if a.Prompt.String() != "installed" {
		t.Fatalf("live mapping did not dispatch: %q; %s", a.Prompt.String(), a.notice)
	}
}

func TestLuaRegistryPublicationPrecedesNestedCallback(t *testing.T) {
	a := pluginFixture(t, `wraith.commands.create("install", function()
  wraith.commands.create("late", function()
    wraith.prompt.set("child")
    wraith.commands.create("leaf", function() end)
  end)
  wraith.action("command.late")
  wraith.prompt.set("after")
  wraith.notify("complete")
end)`)
	ctx, cancel := context.WithTimeout(t.Context(), 3*time.Second)
	defer cancel()
	p := tea.NewProgram(callbackProgram{a, a.runCommand("install")}, tea.WithContext(ctx), tea.WithInput(nil), tea.WithOutput(io.Discard), tea.WithoutRenderer(), tea.WithoutSignalHandler())
	if _, err := p.Run(); err != nil {
		t.Fatal(err)
	}
	if a.Prompt.String() != "after" || a.notice != "complete" || len(a.pluginCommands("leaf")) != 1 {
		t.Fatalf("registry continuation failed: draft=%q notice=%q", a.Prompt.String(), a.notice)
	}
}

func TestLuaRejectedRegistryDoesNotBlockNextCallback(t *testing.T) {
	for _, failure := range []string{"stale", "error", "invalid"} {
		t.Run(failure, func(t *testing.T) {
			suffix := ""
			if failure == "error" {
				suffix = `error("stop registration")`
			}
			a := pluginFixture(t, `
wraith.commands.create("check", function() wraith.notify("working") end)
wraith.commands.create("install", function()
  wraith.commands.create("late", function() end)
  `+suffix+`
end)`)
			m := a.runCommand("install")().(pluginResultMsg)
			if failure == "stale" {
				a.Prompt.Set("new draft")
			}
			if failure == "invalid" {
				if m.Result.Manifest == nil {
					t.Fatal("registration did not publish a manifest")
				}
				m.Result.Manifest.Config.Theme = "missing-theme"
			}
			a.Update(m)
			if len(a.pluginCommands("late")) != 0 {
				t.Fatal("rejected registration became visible")
			}

			ctx, cancel := context.WithTimeout(t.Context(), 3*time.Second)
			defer cancel()
			cmd := a.runCommand("check")
			result := make(chan tea.Msg, 1)
			go func() { result <- cmd() }()
			select {
			case msg := <-result:
				a.Update(msg)
			case <-ctx.Done():
				t.Fatal("rejected manifest left the VM waiting for acknowledgement")
			}
			if a.notice != "working" {
				t.Fatalf("old callback stopped working: %s", a.notice)
			}
		})
	}
}

func TestLuaConfigurationEffectsRetainLiveMappings(t *testing.T) {
	for _, order := range []string{"config-first", "mapping-first"} {
		t.Run(order, func(t *testing.T) {
			config := `wraith.config.set("theme", "nord")`
			mapping := `wraith.keymap.set("prompt.insert", "<F4>", function() wraith.notify("mapped") end)`
			body := config + ";" + mapping
			if order == "mapping-first" {
				body = mapping + ";" + config
			}
			a := pluginFixture(t, `wraith.commands.create("install", function() `+body+` end)`)
			a.Update(a.runCommand("install")())
			if a.Config.Theme != "nord" {
				t.Fatalf("configuration effect failed: %s", a.notice)
			}
			cmd := a.key(tea.KeyMsg{Type: tea.KeyF4})
			if cmd == nil {
				t.Fatal("configuration effect overwrote live mappings")
			}
			a.Update(cmd())
			if !strings.Contains(a.notice, "mapped") {
				t.Fatalf("mapping callback missing: %s", a.notice)
			}
		})
	}
}
