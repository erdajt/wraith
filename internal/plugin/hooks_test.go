package plugin

import "testing"

func TestHookRegistrationRetainsOnlyValidatedCallbacks(t *testing.T) {
	r := openTestRuntime(t, `
for i=1,32 do
 wraith.hooks.add("before_request",function() end,{priority=i,purposes={"chat","audit"}})
end
for i=1,600 do
 assert(not pcall(wraith.hooks.add,"before_request",function() end))
end
assert(not pcall(wraith.hooks.add,"before_command",function() end,{priority=1.5}))
assert(not pcall(wraith.hooks.add,"before_command",function() end,{purposes={"unknown"}}))
assert(not pcall(wraith.hooks.add,"before_command",function() end,{extra=true}))
`)
	m := r.Manifest()
	if len(m.Hooks) != 32 || len(m.Callbacks) != 32 {
		t.Fatalf("invalid hooks retained callbacks: hooks=%d callbacks=%d", len(m.Hooks), len(m.Callbacks))
	}
	for i, hook := range m.Hooks {
		if hook.Priority != i+1 || hook.Owner != "config" || hook.Lease == "" || hook.ID != hook.Callback {
			t.Fatalf("invalid hook identity: %+v", hook)
		}
	}
	m.Hooks[0].Purposes[0] = "changed"
	if r.Manifest().Hooks[0].Purposes[0] != "chat" {
		t.Fatal("manifest hook purposes share storage")
	}
}

func TestHookChangesRequireHostAcceptanceAndRollbackOnError(t *testing.T) {
	r := openTestRuntime(t, `
local id=wraith.hooks.add("before_command",function() end)
wraith.commands.create("replace",function()
 wraith.hooks.remove(id)
 wraith.hooks.add("before_request",function() end)
end)
wraith.commands.create("failed",function()
 wraith.hooks.remove(id)
 error("reject changes")
end)
`)
	original := r.Manifest()
	pending, err := r.Submit(original.Commands[0].Callback, "", nil, original.Config)
	if err != nil {
		t.Fatal(err)
	}
	result := receiveResult(t, pending)
	if result.Err != nil || result.Manifest == nil || result.Manifest.Hooks[0].Kind != "before_request" {
		t.Fatalf("hook change not staged: %+v", result)
	}
	if r.Manifest().Hooks[0].Kind != "before_command" {
		t.Fatal("unaccepted hook became visible")
	}
	if err := r.Acknowledge(result.Revision, false); err != nil {
		t.Fatal(err)
	}
	pending, err = r.Submit(original.Commands[1].Callback, "", nil, original.Config)
	if err != nil {
		t.Fatal(err)
	}
	if result = receiveResult(t, pending); result.Err == nil {
		t.Fatal("failed hook update succeeded")
	}
	if got := r.Manifest().Hooks; len(got) != 1 || got[0].ID != original.Hooks[0].ID {
		t.Fatal("rejected hook changes altered accepted registry")
	}
}
