package plugin

import (
	"context"
	"testing"
	"time"
)

// TestPluginStateObservesCoroutineSetupAndRollback reads host phases without lua bookkeeping.
func TestPluginStateObservesCoroutineSetupAndRollback(t *testing.T) {
	openTestRuntime(t, `
 local state=wraith.plugins.state
 assert(state("tracked")==nil and not pcall(state,1) and not pcall(state,{}))
 package.preload.tracked=function() return {setup=function()
  assert(coroutine.running()~=nil and state("tracked")=="loading")
 end} end
 package.preload.state_entry=function() return true end
 wraith.modules.provider(function(name)
  if name=="state_entry" then return {plugin="tracked",check=function() end,activate=function()
   wraith.plugins.load("tracked",{}, {dependency=false})
  end} end
 end)
 local co=coroutine.create(function() assert(require("state_entry")) end)
 assert(coroutine.resume(co))
 assert(state("tracked")=="loaded")
 package.preload.failed=function() return {setup=function()
  assert(state("failed")=="loading")
  error("setup failed")
 end} end
 assert(not pcall(wraith.plugins.load,"failed") and state("failed")==nil)
 local observed=false
 wraith.modules.provider(function(name)
  if name=="state_probe" then observed=state("tracked")=="loaded" end
  return nil
 end)
 package.preload.state_probe=function() return true end
 assert(require("state_probe") and observed)
 wraith.plugins.unload("tracked")
 assert(state("tracked")==nil)
 package.preload[""]=function() return {setup=function() assert(state("")=="loading") end} end
 wraith.plugins.load("")
 assert(state("")=="loaded")
 wraith.plugins.unload("")
 assert(state("")==nil)
 `)
}

// TestPluginStateReturnsAfterRejectedUnload keeps staged snapshots separate from committed lifecycles.
func TestPluginStateReturnsAfterRejectedUnload(t *testing.T) {
	r := openTestRuntime(t, `
 package.preload.tracked=function() return {setup=function() end} end
 wraith.plugins.load("tracked")
 wraith.commands.create("stage",function()
  wraith.plugins.unload_many({"tracked"})
  assert(wraith.plugins.state("tracked")==nil)
 end)
 wraith.commands.create("verify",function() assert(wraith.plugins.state("tracked")=="loaded") end)
 `)
	for _, name := range []string{"stage", "verify"} {
		manifest := r.Manifest()
		var callback string
		for _, command := range manifest.Commands {
			if command.Name == name {
				callback = command.Callback
			}
		}
		channel, err := r.Submit(callback, "", nil, manifest.Config)
		if err != nil {
			t.Fatal(err)
		}
		result := receiveResult(t, channel)
		if result.Err != nil {
			t.Fatal(result.Err)
		}
		if name == "stage" {
			if result.Manifest == nil {
				t.Fatal("unload did not stage publication")
			}
			if err := r.Acknowledge(result.Revision, false); err != nil {
				t.Fatal(err)
			}
		} else if result.Manifest != nil {
			t.Fatal("state query mutated registrations")
		}
	}
}

// TestPluginStateUnwindsCancellationAndRetainsPurePolicy uses the existing package transaction.
func TestPluginStateUnwindsCancellationAndRetainsPurePolicy(t *testing.T) {
	e := moduleResolutionFixture(t)
	if err := e.L.DoString(`captured=wraith.plugins.state
 package.preload.cancelled=function() return {setup=function()
  assert(captured("cancelled")=="loading")
  while waiting do end
 end} end; waiting=true`); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(t.Context(), 20*time.Millisecond)
	defer cancel()
	e.L.SetContext(ctx)
	if err := e.L.DoString(`wraith.plugins.load("cancelled")`); err == nil {
		t.Fatal("setup did not reach deadline")
	}
	e.L.SetContext(t.Context())
	if err := e.L.DoString(`assert(captured("cancelled")==nil); waiting=false
 wraith.plugins.load("cancelled"); assert(captured("cancelled")=="loaded")`); err != nil {
		t.Fatal(err)
	}
	e.pureHook = true
	if err := e.L.DoString(`assert(not pcall(captured,"cancelled"))`); err != nil || e.hookViolation == nil {
		t.Fatal("state query bypassed pure hook policy", err)
	}
	e.pureHook = false
}
