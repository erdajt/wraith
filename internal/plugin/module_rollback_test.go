package plugin

import (
	"strings"
	"testing"

	lua "github.com/yuin/gopher-lua"
)

// TestOwnedModuleRollbackRetainsCleanupMutation rejects exports written after native source failure.
func TestOwnedModuleRollbackRetainsCleanupMutation(t *testing.T) {
	for _, mode := range []string{"true", "false", "registry", "metatable"} {
		t.Run(mode, func(t *testing.T) {
			e, native, resolution, owner := moduleNativeFixture(t)
			registry := e.L.Get(lua.RegistryIndex).(*lua.LTable)
			replacement := e.L.NewTable()
			e.L.SetGlobal("add_cleanup", e.L.NewFunction(func(L *lua.LState) int {
				e.addCleanup(L.CheckFunction(1))
				return 0
			}))
			e.L.SetGlobal("mutate_cleanup", e.L.NewFunction(func(L *lua.LState) int {
				if e.owner != owner || !e.closing {
					t.Fatal("cleanup lost provider ownership")
				}
				switch mode {
				case "true":
					native.loaded.RawSetString("target", lua.LTrue)
				case "false":
					native.loaded.RawSetString("target", lua.LFalse)
				case "registry":
					registry.RawSetString("_LOADED", replacement)
				case "metatable":
					L.SetMetatable(native.loaded, replacement)
				}
				L.RaiseError("cleanup failed")
				return 0
			}))
			if err := e.L.DoString(`package.preload.target=function()
add_cleanup(function() mutate_cleanup() end)
wraith.commands.create("discarded",function() end)
error("source failed")
end`); err != nil {
				t.Fatal(err)
			}
			_, err := e.invokeOwnedModule(e.L, "target", resolution, "config", owner, native)
			if err == nil || !strings.Contains(err.Error(), "source failed") || !strings.Contains(err.Error(), "cleanup failed") || !strings.Contains(err.Error(), "/reload") {
				t.Fatalf("rollback lost failure context: %v", err)
			}
			if e.moduleClaims["target"].poison == "" || len(e.manifest.Commands) != 0 || len(e.cleanup) != 0 || e.owner != "config" {
				t.Fatal("rollback retained resources or lost failed proof")
			}
			switch mode {
			case "true", "false":
				if native.loaded.RawGetString("target") != lua.LBool(mode == "true") {
					t.Fatal("discarded foreign export")
				}
			case "registry":
				if registry.RawGetString("_LOADED") != replacement {
					t.Fatal("discarded foreign registry")
				}
			case "metatable":
				if e.L.GetMetatable(native.loaded) != replacement {
					t.Fatal("discarded foreign metatable")
				}
			}
			native.loaded.RawSetString("target", lua.LNil)
			registry.RawSetString("_LOADED", native.loaded)
			e.L.SetMetatable(native.loaded, lua.LNil)
			calls := 0
			native.preload.RawSetString("target", e.L.NewFunction(func(L *lua.LState) int { calls++; L.Push(lua.LTrue); return 1 }))
			if _, err := e.invokeOwnedModule(e.L, "target", resolution, "config", owner, native); err == nil || calls != 0 {
				t.Fatal("restoring public state bypassed failed proof")
			}
		})
	}
}
