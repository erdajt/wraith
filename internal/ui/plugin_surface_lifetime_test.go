package ui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

// TestExplicitCallbackCannotApplyEarlierEffectsAfterOwnerUnload checks the entire producer lifetime.
func TestExplicitCallbackCannotApplyEarlierEffectsAfterOwnerUnload(t *testing.T) {
	a := pluginFixture(t, `package.preload.dialogs=function() return {setup=function()
 wraith.commands.create("choose",function()
  wraith.notify("stale notification")
  wraith.ui.select({"one"},{},function() error("dead owner") end)
 end)
end} end
wraith.plugins.load("dialogs")
wraith.commands.create("remove",function() wraith.plugins.unload("dialogs") end)`)
	pending := a.runCommand("choose")().(pluginResultMsg)
	a.Update(a.runCommand("remove")())
	a.Update(pending)
	if a.pluginDialog != nil || !strings.Contains(a.notice, "unloaded") {
		t.Fatalf("unloaded callback applied effects: %s", a.notice)
	}
}

// TestExplicitDialogKeepsNewDraftAndDropsStaleImplicitEdit distinguishes object and input ownership.
func TestExplicitDialogKeepsNewDraftAndDropsStaleImplicitEdit(t *testing.T) {
	a := pluginFixture(t, `wraith.commands.create("choose",function()
 wraith.prompt.set("old replacement")
 wraith.ui.select({"one"},{},function() end)
end)`)
	pending := a.runCommand("choose")().(pluginResultMsg)
	a.Prompt.Set("new draft")
	a.Update(pending)
	if a.Prompt.String() != "new draft" || a.pluginDialog == nil {
		t.Fatal("explicit dialog lost or stale implicit input applied")
	}
}

// TestNativeApprovalSupersedesPluginDialogOnce preserves modal priority and releases the buffer.
func TestNativeApprovalSupersedesPluginDialogOnce(t *testing.T) {
	a := pluginFixture(t, `local decisions=0
wraith.commands.create("choose",function()
 wraith.ui.select({"one"},{},function(ctx)
  decisions=decisions+1
  assert(decisions==1 and ctx.data.cancelled and ctx.data.reason=="superseded")
  wraith.notify("complete")
 end)
end)`)
	a.Update(a.runCommand("choose")())
	buffer := a.pluginDialog.buffer.ID
	a.popup = &menu{Kind: "approval"}
	runScheduledProgram(t, a, a.reconcilePluginDialog())
	if a.pluginDialog != nil || a.popup.Kind != "approval" {
		t.Fatal("native approval lost input ownership")
	}
	if _, err := a.plugins.Surfaces().Buffer(buffer); err == nil {
		t.Fatal("superseded dialog retained its buffer")
	}
	if cmd := a.reconcilePluginDialog(); cmd != nil {
		t.Fatal("dialog cancellation repeated")
	}
}

// TestPluginInputRemapsUseTheModalBuffer keeps normal-mode input mappings out of the prompt.
func TestPluginInputRemapsUseTheModalBuffer(t *testing.T) {
	a := pluginFixture(t, `wraith.keymap.set("buffer.normal","x",{keys="iok"})
wraith.commands.create("input",function() wraith.ui.input({},function(ctx) wraith.notify(ctx.data.value) end) end)`)
	a.Update(a.runCommand("input")())
	a.key(tea.KeyMsg{Type: tea.KeyEsc})
	if cmd := a.key(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("x")}); cmd != nil {
		a.Update(cmd())
	}
	if a.pluginDialog.reader.Buffer.String() != "ok" || a.Prompt.String() != "" {
		t.Fatal("mapped input escaped the modal editor")
	}
}
