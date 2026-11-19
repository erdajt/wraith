package ui

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/erdajt/wraith/internal/plugin"
	"github.com/erdajt/wraith/internal/session"
	"github.com/erdajt/wraith/internal/variables"
)

// TestLuaWorkspaceQueriesRunThroughTheEventLoop verifies real native pages and once callback effects.
func TestLuaWorkspaceQueriesRunThroughTheEventLoop(t *testing.T) {
	a := pluginFixture(t, `wraith.commands.create("inspect",function()
 wraith.sessions.messages({from=65,limit=2},function(ctx)
  assert(ctx.data.ok and #ctx.data.result.items == 2)
  assert(ctx.data.result.items[1].index == 65)
  wraith.notify("complete")
 end)
end)`)
	for range 70 {
		a.Session.Messages = append(a.Session.Messages, session.Message{Role: "user", Content: "page"})
	}
	runScheduledProgram(t, a, a.runCommand("inspect"))
	if len(a.pluginQueries) != 0 {
		t.Fatal("delivered query retained its slot")
	}
}

// TestWorkspaceCancellationAndLateReplies checks owner-scoped cancellation and duplicate delivery.
func TestWorkspaceCancellationAndLateReplies(t *testing.T) {
	a := pluginFixture(t, `wraith.commands.create("inspect",function()
 wraith.sessions.get({},function(ctx)
  assert(not ctx.data.ok and ctx.data.error.code == "cancelled")
  wraith.notify("cancelled once")
 end)
end)`)
	result := a.runCommand("inspect")().(pluginResultMsg)
	query := result.Result.Effects[0].Data.(plugin.WorkspaceQuery)
	command := a.startWorkspaceQuery(a.plugins, query)
	a.cancelWorkspaceQuery(a.plugins, "foreign", query.Lease, query.ID)
	if a.pluginQueries[query.ID].context.Err() != nil {
		t.Fatal("foreign owner cancelled query")
	}
	a.cancelWorkspaceQuery(a.plugins, query.Owner, query.Lease, query.ID)
	message := command().(workspaceQueryMsg)
	applyPluginEvents(t, a, a.deliverWorkspaceQuery(message))
	if a.notice != "cancelled once" || a.deliverWorkspaceQuery(message) != nil || len(a.pluginQueries) != 0 {
		t.Fatalf("query cancellation or duplicate reply: %s", a.notice)
	}
}

// TestWorkspaceOwnerDisposalDropsReplies cancels workers without reentering unloaded Lua.
func TestWorkspaceOwnerDisposalDropsReplies(t *testing.T) {
	a := pluginFixture(t, `wraith.commands.create("inspect",function()
 wraith.sessions.get({},function() error("disposed callback ran") end)
end)`)
	result := a.runCommand("inspect")().(pluginResultMsg)
	query := result.Result.Effects[0].Data.(plugin.WorkspaceQuery)
	command := a.startWorkspaceQuery(a.plugins, query)
	state := a.pluginQueries[query.ID]
	a.plugins = nil
	a.reconcileWorkspaceQueries(false)
	if state.context.Err() != context.Canceled || len(a.pluginQueries) != 0 {
		t.Fatal("disposed query retained active work")
	}
	if a.deliverWorkspaceQuery(command().(workspaceQueryMsg)) != nil {
		t.Fatal("disposed callback was scheduled")
	}
}

// TestWorkspaceInventoryOmitsPrivateValues preserves redacted comments in explicit metadata queries.
func TestWorkspaceInventoryOmitsPrivateValues(t *testing.T) {
	a := pluginFixture(t, `wraith.commands.create("inspect",function()
 wraith.environment.list({},function(ctx)
  assert(ctx.data.ok and ctx.data.result.items[1].name == "TARGET")
  assert(ctx.data.result.items[1].value == nil)
  assert(ctx.data.result.scope == "directory")
  wraith.notify("complete")
 end)
end)`)
	a.Variables = variables.NewSnapshot([]variables.Entry{{Name: "TARGET", Value: "private.example", Description: "target private.example"}}, nil)
	runScheduledProgram(t, a, a.runCommand("inspect"))
	a.observeWorkspaceServices()
	data, err := json.Marshal(a.workspaceEvents.pending)
	if err != nil || strings.Contains(string(data), "private.example") {
		t.Fatalf("workspace event leaked private data: %s, %v", data, err)
	}
}

// TestWorkspaceInvocationCancellation drops outstanding native reads with their provider call.
func TestWorkspaceInvocationCancellation(t *testing.T) {
	a := pluginFixture(t, `wraith.providers.register("queries", {
 models=function() end, show=function() end,
 chat=function()
  wraith.sessions.get({}, function() error("cancelled invocation callback ran") end)
 end,
})`)
	definition := a.plugins.Manifest().Providers[0]
	call, err := a.plugins.BeginInvocation(t.Context(), plugin.InvocationRequest{Binding: plugin.InvocationBinding{
		Kind: "provider", Name: definition.Name, Method: "chat", Owner: definition.Owner,
		Lease: definition.Lease, Callback: definition.Chat, Revision: definition.Revision,
	}})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(call.Close)
	start := <-a.plugins.InvocationStarts()
	pending, err := a.plugins.SubmitInvocation(start, nil, a.Config)
	if err != nil {
		t.Fatal(err)
	}
	result := <-pending
	if result.Err != nil || len(result.Effects) != 1 {
		t.Fatalf("provider query effects: %+v", result)
	}
	query := result.Effects[0].Data.(plugin.WorkspaceQuery)
	if query.Invocation != call.ID {
		t.Fatal("query lost provider invocation identity")
	}
	command := a.startWorkspaceQuery(a.plugins, query)
	state := a.pluginQueries[query.ID]
	call.Close()
	a.reconcileWorkspaceQueries(false)
	if state.context.Err() != context.Canceled || len(a.pluginQueries) != 0 {
		t.Fatal("provider cancellation retained its query")
	}
	if a.deliverWorkspaceQuery(command().(workspaceQueryMsg)) != nil {
		t.Fatal("cancelled provider query reentered Lua")
	}
}
