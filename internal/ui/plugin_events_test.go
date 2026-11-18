package ui

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func TestLuaWindowEventsSeeUpdatedGeometry(t *testing.T) {
	a := pluginFixture(t, `wraith.events.on("window_changed", function() local views = wraith.windows.list(); assert(views[1].visible and views[1].width > 0); wraith.notify(tostring(#views)) end)`)
	a.width, a.height, a.focus = 120, 40, "chat"
	a.key(tea.KeyMsg{Type: tea.KeyCtrlW})
	_, cmd := a.Update(key("v"))
	if cmd == nil {
		t.Fatal("window event was not queued")
	}
	a.Update(cmd())
	if a.notice != "2" {
		t.Fatalf("event saw outdated window geometry: %s", a.notice)
	}
}

func TestLuaCallbackChainsStopWithoutBlockingInput(t *testing.T) {
	a := pluginFixture(t, `wraith.commands.create("repeat", function() wraith.action("command.repeat") end)`)
	cmd := a.runCommand("repeat")
	for i := 0; cmd != nil && i < 20; i++ {
		_, cmd = a.Update(cmd())
	}
	if cmd != nil || a.notice != "plugin callback chain limit reached" {
		t.Fatalf("recursive callback chain did not stop: %s", a.notice)
	}
	a.key(key("draft"))
	if a.Prompt.String() != "draft" {
		t.Fatal("callback chain blocked normal input")
	}
}
