package plugin

import (
	"strings"
	"testing"

	lua "github.com/yuin/gopher-lua"
)

// TestOwnedModuleCoroutineExecutionAndCleanup keeps source and failed-source disposal on the calling thread.
func TestOwnedModuleCoroutineExecutionAndCleanup(t *testing.T) {
	for _, fail := range []bool{false, true} {
		name := "success"
		if fail {
			name = "source failure"
		}
		t.Run(name, func(t *testing.T) {
			e, native, resolution, owner := moduleNativeFixture(t)
			main := e.L
			var child *lua.LState
			cleaned := false
			main.SetGlobal("add_cleanup", main.NewFunction(func(L *lua.LState) int {
				if L != child || e.L != L || e.owner != owner {
					t.Fatal("source cleanup registration lost its thread or owner")
				}
				e.addCleanup(L.CheckFunction(1))
				return 0
			}))
			main.SetGlobal("cleanup_audit", main.NewFunction(func(L *lua.LState) int {
				if L != child || e.L != L || e.owner != owner || !e.closing || L.Context().Err() != nil {
					t.Fatal("failed-source cleanup lost its thread, owner or cleanup context")
				}
				cleaned = true
				return 0
			}))
			main.SetGlobal("invoke", main.NewFunction(func(L *lua.LState) int {
				child = L
				top, context := L.GetTop(), L.Context()
				value, err := e.invokeOwnedModule(L, "target", resolution, "config", owner, native)
				if L == main || e.L != main || L.GetTop() != top || L.Context() != context || e.owner != "config" {
					t.Fatal("owned invocation did not restore its caller state")
				}
				if err != nil {
					L.RaiseError("%v", err)
				}
				L.Push(value)
				return 1
			}))
			main.SetGlobal("fail", lua.LBool(fail))
			if err := main.DoString(`package.preload.target=function()
 wraith.commands.create("thread-command",function() end)
 if fail then add_cleanup(function() cleanup_audit() end); error("source failed") end
 return 42
end
local thread=coroutine.create(function()
 local ok,value=pcall(invoke)
 assert(ok~=fail)
 if ok then assert(value==42) else assert(string.find(value,"source failed",1,true)) end
end)
assert(coroutine.resume(thread))`); err != nil {
				t.Fatal(err)
			}
			if cleaned != fail || e.L != main || main.GetTop() != 0 || len(e.cleanup) != 0 || len(e.packageCalls) != 0 || e.moduleInvocations != 0 {
				t.Fatal("coroutine invocation retained resources or execution state")
			}
			if fail && len(e.manifest.Commands) != 0 || !fail && (len(e.manifest.Commands) != 1 || e.callbackOwners[e.manifest.Commands[0].Callback] != owner) {
				t.Fatal("source command ownership or failed-source rollback changed")
			}
		})
	}
}

// TestOwnedModuleNativeMarkerSkipsMetadata preserves native recursion without poisoning a recovered outer load.
func TestOwnedModuleNativeMarkerSkipsMetadata(t *testing.T) {
	e, native, resolution, owner := moduleNativeFixture(t)
	checks := 0
	resolution.check = e.L.NewFunction(func(*lua.LState) int { checks++; return 0 })
	e.L.SetGlobal("recursive", e.L.NewFunction(func(L *lua.LState) int {
		_, err := e.invokeOwnedModule(L, "target", resolution, owner, owner, native)
		if err == nil || !strings.Contains(err.Error(), "loop or previous error") {
			t.Fatalf("native marker was not delegated: %v", err)
		}
		return 0
	}))
	if err := e.L.DoString(`package.preload.target=function() recursive(); return 42 end`); err != nil {
		t.Fatal(err)
	}
	value, err := e.invokeOwnedModule(e.L, "target", resolution, "config", owner, native)
	if err != nil || value != lua.LNumber(42) || checks != 2 || e.moduleClaims["target"].poison != "" || e.moduleClaims["target"].pending != 0 {
		t.Fatalf("recovered native cycle changed outer proof or lifecycle: %v", err)
	}
}
