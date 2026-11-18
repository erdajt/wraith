package ui

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func TestLuaUnloadRestoresWorkspaceMappingsAfterHostAcceptance(t *testing.T) {
	a := pluginFixture(t, `
local cleaned = 0
wraith.commands.create("identify", function() wraith.notify("base") end)
wraith.keymap.set("prompt.insert", "<F4>", "command.identify")
package.preload.overlay = function()
  return {
    setup = function()
      wraith.commands.create("identify", function() wraith.notify("overlay") end, "overlay command", {replace = true})
      wraith.keymap.set("prompt.insert", "<F4>", function() wraith.notify("overlay mapping") end)
    end,
    teardown = function() cleaned = cleaned + 1 end,
  }
end
wraith.commands.create("install", function() wraith.plugins.load("overlay") end)
wraith.commands.create("remove", function() wraith.plugins.unload("overlay") end)
wraith.commands.create("cleaned", function() wraith.notify(tostring(cleaned)) end)`)
	a.Update(a.runCommand("install")())
	a.Update(a.key(tea.KeyMsg{Type: tea.KeyF4})())
	if a.notice != "overlay mapping" {
		t.Fatalf("loaded mapping was not installed: %s", a.notice)
	}

	rejected := a.runCommand("remove")().(pluginResultMsg)
	if rejected.Result.Manifest == nil {
		t.Fatal("unload did not publish its registration changes")
	}
	rejected.Result.Manifest.Config.Theme = "unknown-theme"
	a.Update(rejected)
	a.Update(a.runCommand("cleaned")())
	if a.notice != "0" {
		t.Fatalf("rejected unload ran cleanup: %s", a.notice)
	}
	a.Update(a.key(tea.KeyMsg{Type: tea.KeyF4})())
	if a.notice != "overlay mapping" {
		t.Fatalf("rejected unload removed the active mapping: %s", a.notice)
	}

	a.Update(a.runCommand("remove")())
	a.Update(a.runCommand("cleaned")())
	if a.notice != "1" {
		t.Fatalf("accepted unload did not clean up once: %s", a.notice)
	}
	a.Update(a.key(tea.KeyMsg{Type: tea.KeyF4})())
	if a.notice != "base" {
		t.Fatalf("unload did not restore the previous command and mapping: %s", a.notice)
	}
}
