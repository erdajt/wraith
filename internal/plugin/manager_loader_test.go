package plugin

import (
	"testing"
)

func TestManagerLoaderOwnsInitActivationAndDependencyCleanup(t *testing.T) {
	r := openManagerRuntime(t, `local loader
 local graph=require("wraith.manager.spec").normalize({
  {name="dependency",main="dependency",dir="/tmp/dependency",lazy=true,opts={}},
  {name="app",main="app",dir="/tmp/app",opts={},dependencies={"dependency"},cmd="app",init=function()
   wraith.commands.create("initialized",function() end)
   return function() print("init cleanup") end
  end}})
 package.preload.dependency=function() return {setup=function() print("dependency setup") end,teardown=function() print("dependency teardown") end} end
 package.preload.app=function() return {setup=function()
  wraith.commands.create("app",function(ctx) wraith.notify(ctx.argument) end,"application",{arguments=true})
  return function() print("app cleanup") end
 end,teardown=function() print("app teardown") end} end
 package.preload.manager_test=function() return {setup=function() end,
  configure=function()
   loader=require("wraith.manager.loader").new(graph,{app="/tmp/app",dependency="/tmp/dependency"},"manager_test")
   loader.initialize()
  end,trigger=function(...) loader.trigger(...) end,
  inspect=function()
   assert(loader.is_loaded("app") and loader.is_loaded("dependency"))
   assert(not pcall(loader.unload,"dependency"))
   loader.unload("app"); loader.unload("app"); loader.unload_all()
  end} end
 wraith.plugins.load("manager_test")
 wraith.plugins.call("manager_test","configure")
 assert(not pcall(wraith.plugins.unload,"manager_test"))
 wraith.commands.create("inspect",function() wraith.plugins.call("manager_test","inspect") end)
 `)
	m := r.Manifest()
	var trigger string
	for _, command := range m.Commands {
		if command.Name == "app" {
			trigger = command.Callback
		}
	}
	ch, err := r.Submit(trigger, "", map[string]any{"argument": "original arguments"}, m.Config)
	if err != nil {
		t.Fatal(err)
	}
	result := receiveResult(t, ch)
	if result.Err != nil || result.Manifest == nil || len(result.Effects) != 2 || result.Effects[1].Name != "command.app" || result.Effects[1].Argument != "original arguments" {
		t.Fatalf("lazy activation or first command handoff failed: %+v", result)
	}
	if err := r.Acknowledge(result.Revision, true); err != nil {
		t.Fatal(err)
	}
	m = r.Manifest()
	for _, command := range m.Commands {
		if command.Name == "inspect" {
			ch, err = r.Submit(command.Callback, "", nil, m.Config)
		}
	}
	if err != nil {
		t.Fatal(err)
	}
	result = receiveResult(t, ch)
	if result.Err != nil || result.Manifest == nil {
		t.Fatalf("dependency guard or unload failed: %+v", result)
	}
	if err := r.Acknowledge(result.Revision, true); err != nil {
		t.Fatal(err)
	}
	for _, command := range r.Manifest().Commands {
		if command.Name == "app" || command.Name == "initialized" {
			t.Fatalf("unloaded package left registrations: %+v", command)
		}
	}
}

// TestManagerLazyCommandRetriesAfterFailedAndRejectedActivation preserves the original trigger across both rollback paths.
func TestManagerLazyCommandRetriesAfterFailedAndRejectedActivation(t *testing.T) {
	r := openManagerRuntime(t, `local loader, attempts= nil,0
 package.preload.app=function() return {setup=function()
  attempts=attempts+1
  if attempts==1 then
   wraith.commands.create("partial",function() end)
   error("setup failed")
  end
  wraith.commands.create("app",function(ctx) wraith.notify(attempts..":"..ctx.argument) end,"app",{arguments=true})
 end} end
 package.preload.manager_test=function() return {setup=function() end,
  configure=function()
   local graph=require("wraith.manager.spec").normalize({{name="app",main="app",dir="/tmp/app",opts={},cmd="app"}})
   loader=require("wraith.manager.loader").new(graph,{app="/tmp/app"},"manager_test")
   loader.initialize()
  end,trigger=function(...) loader.trigger(...) end} end
 wraith.plugins.load("manager_test")
 wraith.plugins.call("manager_test","configure")
 `)
	submit := func(argument string) Result {
		t.Helper()
		m := r.Manifest()
		var id string
		for _, command := range m.Commands {
			if command.Name == "app" {
				id = command.Callback
			}
		}
		ch, err := r.Submit(id, "", map[string]any{"argument": argument}, m.Config)
		if err != nil {
			t.Fatal(err)
		}
		result := receiveResult(t, ch)
		return result
	}
	failed := submit("failed")
	if failed.Err == nil || failed.Manifest != nil || len(failed.Effects) != 0 {
		t.Fatalf("failed setup published changes: %+v", failed)
	}
	for _, command := range r.Manifest().Commands {
		if command.Name == "partial" {
			t.Fatal("failed setup retained its partial command")
		}
	}

	first := submit("rejected")
	if first.Err != nil || first.Manifest == nil || r.Acknowledge(first.Revision, false) != nil {
		t.Fatalf("first activation did not request publication: %+v", first)
	}
	second := submit("accepted")
	if second.Err != nil || second.Manifest == nil || r.Acknowledge(second.Revision, true) != nil || len(second.Effects) != 1 || second.Effects[0].Argument != "accepted" {
		t.Fatalf("restored lazy stub did not retry original command: %+v", second)
	}
	final := submit("delivered")
	if final.Err != nil || len(final.Effects) != 1 || final.Effects[0].Argument != "3:delivered" {
		t.Fatalf("final package callback did not replace restored stub: %+v", final)
	}
}
