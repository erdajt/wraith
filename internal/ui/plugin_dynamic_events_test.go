package ui

import (
	"context"
	"io"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

func TestLuaDynamicEventPublishesBeforeEmitting(t *testing.T) {
	a := pluginFixture(t, `wraith.commands.create("install", function()
  wraith.events.on("plugin.review.selected", function(ctx)
    assert(ctx.data.id == "task-1")
    wraith.prompt.set("selected " .. ctx.data.id)
  end)
  wraith.events.emit("plugin.review.selected", {id = "task-1"})
  wraith.notify("complete")
end)`)
	ctx, cancel := context.WithTimeout(t.Context(), 3*time.Second)
	defer cancel()
	p := tea.NewProgram(callbackProgram{a, a.runCommand("install")}, tea.WithContext(ctx), tea.WithInput(nil), tea.WithOutput(io.Discard), tea.WithoutRenderer(), tea.WithoutSignalHandler())
	if _, err := p.Run(); err != nil {
		t.Fatal(err)
	}
	if a.Prompt.String() != "selected task-1" || a.notice != "complete" {
		t.Fatalf("dynamic event did not finish before caller: %q; %s", a.Prompt.String(), a.notice)
	}
}

func TestLuaRegistryOnlyCallbackRunsWhileResponseIsBusy(t *testing.T) {
	a := pluginFixture(t, `wraith.commands.create("install", function()
  wraith.commands.create("late", function() wraith.notify("ready") end)
end)`)
	a.busy = true
	a.Update(a.runCommand("install")())
	if len(a.pluginCommands("late")) != 1 || !a.busy {
		t.Fatalf("registry-only update changed model work or failed: %s", a.notice)
	}
	a.Update(a.runCommand("late")())
	if a.notice != "ready" {
		t.Fatalf("late callback failed during response: %s", a.notice)
	}
}

func TestLuaManifestDoesNotApplySettingsBeforeOrderedEffects(t *testing.T) {
	a := pluginFixture(t, `wraith.commands.create("install", function()
  wraith.commands.create("late", function() end)
  wraith.config.set("theme", "nord")
end)`)
	a.popup = &menu{Kind: "approval"}
	before := a.Config.Theme
	a.Update(a.callPlugin(a.pluginManifest.Commands[0].Callback, "", "")())
	if a.Config.Theme != before {
		t.Fatal("manifest applied configuration ahead of the guarded config effect")
	}
	if len(a.pluginCommands("late")) != 1 {
		t.Fatal("independent registry change was not published")
	}
}
