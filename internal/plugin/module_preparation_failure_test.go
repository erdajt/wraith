package plugin

import (
	"context"
	"strings"
	"testing"

	lua "github.com/yuin/gopher-lua"
)

// TestModulePreparationFailureRestoresNestedLifecycles retries after prerequisite source and setup completed.
func TestModulePreparationFailureRestoresNestedLifecycles(t *testing.T) {
	for _, mode := range []string{"error", "cancel", "foreign"} {
		t.Run(mode, func(t *testing.T) {
			e, native := moduleRequireFixture(t)
			main := e.L
			ctx, cancel := context.WithCancel(t.Context())
			defer cancel()
			main.SetGlobal("fail_prepare", main.NewFunction(func(L *lua.LState) int {
				if mode == "cancel" {
					cancel()
					return 0
				}
				if mode == "foreign" {
					native.loaded.RawSetString("target", lua.LFalse)
				}
				L.RaiseError("preparation failed")
				return 0
			}))
			if err := main.DoString(`descriptors.target = {
 plugin="provider", check=function() end,
 prepare=function()
  preparations=(preparations or 0)+1
  wraith.plugins.load("initializer", {}, {dependency=false})
  fail_prepare()
 end,
 activate=function() wraith.plugins.load("provider", {}, {dependency=false}) end,
}
package.preload.initializer=function() return {setup=function() require("target") end} end
package.preload.provider=function() return {setup=function() end} end
package.preload.target=function()
 runs=(runs or 0)+1
 wraith.commands.create("discarded", function() end)
 return {}
end`); err != nil {
				t.Fatal(err)
			}
			main.SetContext(ctx)
			if err := main.DoString(`require("target")`); err == nil {
				t.Fatal("preparation failure succeeded")
			}
			if len(e.packages) != 0 || len(e.manifest.Commands) != 0 || len(e.moduleExecutions) != 0 || len(e.modulePreparing) != 0 || len(e.moduleActivating) != 0 || e.owner != "config" || e.L != main {
				t.Fatal("preparation failure retained lifecycle or execution state")
			}
			main.SetContext(t.Context())
			main.SetGlobal("fail_prepare", main.NewFunction(func(*lua.LState) int { return 0 }))
			if mode == "foreign" {
				if native.loaded.RawGetString("target") != lua.LFalse || e.moduleClaims["target"].poison == "" {
					t.Fatal("foreign write was discarded or unpoisoned")
				}
				native.loaded.RawSetString("target", lua.LNil)
				if err := main.DoString(`require("target")`); err == nil || main.GetGlobal("preparations") != lua.LNumber(1) {
					t.Fatal("poisoned preparation retried")
				}
			} else {
				if native.loaded.RawGetString("target") != lua.LNil || e.moduleClaims["target"].poison != "" {
					t.Fatal("clean preparation failure poisoned its verified export")
				}
				if err := main.DoString(`assert(require("target")); assert(preparations==2 and runs==2)`); err != nil {
					t.Fatal(err)
				}
			}
		})
	}
}

// TestModulePreparationAlwaysRevalidates rejects changed provenance before activation or a ready return.
func TestModulePreparationAlwaysRevalidates(t *testing.T) {
	for _, ready := range []bool{false, true} {
		name := "absent"
		if ready {
			name = "ready"
		}
		t.Run(name, func(t *testing.T) {
			e, native, resolution := moduleActivationFixture(t)
			prepared, checks := false, 0
			resolution.check = e.L.NewFunction(func(L *lua.LState) int {
				checks++
				if prepared {
					L.RaiseError("selected source changed")
				}
				return 0
			})
			resolution.prepare = e.L.NewFunction(func(L *lua.LState) int {
				if ready {
					if err := L.DoString(`package.preload.provider=function() return {setup=function() end} end; wraith.plugins.load("provider", {}, {dependency=false})`); err != nil {
						L.RaiseError("%v", err)
					}
				}
				prepared = true
				return 0
			})
			if _, err := e.activateModuleProvider(e.L, "target", resolution, "config", native); err == nil || !strings.Contains(err.Error(), "selected source changed") {
				t.Fatalf("post-preparation verification: %v", err)
			}
			if checks != 2 || e.L.GetGlobal("activations") != lua.LNil || len(e.packages) != 0 || len(e.modulePreparing) != 0 {
				t.Fatal("changed source reached activation or escaped rollback")
			}
		})
	}
}

// TestModulePreparationKeepsActivationCycleGuard rejects callbacks that recurse after preparation ends.
func TestModulePreparationKeepsActivationCycleGuard(t *testing.T) {
	e, _ := moduleRequireFixture(t)
	if err := e.L.DoString(`descriptors.target = {
 plugin="provider", check=function() end,
 prepare=function() preparations=(preparations or 0)+1 end,
 activate=function() return require("target") end,
}
local ok, err = pcall(require, "target")
assert(not ok and tostring(err):find("recursive module activation", 1, true))
assert(preparations==1)`); err != nil {
		t.Fatal(err)
	}
	if len(e.modulePreparing) != 0 || len(e.moduleActivating) != 0 || len(e.packages) != 0 {
		t.Fatal("activation cycle retained preparation state")
	}
}
