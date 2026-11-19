package plugin

import (
	"context"
	"fmt"
	"strings"
	"testing"

	lua "github.com/yuin/gopher-lua"
)

// TestOwnedModuleReservesCapacityBeforeNestedSources prevents a child import from consuming its caller's receipt slot.
func TestOwnedModuleReservesCapacityBeforeNestedSources(t *testing.T) {
	e, native, resolution, owner := moduleNativeFixture(t)
	for index := range 1023 {
		release, err := e.claimModule(fmt.Sprintf("reserved%d", index))
		if err != nil {
			t.Fatal(err)
		}
		release()
	}
	innerRan := false
	native.preload.RawSetString("inner", e.L.NewFunction(func(*lua.LState) int { innerRan = true; return 0 }))
	native.preload.RawSetString("outer", e.L.NewFunction(func(L *lua.LState) int {
		if _, err := e.invokeOwnedModule(L, "inner", resolution, owner, owner, native); err == nil || !strings.Contains(err.Error(), "exceed 1024") {
			t.Fatalf("nested import consumed the final slot: %v", err)
		}
		L.Push(lua.LNumber(42))
		return 1
	}))
	value, err := e.invokeOwnedModule(e.L, "outer", resolution, "config", owner, native)
	if err != nil || value != lua.LNumber(42) || innerRan || e.moduleClaims["outer"].pending != 0 || len(e.moduleClaims) != 1024 || len(e.moduleExecutions) != 1 {
		t.Fatalf("outer execution lost its reserved capacity: %v", err)
	}
}

// TestOwnedModuleProofDistinguishesNilExecutions requires fresh source evidence for every native execution.
func TestOwnedModuleProofDistinguishesNilExecutions(t *testing.T) {
	for _, previous := range []bool{false, true} {
		t.Run(map[bool]string{false: "first load", true: "after verified nil"}[previous], func(t *testing.T) {
			e, native, resolution, owner := moduleNativeFixture(t)
			serial := 0
			native.preload.RawSetString("owned", e.L.NewFunction(func(*lua.LState) int {
				serial++
				native.loaded.RawSetString("owned", lua.LNil)
				return 0
			}))
			resolution.check = e.L.NewFunction(func(L *lua.LState) int {
				if L.CheckString(2) == "before" {
					L.Push(lua.LNumber(serial))
					return 1
				}
				if !L.CheckBool(4) || serial <= L.CheckInt(3) {
					L.RaiseError("selected source did not execute")
				}
				return 0
			})
			if previous {
				if value, err := e.invokeOwnedModule(e.L, "owned", resolution, "config", owner, native); err != nil || value != lua.LNil {
					t.Fatalf("legitimate nil export: %v", err)
				}
			}
			foreign := e.L.NewFunction(func(*lua.LState) int {
				native.loaded.RawSetString("owned", lua.LNil)
				return 0
			})
			loaders := e.L.GetField(e.L.Get(lua.RegistryIndex), "_LOADERS").(*lua.LTable)
			loaders.RawSetInt(1, e.L.NewFunction(func(L *lua.LState) int { L.Push(foreign); return 1 }))
			if _, err := e.invokeOwnedModule(e.L, "owned", resolution, "config", owner, native); err == nil || !strings.Contains(err.Error(), "selected source did not execute") {
				t.Fatalf("foreign nil result was accepted: %v", err)
			}
			if e.moduleClaims["owned"].poison == "" || native.loaded.RawGetString("owned") != lua.LNil {
				t.Fatal("foreign nil execution lost its failed proof")
			}
		})
	}
}

// TestOwnedModulePrecheckMutationStopsExecution poisons unexpected cache writes before a loader runs.
func TestOwnedModulePrecheckMutationStopsExecution(t *testing.T) {
	e, native, resolution, owner := moduleNativeFixture(t)
	ran := false
	native.preload.RawSetString("owned", e.L.NewFunction(func(*lua.LState) int { ran = true; return 0 }))
	resolution.check = e.L.NewFunction(func(*lua.LState) int {
		native.loaded.RawSetString("owned", lua.LString("foreign"))
		return 0
	})
	if _, err := e.invokeOwnedModule(e.L, "owned", resolution, "config", owner, native); err == nil || ran {
		t.Fatal("source ran after precheck changed its cache")
	}
	if e.moduleClaims["owned"].poison == "" || native.loaded.RawGetString("owned") != lua.LString("foreign") {
		t.Fatal("precheck mutation was cleared or became retryable")
	}
}

// TestOwnedModulePoisonsFailedPostchecks preserves unverified exports while discarding registrations.
func TestOwnedModulePoisonsFailedPostchecks(t *testing.T) {
	for _, failure := range []string{"error", "cancel", "cache", "metatable", "registry", "generation"} {
		t.Run(failure, func(t *testing.T) {
			e, native, resolution, owner := moduleNativeFixture(t)
			if err := e.L.DoString(`package.preload.owned=function()
 wraith.commands.create("from-source",function() end)
 return {ready=true}
end`); err != nil {
				t.Fatal(err)
			}
			ctx, cancel := context.WithCancel(t.Context())
			defer cancel()
			e.L.SetContext(ctx)
			checks := 0
			resolution.check = e.L.NewFunction(func(L *lua.LState) int {
				checks++
				if checks == 2 {
					switch failure {
					case "error":
						L.RaiseError("postcheck failed")
					case "cancel":
						cancel()
					case "cache":
						native.loaded.RawSetString("owned", lua.LString("foreign"))
					case "metatable":
						L.SetMetatable(native.loaded, L.NewTable())
					case "registry":
						L.Get(lua.RegistryIndex).(*lua.LTable).RawSetString("_LOADED", L.NewTable())
					case "generation":
						record := e.packages[resolution.plugin]
						record.owner = "replaced"
						e.packages[resolution.plugin] = record
					}
				}
				return 0
			})
			if value, err := e.invokeOwnedModule(e.L, "owned", resolution, "config", owner, native); err == nil || value != lua.LNil {
				t.Fatal("unverified source result escaped")
			}
			claim := e.moduleClaims["owned"]
			if checks != 2 || claim.poison == "" || claim.pending != 0 || len(e.manifest.Commands) != 0 || len(e.moduleExecutions) != 0 {
				t.Fatal("postcheck retained registrations or lost durable poison")
			}
			exposed := native.loaded.RawGetString("owned")
			if exposed == lua.LNil || failure == "cache" && exposed != lua.LString("foreign") {
				t.Fatal("unverified cache value was discarded")
			}
			e.L.SetContext(t.Context())
			e.L.SetMetatable(native.loaded, lua.LNil)
			e.L.Get(lua.RegistryIndex).(*lua.LTable).RawSetString("_LOADED", native.loaded)
			if _, err := e.invokeOwnedModule(e.L, "owned", resolution, "config", owner, native); err == nil || !strings.Contains(err.Error(), "poisoned") || checks != 2 {
				t.Fatalf("failed proof was retried: %v", err)
			}
		})
	}
}

// TestOwnedModuleCachedCancellationRemainsRetryable keeps prior receipts when no source ran.
func TestOwnedModuleCachedCancellationRemainsRetryable(t *testing.T) {
	e, native, resolution, owner := moduleNativeFixture(t)
	if err := e.L.DoString(`package.preload.owned=function() return {ready=true} end`); err != nil {
		t.Fatal(err)
	}
	value, err := e.invokeOwnedModule(e.L, "owned", resolution, "config", owner, native)
	if err != nil {
		t.Fatal(err)
	}
	receipt := e.moduleExecutions["owned"]
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	e.L.SetContext(ctx)
	checks := 0
	resolution.check = e.L.NewFunction(func(*lua.LState) int {
		checks++
		if checks == 2 {
			cancel()
		}
		return 0
	})
	if _, err := e.invokeOwnedModule(e.L, "owned", resolution, "config", owner, native); err == nil {
		t.Fatal("ignored canceled postcheck")
	}
	if native.loaded.RawGetString("owned") != value || e.moduleExecutions["owned"] != receipt || e.moduleClaims["owned"].poison != "" {
		t.Fatal("cached cancellation invalidated a verified export")
	}
	e.L.SetContext(t.Context())
	if got, err := e.invokeOwnedModule(e.L, "owned", resolution, "config", owner, native); err != nil || got != value {
		t.Fatalf("cached retry failed: %v", err)
	}
}

// TestOwnedModuleRejectsCacheInterception never interprets synthesized cache hits as owned executions.
func TestOwnedModuleRejectsCacheInterception(t *testing.T) {
	e, native, resolution, owner := moduleNativeFixture(t)
	called := false
	meta := e.L.NewTable()
	meta.RawSetString("__index", e.L.NewFunction(func(L *lua.LState) int {
		called = true
		L.Push(lua.LTrue)
		return 1
	}))
	e.L.SetMetatable(native.loaded, meta)
	if _, err := e.invokeOwnedModule(e.L, "owned", resolution, "config", owner, native); err == nil || called || len(e.moduleClaims) != 0 {
		t.Fatal("cache interception passed preflight")
	}
	e.closing = true
	native.loaded.RawSetString("owned", lua.LTrue)
	if _, err := e.closingModuleValue(e.L, "owned", native.pkg, native.loaded, native.preload); err == nil || called {
		t.Fatal("cleanup accepted an intercepted cache")
	}
}

// TestOwnedModuleDependencyFailureClearsOnlyFreshSources retains preexisting provider cache on a rejected edge.
func TestOwnedModuleDependencyFailureClearsOnlyFreshSources(t *testing.T) {
	for _, cached := range []bool{false, true} {
		t.Run(map[bool]string{false: "source", true: "cached"}[cached], func(t *testing.T) {
			e, native, resolution, owner := moduleNativeFixture(t)
			if err := e.L.DoString(`package.preload.consumer=function() return {setup=function() end} end
wraith.plugins.load("consumer")
package.preload.owned=function() return {ready=true} end`); err != nil {
				t.Fatal(err)
			}
			if cached {
				if _, err := e.invokeOwnedModule(e.L, "owned", resolution, "config", owner, native); err != nil {
					t.Fatal(err)
				}
			}
			before := native.loaded.RawGetString("owned")
			if err := e.dependOnPackage(owner, "consumer"); err != nil {
				t.Fatal(err)
			}
			consumer := e.packages["consumer"].owner
			if _, err := e.invokeOwnedModule(e.L, "owned", resolution, consumer, owner, native); err == nil || !strings.Contains(err.Error(), "cycle") {
				t.Fatalf("dependency cycle escaped: %v", err)
			}
			if native.loaded.RawGetString("owned") != before || e.moduleClaims["owned"].poison != "" || len(e.packages["consumer"].dependencies) != 0 {
				t.Fatal("dependency rollback crossed its savepoint")
			}
		})
	}
}
