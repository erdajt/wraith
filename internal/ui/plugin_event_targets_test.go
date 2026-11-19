package ui

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func TestLazyEventHandoffPublishesBeforeDeliveringOnlyNewListeners(t *testing.T) {
	a := pluginFixture(t, `local old, fresh = 0, 0
wraith.events.on("ready",function() old=old+1 end)
wraith.commands.create("activate",function()
 local id=wraith.events.on("ready",function(ctx)
  assert(ctx.data.value=="trigger" and old==1)
  fresh=fresh+1
  wraith.notify("delivered:"..fresh)
 end)
 wraith.events.dispatch("ready",{value="trigger"},{listeners={id}})
end)
wraith.commands.create("check",function() assert(old==1 and fresh==1); wraith.notify("verified") end)`)
	var drain func(tea.Cmd)
	drain = func(cmd tea.Cmd) {
		if cmd == nil {
			return
		}
		switch msg := cmd().(type) {
		case tea.BatchMsg:
			for _, child := range msg {
				drain(child)
			}
		case pluginResultMsg:
			_, next := a.Update(msg)
			drain(next)
		}
	}
	drain(a.callPluginData("", "ready", "", nil))
	drain(a.runCommand("activate"))
	if a.notice != "delivered:1" {
		t.Fatalf("first delivery: %s", a.notice)
	}
	drain(a.runCommand("check"))
	if a.notice != "verified" {
		t.Fatalf("handoff duplicated a listener: %s", a.notice)
	}
}
