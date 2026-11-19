package plugin

import (
	"testing"
)

func TestManagerLazyKeysRunOnlyAfterAcceptedActivation(t *testing.T) {
	for _, target := range []string{"initial", "changed"} {
		t.Run(target, func(t *testing.T) {
			r := openManagerRuntime(t, `local loader,count=nil,0
 package.preload.app=function() return {} end
 package.preload.manager_test=function() return {setup=function() end,
  configure=function()
   local graph=require("wraith.manager.spec").normalize({{name="app",dir="/tmp/app",keys={{"z",function(ctx)
    count=count+1; wraith.notify(count..":"..ctx.count)
   end}}}})
   loader=require("wraith.manager.loader").new(graph,{app="/tmp/app"},"manager_test")
   loader.initialize()
  end,trigger=function(...) loader.trigger(...) end} end
 wraith.plugins.load("manager_test"); wraith.plugins.call("manager_test","configure")
 wraith.commands.create("check",function() assert(count==0) end)
 `)
			var key, check string
			for _, binding := range r.Manifest().Config.Keymaps {
				if binding.Keys == "z" && binding.Mode == "chat.normal" {
					key = binding.Action
				}
			}
			for _, command := range r.Manifest().Commands {
				if command.Name == "check" {
					check = command.Callback
				}
			}
			for _, accepted := range []bool{false, true} {
				ch, err := r.Submit(key, "", map[string]any{"count": 3, "target": "initial"}, r.Manifest().Config)
				if err != nil {
					t.Fatal(err)
				}
				result := receiveResult(t, ch)
				if result.Err != nil || result.Manifest == nil || len(result.Effects) != 1 || result.Effects[0].Name != "schedule.callback" {
					t.Fatalf("key was not deferred: %+v", result)
				}
				if err := r.Acknowledge(result.Revision, accepted); err != nil {
					t.Fatal(err)
				}
				pending := result.Effects[0].Argument
				ch, err = r.Submit(check, "", nil, r.Manifest().Config)
				if err != nil {
					t.Fatal(err)
				}
				if result := receiveResult(t, ch); result.Err != nil {
					t.Fatalf("key ran before host handoff: %v", result.Err)
				}
				if !accepted {
					continue
				}
				ch, err = r.Submit(pending, "", map[string]any{"target": target}, r.Manifest().Config)
				if err != nil {
					t.Fatal(err)
				}
				result = receiveResult(t, ch)
				if target == "changed" {
					if result.Err == nil || len(result.Effects) != 0 {
						t.Fatalf("stale key was delivered: %+v", result)
					}
					continue
				}
				if result.Err != nil || len(result.Effects) != 1 || result.Effects[0].Argument != "1:3" {
					t.Fatalf("key count or single delivery changed: %+v", result)
				}
			}
		})
	}
}

func TestManagerReloadKeepsOldAndNewCleanupClosuresSeparate(t *testing.T) {
	r := openManagerRuntime(t, `local loader,serial,trace=nil,0,{}
 package.preload.app=function() return {setup=function()
  serial=serial+1; local current=serial
  wraith.commands.create("alive",function() wraith.notify(current) end)
  return function() trace[#trace+1]="cleanup"..current end
 end,teardown=function() trace[#trace+1]="teardown" end} end
 package.preload.manager_test=function() return {setup=function() end,
  configure=function()
   local graph=require("wraith.manager.spec").normalize({{name="app",main="app",dir="/tmp/app",opts={},lazy=true,init=function()
    return function() trace[#trace+1]="init" end
   end}})
   loader=require("wraith.manager.loader").new(graph,{app="/tmp/app"},"manager_test"); loader.initialize(); loader.load("app")
  end,reload=function() loader.unload("app"); loader.load("app") end,
  check=function() assert(table.concat(trace,",")=="cleanup1,teardown,init",table.concat(trace,",")) end,
  remove=function() loader.unload_all() end,
  final=function() assert(table.concat(trace,",")=="cleanup1,teardown,init,cleanup2,teardown,init",table.concat(trace,",")) end} end
 wraith.plugins.load("manager_test"); wraith.plugins.call("manager_test","configure")
 wraith.commands.create("operate",function(ctx) wraith.plugins.call("manager_test",ctx.argument) end,"operation",{arguments=true})
 `)
	var operation string
	for _, command := range r.Manifest().Commands {
		if command.Name == "operate" {
			operation = command.Callback
		}
	}
	for _, method := range []string{"reload", "check", "remove", "final"} {
		ch, err := r.Submit(operation, "", map[string]any{"argument": method}, r.Manifest().Config)
		if err != nil {
			t.Fatal(err)
		}
		result := receiveResult(t, ch)
		if result.Err != nil {
			t.Fatalf("%s: %v", method, result.Err)
		}
		if result.Manifest != nil {
			if err := r.Acknowledge(result.Revision, true); err != nil {
				t.Fatal(err)
			}
		}
	}
}

func TestManagerFailedAndRejectedSetupCleanEachActivationOnce(t *testing.T) {
	r := openManagerRuntime(t, `local loader,attempt,cleanup,teardown= nil,0,0,0
 package.preload.app=function() return {setup=function()
  attempt=attempt+1
  if attempt==1 then error("first setup failed") end
  return function() cleanup=cleanup+1 end
 end,teardown=function() teardown=teardown+1 end} end
 package.preload.manager_test=function() return {setup=function() end,
  configure=function()
   local graph=require("wraith.manager.spec").normalize({{name="app",main="app",dir="/tmp/app",opts={},lazy=true}})
   loader=require("wraith.manager.loader").new(graph,{app="/tmp/app"},"manager_test"); loader.initialize()
  end,load=function() loader.load("app") end,remove=function() loader.unload_all() end,
  failed=function() assert(cleanup==0 and teardown==1) end,
  rejected=function() assert(cleanup==1 and teardown==2 and not loader.is_loaded("app")) end,
  final=function() assert(cleanup==2 and teardown==3) end} end
 wraith.plugins.load("manager_test"); wraith.plugins.call("manager_test","configure")
 wraith.commands.create("operate",function(ctx) wraith.plugins.call("manager_test",ctx.argument) end,"operation",{arguments=true})
 `)
	id := r.Manifest().Commands[0].Callback
	for index, method := range []string{"load", "failed", "load", "rejected", "load", "remove", "final"} {
		ch, err := r.Submit(id, "", map[string]any{"argument": method}, r.Manifest().Config)
		if err != nil {
			t.Fatal(err)
		}
		result := receiveResult(t, ch)
		if index == 0 {
			if result.Err == nil || result.Manifest != nil {
				t.Fatalf("failed setup was accepted: %+v", result)
			}
			continue
		}
		if result.Err != nil {
			t.Fatalf("%s: %v", method, result.Err)
		}
		if result.Manifest != nil {
			if err := r.Acknowledge(result.Revision, index != 2); err != nil {
				t.Fatal(err)
			}
		}
	}
}
