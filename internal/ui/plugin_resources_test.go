package ui

import (
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

// resourceResult waits for a real managed timer and captures its detached callback result.
func resourceResult(t *testing.T, a *App) pluginResourceResultMsg {
	t.Helper()
	if err := a.plugins.Activate(a.pluginManifest.Resources); err != nil {
		t.Fatal(err)
	}
	select {
	case event := <-a.plugins.Events():
		cmd := a.callPluginResource(pluginResourceMsg{Runtime: a.plugins, Event: event})
		if cmd == nil {
			t.Fatal("service callback was not submitted")
		}
		result, ok := cmd().(pluginResourceResultMsg)
		if !ok {
			t.Fatal("service callback did not return a result")
		}
		return result
	case <-time.After(3 * time.Second):
		t.Fatal("managed timer did not finish")
	}
	return pluginResourceResultMsg{}
}

func TestLuaServiceEffectsKeepTheirOwnDraftChanges(t *testing.T) {
	for _, source := range []string{
		`wraith.prompt.set("first"); wraith.prompt.set("second")`,
		`wraith.theme.use("nord"); wraith.prompt.set("second")`,
		`wraith.action("command.inner"); wraith.prompt.set("second")`,
	} {
		t.Run(source, func(t *testing.T) {
			a := pluginFixture(t, `
wraith.commands.create("inner", function() wraith.prompt.set("inner") end)
wraith.timer.start(1, function()
  `+source+`
  wraith.notify("complete")
end)`)
			result := resourceResult(t, a)
			runScheduledProgram(t, a, func() tea.Msg { return result })
			if a.Prompt.String() != "second" {
				t.Fatalf("service dropped an authorized continuation: %q", a.Prompt.String())
			}
		})
	}
}

func TestLuaServiceNotificationSurvivesTypingWithoutReplacingDraft(t *testing.T) {
	a := pluginFixture(t, `wraith.timer.start(1, function()
  wraith.prompt.set("obsolete")
  wraith.notify("complete")
end)`)
	result := resourceResult(t, a)
	a.Prompt.Set("still typing")
	a.focus = "chat"
	runScheduledProgram(t, a, func() tea.Msg { return result })
	if a.Prompt.String() != "still typing" {
		t.Fatal("service callback overwrote a newer draft")
	}
}

func TestLuaServiceRegistryHasIndependentKeymapGuard(t *testing.T) {
	for _, change := range []string{"draft", "theme", "keymap"} {
		t.Run(change, func(t *testing.T) {
			a := pluginFixture(t, `wraith.keymap.set("prompt.normal", "<F4>", "noop")
wraith.timer.start(1, function()
  wraith.commands.create("service-ready", function() end)
  wraith.prompt.set("obsolete")
  wraith.notify("complete")
end)`)
			result := resourceResult(t, a)
			switch change {
			case "draft":
				a.Prompt.Set("still typing")
			case "theme":
				a.Config.Theme = "nord"
			case "keymap":
				a.Config.Keymaps = append(a.Config.Keymaps, a.Config.Keymaps...)
				if len(a.Config.Keymaps) == 0 {
					t.Fatal("fixture needs a registered keymap")
				}
			}
			a.applyPluginResult(result.Result)
			registered := len(a.pluginCommands("service-ready")) == 1
			if registered != (change != "keymap") {
				t.Fatalf("registry conflict check failed for %s", change)
			}
			if a.Prompt.String() == "obsolete" {
				t.Fatal("registry installation revived a stale draft effect")
			}
		})
	}
}

func TestLuaStaleServiceParentDoesNotReviveAfterObserver(t *testing.T) {
	a := pluginFixture(t, `wraith.events.on("service.ready", function()
  wraith.notify("observer finished")
end)
wraith.timer.start(1, function()
  wraith.events.emit("service.ready", {})
  wraith.prompt.set("obsolete")
  wraith.notify("complete")
end)`)
	result := resourceResult(t, a)
	a.Prompt.Set("new input")
	runScheduledProgram(t, a, func() tea.Msg { return result })
	if a.Prompt.String() != "new input" {
		t.Fatal("observer continuation revived stale parent input effects")
	}
}

func TestLuaServiceFromRetiredRuntimeCannotChangeWorkspace(t *testing.T) {
	a := pluginFixture(t, `wraith.timer.start(1, function()
  wraith.prompt.set("obsolete")
  wraith.notify("retired")
end)`)
	result := resourceResult(t, a)
	a.plugins.Close()
	a.plugins = nil
	a.notice = "current runtime"
	a.Update(result)
	if a.Prompt.String() != "" || a.notice != "current runtime" {
		t.Fatal("retired resource callback changed current workspace")
	}
}
