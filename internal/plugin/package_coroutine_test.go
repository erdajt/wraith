package plugin

import (
	"context"
	"strings"
	"testing"

	lua "github.com/yuin/gopher-lua"
)

// TestDirectPluginCoroutinesPreserveCallerAndOwnership exercises nested native lifecycle calls.
func TestDirectPluginCoroutinesPreserveCallerAndOwnership(t *testing.T) {
	e := moduleResolutionFixture(t)
	main := e.L
	main.SetGlobal("audit", main.NewFunction(func(L *lua.LState) int {
		name := L.CheckString(1)
		thread := main
		if L.Get(2) != lua.LNil {
			thread = L.CheckThread(2)
		}
		if L != thread || e.L != thread || e.owner != e.packages[name].owner {
			L.RaiseError("plugin %s lost its caller thread or owner", name)
		}
		return 0
	}))
	if err := main.DoString(`
local expected, token = nil, {}
package.preload.child=function()
 audit("child", expected)
 return {
  setup=function(opts)
   audit("child", expected)
   assert(opts.token==token)
   wraith.commands.create("child-owned",function() end)
  end,
  value=function(value)
   audit("child", expected)
   assert(value==token and not pcall(wraith.plugins.unload,"parent"))
   return value,"discarded"
  end,
 }
end
package.preload.parent=function()
 audit("parent", expected)
 return {
  setup=function(opts)
   audit("parent", expected)
   assert(opts.token==token)
   local caller=expected
   coroutine.wrap(function()
    expected=coroutine.running()
    wraith.plugins.load("child",{token=token})
   end)()
   expected=caller
   audit("parent", expected)
   wraith.commands.create("parent-owned",function() end)
  end,
  value=function(value)
   audit("parent", expected)
   local first,second=wraith.plugins.call("child","value",value)
   assert(first==token and second==nil)
   audit("parent", expected)
   return first,"discarded"
  end,
 }
end
local child=coroutine.create(function()
 expected=coroutine.running()
 local sentinel={}
 wraith.plugins.load("parent",{token=token})
 local first,second=wraith.plugins.call("parent","value",token)
 assert(first==token and second==nil and sentinel~=token)
end)
local ok,err=coroutine.resume(child)
assert(ok,err)
expected=nil
assert(wraith.plugins.call("parent","value",token)==token)
assert(not pcall(wraith.plugins.unload,"child"))
`); err != nil {
		t.Fatal(err)
	}
	if e.L != main || e.owner != "config" || main.GetTop() != 0 || len(e.packageCalls) != 0 {
		t.Fatal("direct lifecycle call retained coroutine state")
	}
	if len(e.manifest.Commands) != 2 {
		t.Fatal("nested setup did not retain both registrations")
	}
	for _, command := range e.manifest.Commands {
		name := strings.TrimSuffix(command.Name, "-owned")
		if e.callbackOwners[command.Callback] != e.packages[name].owner {
			t.Fatalf("command %s lost its lifecycle owner", command.Name)
		}
	}
}

// TestDirectPluginCoroutineRollback keeps disposal on the caller after failure or cancellation.
func TestDirectPluginCoroutineRollback(t *testing.T) {
	for _, mode := range []string{"load", "call"} {
		for _, cancelled := range []bool{false, true} {
			label := mode + "/error"
			if cancelled {
				label = mode + "/cancelled"
			}
			t.Run(label, func(t *testing.T) {
				e := moduleResolutionFixture(t)
				main, owners, cleaned := e.L, map[string]string{}, 0
				main.SetGlobal("audit", main.NewFunction(func(L *lua.LState) int {
					name := L.CheckString(1)
					thread := main
					if caller := main.GetGlobal("caller"); caller != lua.LNil {
						thread = caller.(*lua.LState)
					}
					if L != thread || e.L != thread || L.Context().Err() != nil {
						L.RaiseError("lifecycle lost its active thread or context")
					}
					if L.OptBool(2, false) {
						if !e.closing || e.owner != owners[name] {
							L.RaiseError("rollback lost its cleanup owner")
						}
						cleaned++
					} else {
						owners[name] = e.packages[name].owner
						if e.owner != owners[name] {
							L.RaiseError("lifecycle lost its registration owner")
						}
					}
					return 0
				}))
				main.SetGlobal("mode", lua.LString(mode))
				if err := main.DoString(`
package.preload.transient=function() return {
 setup=function() audit("transient"); wraith.commands.create("transient-owned",function() end) end,
 teardown=function() audit("transient",true) end,
} end
package.preload.worker=function() return {
 setup=function()
  audit("worker"); wraith.commands.create("worker-owned",function() end)
  if mode=="load" and fail then stop(); while fail do end end
 end,
 run=function()
  audit("worker"); wraith.plugins.load("transient",{}, {dependency=false})
  if fail then stop(); while fail do end end
  return 42
 end,
 teardown=function() audit("worker",true) end,
} end
if mode=="call" then wraith.plugins.load("worker") end
function exercise()
 local child=coroutine.create(function()
  caller=coroutine.running()
  local sentinel={}
  local ok,value
  if mode=="load" then ok,value=pcall(wraith.plugins.load,"worker")
  else ok,value=pcall(wraith.plugins.call,"worker","run") end
  assert(ok~=fail and sentinel)
  if not ok then assert(tostring(value):find("intentional failure",1,true))
  elseif mode=="call" then assert(value==42) end
 end)
 local ok,err=coroutine.resume(child); assert(ok,err)
end
`); err != nil {
					t.Fatal(err)
				}
				ctx, cancel := context.WithCancel(t.Context())
				defer cancel()
				main.SetContext(ctx)
				main.SetGlobal("stop", main.NewFunction(func(L *lua.LState) int {
					if cancelled {
						cancel()
					} else {
						L.RaiseError("intentional failure")
					}
					return 0
				}))
				err := main.DoString(`fail=true; exercise()`)
				if cancelled && (err == nil || !strings.Contains(err.Error(), "context canceled")) || !cancelled && err != nil {
					t.Fatalf("failed coroutine: %v", err)
				}
				if cleaned != 1 || e.L != main || e.owner != "config" || main.GetTop() != 0 || len(e.packageCalls) != 0 {
					t.Fatalf("rollback leaked caller state or cleanup: %d", cleaned)
				}
				want := 0
				if mode == "call" {
					want = 1
				}
				if len(e.packages) != want || len(e.manifest.Commands) != want {
					t.Fatal("rollback leaked child registrations")
				}
				main.SetContext(t.Context())
				if err := main.DoString(`fail=false; exercise(); caller=nil`); err != nil {
					t.Fatalf("retry after rollback: %v", err)
				}
			})
		}
	}
}

// TestDirectPluginCoroutineResumesWithCallbackDeadline refreshes a startup-created thread before use.
func TestDirectPluginCoroutineResumesWithCallbackDeadline(t *testing.T) {
	r := openTestRuntime(t, `
package.preload.runtime_worker=function()
 local expected
 return {
  setup=function(opts)
   expected=opts.thread
   assert(coroutine.running()==expected)
   wraith.commands.create("runtime-owned",function() end)
  end,
  value=function(thread,block)
   assert(coroutine.running()==thread and thread==expected)
   while block do end
   return thread
  end,
 }
end
local function new_thread()
 return coroutine.create(function(block)
  local thread=coroutine.running()
  wraith.plugins.load("runtime_worker",{thread=thread})
  assert(wraith.plugins.call("runtime_worker","value",thread,block)==thread)
 end)
end
local thread=new_thread()
wraith.commands.create("resume",function(ctx)
 if coroutine.status(thread)=="dead" then thread=new_thread() end
 local ok,err=coroutine.resume(thread,ctx.block)
 assert(ok,err)
end)
`)
	m := r.Manifest()
	for _, blocked := range []bool{true, false} {
		pending, err := r.Submit(m.Commands[0].Callback, "", map[string]any{"block": blocked}, m.Config)
		if err != nil {
			t.Fatal(err)
		}
		result := receiveResult(t, pending)
		if blocked {
			if result.Err == nil || !strings.Contains(result.Err.Error(), "deadline") || result.Manifest != nil || len(result.Effects) != 0 {
				t.Fatalf("coroutine escaped its callback deadline: %+v", result)
			}
			if len(r.Manifest().Commands) != 1 {
				t.Fatal("timed-out coroutine published plugin commands")
			}
			continue
		}
		if result.Err != nil || result.Manifest == nil || len(result.Manifest.Commands) != 2 {
			t.Fatalf("fresh coroutine could not retry: %+v", result)
		}
		if err := r.Acknowledge(result.Revision, true); err != nil {
			t.Fatal(err)
		}
	}
}
