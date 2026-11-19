package ui

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/erdajt/wraith/internal/plugin"
)

// uiInvocation binds a registered test provider without calling an external backend.
func uiInvocation(t *testing.T, a *App) (*plugin.Invocation, pluginInvocationMsg) {
	t.Helper()
	d := a.pluginManifest.Providers[0]
	call, err := a.plugins.BeginInvocation(t.Context(), plugin.InvocationRequest{
		Binding: plugin.InvocationBinding{Kind: "provider", Name: d.Name, Method: "chat", Owner: d.Owner, Lease: d.Lease, Callback: d.Chat, Revision: d.Revision},
		Data:    json.RawMessage(`{}`),
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(call.Close)
	return call, pluginInvocationMsg{a.plugins, <-a.plugins.InvocationStarts()}
}

func TestInvocationHostRejectsFailedEffectsAndRetainsDeferredReply(t *testing.T) {
	for _, failing := range []bool{false, true} {
		t.Run(map[bool]string{true: "failure", false: "deferred"}[failing], func(t *testing.T) {
			body := `wraith.schedule(function(ctx) wraith.invocation.finish(ctx.invocation.id,{content="done"}) end)`
			if failing {
				body = `error("expected provider failure")`
			}
			a := pluginFixture(t, `wraith.providers.register("custom",{models=function() end,show=function() end,chat=function(ctx) `+body+` end})`)
			call, start := uiInvocation(t, a)
			result := a.callPluginInvocation(start)().(pluginInvocationResultMsg).Result
			cmd := a.applyPluginResult(result)
			if !failing {
				if !a.plugins.InvocationValid(call.ID) {
					t.Fatal("successful callback ended before its scheduled reply")
				}
				scheduled := cmd().(pluginScheduledMsg)
				a.applyPluginResult(a.runPluginSchedule(scheduled)().(pluginResultMsg))
			}
			event, err := call.Next(t.Context())
			if err != nil || !event.Final || failing != strings.Contains(event.Error, "expected provider failure") {
				t.Fatalf("terminal reply: %+v, %v", event, err)
			}
		})
	}
}

func TestInvocationCancellationDismissesOwnedDialog(t *testing.T) {
	a := pluginFixture(t, `wraith.providers.register("custom",{models=function() end,show=function() end,chat=function(ctx)
 wraith.ui.input({title="provider choice"},function(next)
  wraith.invocation.finish(ctx.invocation.id,{content="must not finish"})
 end)
end})`)
	call, start := uiInvocation(t, a)
	result := a.callPluginInvocation(start)().(pluginInvocationResultMsg).Result
	applyPluginEvents(t, a, a.applyPluginResult(result))
	if a.pluginDialog == nil {
		t.Fatal("invocation dialog did not open")
	}
	id := a.pluginDialog.buffer.ID
	a.plugins.AbortInvocation(call.ID, context.Canceled)
	a.reconcilePluginDialog()
	if a.pluginDialog != nil {
		t.Fatal("canceled invocation retained its modal")
	}
	if _, err := a.plugins.Surfaces().Buffer(id); err == nil {
		t.Fatal("canceled dialog retained its buffer")
	}
}

func TestInvocationNestedCommandPreservesCancellation(t *testing.T) {
	a := pluginFixture(t, `wraith.commands.create("child",function(ctx)
 assert(ctx.invocation ~= nil)
 wraith.schedule(function(next)
  assert(next.invocation.id==ctx.invocation.id)
  wraith.invocation.finish(ctx.invocation.id,{content="nested"})
 end)
end)
wraith.providers.register("custom",{models=function() end,show=function() end,chat=function()
 wraith.action("command.child")
end})`)
	call, start := uiInvocation(t, a)
	result := a.callPluginInvocation(start)().(pluginInvocationResultMsg).Result
	complete := func() tea.Msg {
		event, err := call.Next(t.Context())
		if err != nil || event.Error != "" || string(event.Data) != `{"content":"nested"}` {
			return pluginResultMsg{Runtime: a.plugins, Result: plugin.Result{Err: context.Canceled}}
		}
		return pluginResultMsg{Runtime: a.plugins, Explicit: true, Result: plugin.Result{Effects: []plugin.Effect{{Name: "notify", Argument: "complete"}}}}
	}
	runScheduledProgram(t, a, tea.Batch(a.applyPluginResult(result), complete))
}
