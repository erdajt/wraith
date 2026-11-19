package ui

import (
	"errors"
	"testing"
	"time"

	"github.com/erdajt/wraith/internal/plugin"
)

// TestSecondDialogCancelsWithoutReplacingFirst preserves the first pending decision in a callback batch.
func TestSecondDialogCancelsWithoutReplacingFirst(t *testing.T) {
	a := pluginFixture(t, `wraith.commands.create("dialogs",function()
 wraith.ui.input({initial="first"},function(ctx) wraith.notify("first "..ctx.data.value) end)
 wraith.ui.input({initial="second"},function(ctx)
  assert(ctx.data.cancelled and ctx.data.reason=="busy")
  wraith.notify("second cancelled")
 end)
end)`)
	result := a.runCommand("dialogs")().(pluginResultMsg)
	first := result.Result.Effects[0].Data.(plugin.Dialog)
	second := result.Result.Effects[1].Data.(plugin.Dialog)
	a.openPluginDialog(a.plugins, first)
	a.Update(a.openPluginDialog(a.plugins, second)())
	if a.pluginDialog == nil || a.pluginDialog.spec.ID != first.ID || a.notice != "second cancelled" {
		t.Fatal("second dialog replaced the first")
	}
	a.Update(a.finishPluginDialog(map[string]any{"value": "accepted"})())
	if a.notice != "first accepted" {
		t.Fatal(a.notice)
	}
}

// TestDeletedDialogBufferCancelsOnce prevents a deleted picker from retaining input ownership.
func TestDeletedDialogBufferCancelsOnce(t *testing.T) {
	a := pluginFixture(t, `local calls=0
wraith.commands.create("dialog",function()
 wraith.ui.input({},function(ctx)
  calls=calls+1; assert(calls==1 and ctx.data.cancelled)
  wraith.notify(ctx.data.reason)
 end)
end)`)
	result := a.runCommand("dialog")().(pluginResultMsg)
	spec := result.Result.Effects[0].Data.(plugin.Dialog)
	a.openPluginDialog(a.plugins, spec)
	buffer := a.pluginDialog.buffer
	_, err := a.plugins.Surfaces().HostApply(plugin.SurfaceMutation{Operation: "buffer.delete", ID: buffer.ID, Expected: buffer.Revision})
	if err != nil {
		t.Fatal(err)
	}
	cmd := a.reconcilePluginDialog()
	if cmd == nil || a.pluginDialog != nil {
		t.Fatal("deleted dialog kept focus")
	}
	a.Update(cmd())
	if a.notice != "buffer disposed" || a.reconcilePluginDialog() != nil {
		t.Fatal("deleted dialog cancellation was not terminal")
	}
	if cmd := a.submitPluginDecision(pluginDecisionMsg{a.plugins, spec, map[string]any{"value": "replay"}}); cmd != nil {
		a.Update(cmd())
	}
	if a.notice != "buffer disposed" {
		t.Fatal("decision callback was replayed")
	}
}

// TestDialogBlocksConfigurationReplacement keeps editor ownership intact during a live decision.
func TestDialogBlocksConfigurationReplacement(t *testing.T) {
	a := pluginFixture(t, `wraith.commands.create("dialog",function() wraith.ui.input({},function() end) end)`)
	result := a.runCommand("dialog")().(pluginResultMsg)
	a.openPluginDialog(a.plugins, result.Result.Effects[0].Data.(plugin.Dialog))
	next := a.Config
	next.Theme = "nord"
	if _, err := a.applyPluginConfig(next); err == nil || a.Config.Theme == "nord" {
		t.Fatal("configuration replaced an active dialog")
	}
}

// TestDialogDecisionRetriesFullQueueAndStopsAfterRuntimeClose preserves one-shot ownership under pressure.
func TestDialogDecisionRetriesFullQueueAndStopsAfterRuntimeClose(t *testing.T) {
	a := pluginFixture(t, `wraith.commands.create("dialog",function()
 wraith.ui.input({},function(ctx) wraith.notify("accepted once") end)
end)
wraith.commands.create("block",function() while true do end end)`)
	result := a.runCommand("dialog")().(pluginResultMsg)
	spec := result.Result.Effects[0].Data.(plugin.Dialog)
	a.openPluginDialog(a.plugins, spec)
	var blocked <-chan plugin.Result
	for _, command := range a.pluginManifest.Commands {
		if command.Name == "block" {
			var err error
			blocked, err = a.plugins.Submit(command.Callback, "", nil, a.Config)
			if err != nil {
				t.Fatal(err)
			}
		}
	}
	var queued []<-chan plugin.Result
	full := false
	for range 200 {
		result, err := a.plugins.Submit("", "plugin.test.noop", nil, a.Config)
		if errors.Is(err, plugin.ErrQueueFull) {
			full = true
			break
		}
		if err != nil {
			t.Fatal(err)
		}
		queued = append(queued, result)
	}
	if !full {
		t.Fatal("queue did not fill")
	}
	retry := a.finishPluginDialog(map[string]any{"value": "accepted"})
	if retry == nil || a.pluginDialog != nil {
		t.Fatal("decision retained modal input")
	}
	pending, ok := retry().(pluginDecisionMsg)
	if !ok {
		t.Fatal("queue pressure did not retain decision")
	}
	select {
	case <-blocked:
	case <-time.After(3 * time.Second):
		t.Fatal("blocked callback did not finish")
	}
	for _, result := range queued {
		select {
		case <-result:
		case <-time.After(time.Second):
			t.Fatal("queue did not drain")
		}
	}
	cmd := a.submitPluginDecision(pending)
	if cmd == nil {
		t.Fatal("decision retry was lost")
	}
	a.Update(cmd())
	if a.notice != "accepted once" {
		t.Fatal(a.notice)
	}
	a.plugins.Close()
	if a.submitPluginDecision(pending) != nil {
		t.Fatal("retired runtime accepted a stale decision")
	}
}
