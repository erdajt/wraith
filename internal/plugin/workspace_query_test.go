package plugin

import (
	"strings"
	"testing"
)

// TestWorkspaceQueryCallbacks verifies detached options, ownership and once-only delivery.
func TestWorkspaceQueryCallbacks(t *testing.T) {
	runtime := openTestRuntime(t, `wraith.commands.create("history", function()
 local options = {id="0123456789abcdef01234567",pattern="open",limit=20}
 local id = wraith.jobs.output(options, function(ctx)
  assert(ctx.data.ok and ctx.data.result.lines[1].text == "open")
  wraith.notify("read")
 end)
 options.pattern = "changed"
 wraith.workspace.cancel(id)
end)`)
	manifest := runtime.Manifest()
	result, err := runtime.Submit(manifest.Commands[0].Callback, "", nil, manifest.Config)
	if err != nil {
		t.Fatal(err)
	}
	got := receiveResult(t, result)
	if got.Err != nil || len(got.Effects) != 2 {
		t.Fatalf("query effects: %+v", got)
	}
	query := got.Effects[0].Data.(WorkspaceQuery)
	if query.Kind != "jobs.output" || query.Options.Pattern != "open" || !runtime.Surfaces().OwnerValid(query.Owner, query.Lease) || got.Effects[1].Argument != query.ID {
		t.Fatalf("query ownership or options: %+v", query)
	}

	snapshot := map[string]any{"data": map[string]any{"ok": true, "result": map[string]any{"lines": []any{map[string]any{"text": "open"}}}}}
	for attempt := range 2 {
		result, err = runtime.Submit(query.ID, "", snapshot, manifest.Config)
		if err != nil {
			t.Fatal(err)
		}
		got = receiveResult(t, result)
		if got.Err != nil || len(got.Effects) != 1-attempt {
			t.Fatalf("delivery %d: %+v", attempt, got)
		}
	}
}

// TestWorkspaceQueryValidation rejects unsupported fields before retaining a private callback.
func TestWorkspaceQueryValidation(t *testing.T) {
	for _, source := range []string{
		`wraith.jobs.output({id="bad"},function() end)`,
		`wraith.jobs.output({id="0123456789abcdef01234567",share_output=true},function() end)`,
		`wraith.jobs.list({session="0123456789abcdef01234567",directory="/tmp"},function() end)`,
		`wraith.sessions.list({limit=101},function() end)`,
		`wraith.sessions.messages({from=-1},function() end)`,
		`wraith.sessions.message({index=0},function() end)`,
		`wraith.sessions.message({revision="current",field="environment"},function() end)`,
		`wraith.terminals.list({output=true},function() end)`,
		`wraith.environment.list({values=true},function() end)`,
	} {
		t.Run(source, func(t *testing.T) {
			runtime := openTestRuntime(t, `wraith.commands.create("invalid",function() `+source+` end)`)
			manifest := runtime.Manifest()
			result, err := runtime.Submit(manifest.Commands[0].Callback, "", nil, manifest.Config)
			if err != nil {
				t.Fatal(err)
			}
			got := receiveResult(t, result)
			if got.Err == nil || len(got.Effects) != 0 {
				t.Fatalf("invalid query accepted: %+v", got)
			}
		})
	}
}

// TestWorkspaceQuerySetupDefersHostWork keeps setup free of native workspace reads.
func TestWorkspaceQuerySetupDefersHostWork(t *testing.T) {
	runtime := openTestRuntime(t, `wraith.sessions.list({},function() end)`)
	manifest := runtime.Manifest()
	channel, err := runtime.Submit("", "ready", nil, manifest.Config)
	if err != nil {
		t.Fatal(err)
	}
	got := receiveResult(t, channel)
	if got.Err != nil || len(got.Effects) != 1 || got.Effects[0].Name != "schedule.callback" {
		t.Fatalf("setup dispatch: %+v", got)
	}
	if got.Revision != 0 {
		if err := runtime.Acknowledge(got.Revision, true); err != nil {
			t.Fatal(err)
		}
	}
	channel, err = runtime.Submit(got.Effects[0].Argument, "", nil, manifest.Config)
	if err != nil {
		t.Fatal(err)
	}
	got = receiveResult(t, channel)
	if got.Err != nil || len(got.Effects) != 1 || !strings.HasPrefix(got.Effects[0].Argument, "lua.schedule.") || got.Effects[0].Name != "workspace.query" {
		t.Fatalf("deferred query: %+v", got)
	}
}

// TestWorkspaceLifecycleEventsCanBeObserved accepts native service changes without namespaced aliases.
func TestWorkspaceLifecycleEventsCanBeObserved(t *testing.T) {
	runtime := openTestRuntime(t, `for _,name in ipairs({"jobs_changed","terminals_changed","environment_changed"}) do
 wraith.events.on(name,function() end)
 end`)
	if len(runtime.Manifest().Events) != 3 {
		t.Fatal("native service events missing")
	}
}
