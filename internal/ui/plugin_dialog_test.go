package ui

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/erdajt/wraith/internal/plugin"
)

// TestPluginSelectFiltersStableItemsAndAcceptsFirstMatch exercises the real Lua callback boundary.
func TestPluginSelectFiltersStableItemsAndAcceptsFirstMatch(t *testing.T) {
	a := pluginFixture(t, `wraith.commands.create("choose",function()
 wraith.ui.select({{id="one",label="alpha"},{id="two",label="beta"}}, {}, function(ctx)
  assert(ctx.data.item_id=="two" and ctx.data.index==2 and not ctx.data.cancelled)
  wraith.notify("selected")
 end)
end)`)
	result := a.runCommand("choose")().(pluginResultMsg)
	a.openPluginDialog(a.plugins, result.Result.Effects[0].Data.(plugin.Dialog))
	if a.pluginDialog == nil {
		t.Fatal("dialog did not open")
	}
	buffer := a.pluginDialog.buffer.ID
	a.pluginDialogKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("/")})
	a.pluginDialogKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("beta")})
	cmd := a.pluginDialogKey(tea.KeyMsg{Type: tea.KeyEnter})
	if a.pluginDialog != nil || cmd == nil {
		t.Fatal("first enter did not accept the filtered item")
	}
	a.Update(cmd())
	if a.notice != "selected" {
		t.Fatalf("callback: %s", a.notice)
	}
	if _, err := a.plugins.Surfaces().Buffer(buffer); err == nil {
		t.Fatal("completed modal retained its buffer")
	}
}

// TestPluginInputKeepsDraftAndCancellationSeparate verifies vim input and native modal priority.
func TestPluginInputKeepsDraftAndCancellationSeparate(t *testing.T) {
	a := pluginFixture(t, `wraith.commands.create("input",function()
 wraith.ui.input({initial="draft"},function(ctx)
  if ctx.data.cancelled then wraith.notify(ctx.data.reason or "cancelled")
  else wraith.notify(ctx.data.value) end
 end)
end)`)
	a.Prompt.Set("conversation draft")
	request := func() plugin.Dialog {
		t.Helper()
		return a.callPlugin(a.pluginManifest.Commands[0].Callback, "", "")().(pluginResultMsg).Result.Effects[0].Data.(plugin.Dialog)
	}
	a.openPluginDialog(a.plugins, request())
	a.pluginDialogKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(" edited")})
	a.Update(a.pluginDialogKey(tea.KeyMsg{Type: tea.KeyEnter})())
	if a.notice != "draft edited" || a.Prompt.String() != "conversation draft" {
		t.Fatalf("input result = %q, draft = %q", a.notice, a.Prompt.String())
	}
	a.popup = &menu{Kind: "approval"}
	a.Update(a.openPluginDialog(a.plugins, request())())
	if a.notice != "busy" || a.popup.Kind != "approval" || a.pluginDialog != nil {
		t.Fatal("plugin input replaced native approval")
	}
}
