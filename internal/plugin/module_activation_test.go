package plugin

import (
	"strings"
	"testing"

	lua "github.com/yuin/gopher-lua"
)

// moduleActivationFixture routes one source through the real owned invocation path.
func moduleActivationFixture(t *testing.T) (*engine, nativeModules, *moduleResolution) {
	t.Helper()
	e := moduleResolutionFixture(t)
	native, err := captureNativeModules(e.L)
	if err != nil {
		t.Fatal(err)
	}
	e.owner = "resolver"
	if _, err = e.addModuleProvider(e.L.NewFunction(func(*lua.LState) int { return 0 })); err != nil {
		t.Fatal(err)
	}
	if err = e.L.DoString(`function activate(name) assert(name=="target"); activations=(activations or 0)+1; wraith.plugins.load("provider",{},{dependency=false}) end
function verify(name,phase) assert(name=="target"); assert(phase=="validate" or phase=="before" or phase=="after"); return false end
package.preload.target=function() runs=(runs or 0)+1; wraith.commands.create("source",function() end); return {} end
package.preload.provider=function() return {
setup=function() local co=coroutine.create(function() assert(type(require("target"))=="table") end); assert(coroutine.resume(co)); if fail_setup then error("setup failed") end end,
teardown=function() cleanup_state() end} end`); err != nil {
		t.Fatal(err)
	}
	resolution := &moduleResolution{provider: e.moduleProviders[0], plugin: "provider", activate: e.L.GetGlobal("activate").(*lua.LFunction), check: e.L.GetGlobal("verify").(*lua.LFunction)}
	e.L.SetGlobal("cleanup_state", e.L.NewFunction(func(*lua.LState) int { return 0 }))
	e.L.SetGlobal("require", e.L.NewFunction(func(L *lua.LState) int {
		name := L.CheckString(1)
		if name == "target" {
			value, err := e.invokeOwnedModule(L, name, resolution, e.owner, e.packages["provider"].owner, native)
			if err != nil {
				L.RaiseError("%v", err)
			}
			L.Push(value)
		} else {
			if err := L.CallByParam(lua.P{Fn: native.require, NRet: 1, Protect: true}, lua.LString(name)); err != nil {
				L.RaiseError("%v", err)
			}
		}
		return 1
	}))
	e.owner = "config"
	return e, native, resolution
}

// TestModuleActivationRunsCoroutineLifecycle keeps nested source and failed teardown on their callers.
func TestModuleActivationRunsCoroutineLifecycle(t *testing.T) {
	for _, failure := range []bool{false, true} {
		t.Run(map[bool]string{false: "success", true: "failed setup"}[failure], func(t *testing.T) {
			e, native, resolution := moduleActivationFixture(t)
			main := e.L
			var caller *lua.LState
			cleaned := false
			main.SetGlobal("fail_setup", lua.LBool(failure))
			main.SetGlobal("cleanup_state", main.NewFunction(func(L *lua.LState) int {
				if L == main || L != caller || e.L != L || !e.closing {
					t.Fatal("cleanup left the caller coroutine")
				}
				cleaned = true
				return 0
			}))
			main.SetGlobal("begin", main.NewFunction(func(L *lua.LState) int {
				caller = L
				owner, err := e.activateModuleProvider(L, "target", resolution, e.owner, native)
				if failure {
					if err == nil || owner != "" {
						t.Fatal("failed setup appeared active")
					}
				} else if err != nil || owner != e.packages["provider"].owner {
					t.Fatalf("activation: %v", err)
				}
				return 0
			}))
			if err := main.DoString(`local co=coroutine.create(function() begin() end); local ok,err=coroutine.resume(co); assert(ok,err)`); err != nil {
				t.Fatal(err)
			}
			if e.L != main || e.owner != "config" || len(e.packageCalls) != 0 || len(e.moduleActivating) != 0 || e.moduleClaims["target"].pending != 0 || main.GetTop() != 0 {
				t.Fatal("activation leaked caller state")
			}
			if failure {
				if !cleaned || len(e.packages) != 0 || len(e.manifest.Commands) != 0 || native.loaded.RawGetString("target") != lua.LNil || e.moduleClaims["target"].poison != "" {
					t.Fatal("setup rollback retained source resources")
				}
				main.SetGlobal("fail_setup", lua.LFalse)
				if _, err := e.activateModuleProvider(main, "target", resolution, "config", native); err != nil || main.GetGlobal("runs") != lua.LNumber(2) {
					t.Fatalf("clean setup retry: %v", err)
				}
			} else {
				owner := e.packages["provider"].owner
				if len(e.manifest.Commands) != 1 || e.callbackOwners[e.manifest.Commands[0].Callback] != owner || e.moduleExecutions["target"].plugin != "provider" {
					t.Fatal("source lost lifecycle ownership")
				}
				if _, err := e.activateModuleProvider(main, "target", resolution, "config", native); err != nil || main.GetGlobal("activations") != lua.LNumber(1) {
					t.Fatalf("ready provider activated again: %v", err)
				}
			}
		})
	}
}

// TestModuleActivationRejectsValidationMutation stops before activating a changed cache.
func TestModuleActivationRejectsValidationMutation(t *testing.T) {
	e, native, resolution := moduleActivationFixture(t)
	resolution.check = e.L.NewFunction(func(*lua.LState) int { native.loaded.RawSetString("target", lua.LTrue); return 0 })
	if _, err := e.activateModuleProvider(e.L, "target", resolution, "config", native); err == nil || e.L.GetGlobal("activations") != lua.LNil || e.moduleClaims["target"].poison == "" {
		t.Fatal("validation mutation reached activation")
	}
	if native.loaded.RawGetString("target") != lua.LTrue {
		t.Fatal("discarded unverified cache")
	}
}

// TestModuleActivationDistinguishesLoadingAndRetirement avoids partial or stale generations.
func TestModuleActivationDistinguishesLoadingAndRetirement(t *testing.T) {
	e, native, resolution := moduleActivationFixture(t)
	owner, err := e.activateModuleProvider(e.L, "target", resolution, "config", native)
	if err != nil {
		t.Fatal(err)
	}
	record := e.packages["provider"]
	record.loading = true
	e.packages["provider"] = record
	if _, err := e.activateModuleProvider(e.L, "target", resolution, "config", native); err == nil {
		t.Fatal("entered foreign loading provider")
	}
	if got, err := e.activateModuleProvider(e.L, "target", resolution, owner, native); err != nil || got != owner {
		t.Fatalf("self-loading access: %v", err)
	}
	record.loading = false
	e.packages["provider"] = record
	if err := e.unloadPlugin("provider"); err != nil {
		t.Fatal(err)
	}
	if _, err := e.activateModuleProvider(e.L, "target", resolution, "config", native); err == nil || !strings.Contains(err.Error(), "retirement") {
		t.Fatal("reactivated before retirement acceptance")
	}
}

// TestModuleActivationBoundsMissingTargetsAndRecursion rejects continuations that never publish a lifecycle.
func TestModuleActivationBoundsMissingTargetsAndRecursion(t *testing.T) {
	for _, mode := range []string{"missing", "recursive", "depth", "stale"} {
		t.Run(mode, func(t *testing.T) {
			e, native, resolution := moduleActivationFixture(t)
			calls := 0
			resolution.activate = e.L.NewFunction(func(L *lua.LState) int {
				calls++
				if mode == "recursive" {
					if _, err := e.activateModuleProvider(L, "target", resolution, e.owner, native); err != nil {
						L.RaiseError("%v", err)
					}
				}
				if mode == "stale" {
					e.moduleProviders = nil
				}
				return 0
			})
			if mode == "depth" {
				e.moduleActivating = make([]string, 32)
			}
			if _, err := e.activateModuleProvider(e.L, "target", resolution, "config", native); err == nil {
				t.Fatal("accepted incomplete activation")
			}
			if mode == "depth" && calls != 0 || mode != "depth" && calls != 1 || len(e.moduleProviders) != 1 || e.moduleClaims["target"].pending != 0 {
				t.Fatal("activation bound or rollback failed")
			}
		})
	}
}

// TestModuleActivationPoisonsFailedTableReplacement preserves foreign state after activation errors.
func TestModuleActivationPoisonsFailedTableReplacement(t *testing.T) {
	e, native, resolution := moduleActivationFixture(t)
	registry := e.L.Get(lua.RegistryIndex).(*lua.LTable)
	replacement := e.L.NewTable()
	resolution.activate = e.L.NewFunction(func(L *lua.LState) int {
		registry.RawSetString("_LOADED", replacement)
		L.RaiseError("activation failed")
		return 0
	})
	if _, err := e.activateModuleProvider(e.L, "target", resolution, "config", native); err == nil || e.moduleClaims["target"].poison == "" {
		t.Fatal("failed activation lost table mutation proof")
	}
	if registry.RawGetString("_LOADED") != replacement {
		t.Fatal("discarded foreign cache table")
	}
	registry.RawSetString("_LOADED", native.loaded)
	calls := 0
	resolution.activate = e.L.NewFunction(func(*lua.LState) int { calls++; return 0 })
	if _, err := e.activateModuleProvider(e.L, "target", resolution, "config", native); err == nil || calls != 0 {
		t.Fatal("poisoned activation retried")
	}
}

// TestModuleActivationPoisonsFailedCacheWrites rejects unverified exports after rollback.
func TestModuleActivationPoisonsFailedCacheWrites(t *testing.T) {
	for _, value := range []lua.LValue{lua.LTrue, lua.LFalse} {
		t.Run(value.String(), func(t *testing.T) {
			e, native, resolution := moduleActivationFixture(t)
			resolution.activate = e.L.NewFunction(func(L *lua.LState) int {
				native.loaded.RawSetString("target", value)
				L.RaiseError("activation failed")
				return 0
			})
			if _, err := e.activateModuleProvider(e.L, "target", resolution, "config", native); err == nil || e.moduleClaims["target"].poison == "" || native.loaded.RawGetString("target") != value {
				t.Fatal("failed activation lost unverified export")
			}
			native.loaded.RawSetString("target", lua.LNil)
			calls := 0
			resolution.activate = e.L.NewFunction(func(*lua.LState) int { calls++; return 0 })
			if _, err := e.activateModuleProvider(e.L, "target", resolution, "config", native); err == nil || calls != 0 {
				t.Fatal("poisoned export retried")
			}
		})
	}
}

// TestModuleActivationPoisonsCleanupMutation checks native state after outer activation rollback.
func TestModuleActivationPoisonsCleanupMutation(t *testing.T) {
	e, native, resolution := moduleActivationFixture(t)
	registry := e.L.Get(lua.RegistryIndex).(*lua.LTable)
	replacement := e.L.NewTable()
	if err := e.L.DoString(`function failed_activate(name) activate(name); error("activation failed after setup") end`); err != nil {
		t.Fatal(err)
	}
	resolution.activate = e.L.GetGlobal("failed_activate").(*lua.LFunction)
	e.L.SetGlobal("cleanup_state", e.L.NewFunction(func(*lua.LState) int { registry.RawSetString("_LOADED", replacement); return 0 }))
	if _, err := e.activateModuleProvider(e.L, "target", resolution, "config", native); err == nil || e.moduleClaims["target"].poison == "" || registry.RawGetString("_LOADED") != replacement {
		t.Fatal("cleanup mutation escaped activation proof")
	}
}
