package plugin

import (
	"strings"
	"testing"

	lua "github.com/yuin/gopher-lua"
)

// TestModuleRequireOuterRollbackChecksTeardown retains proof after activation succeeds and source fails.
func TestModuleRequireOuterRollbackChecksTeardown(t *testing.T) {
	for _, mutation := range []string{"none", "cache", "registry"} {
		t.Run(mutation, func(t *testing.T) {
			e, native := moduleRequireFixture(t)
			registry := e.L.Get(lua.RegistryIndex).(*lua.LTable)
			foreign := e.L.NewTable()
			cleaned := 0
			e.L.SetGlobal("cleanup_state", e.L.NewFunction(func(L *lua.LState) int {
				if !e.closing || e.L != L || !strings.HasPrefix(e.owner, "plugin:provider:") {
					t.Fatal("outer rollback lost cleanup ownership")
				}
				cleaned++
				switch mutation {
				case "cache":
					native.loaded.RawSetString("target", foreign)
				case "registry":
					registry.RawSetString("_LOADED", foreign)
				}
				return 0
			}))
			if err := e.L.DoString(`
descriptors.target={plugin="provider",check=function() end,
activate=function() wraith.plugins.load("provider",{},{dependency=false}) end}
package.preload.provider=function() return {
setup=function() wraith.commands.create("setup-resource",function() end) end,
teardown=function() cleanup_state() end} end
package.preload.target=function() error("source failed after setup") end
local ok,err=pcall(require,"target")
assert(not ok and tostring(err):find("source failed after setup",1,true))
`); err != nil {
				t.Fatal(err)
			}
			if cleaned != 1 || len(e.packages) != 0 || len(e.cleanup) != 0 || len(e.manifest.Commands) != 0 {
				t.Fatal("failed source retained the activated lifecycle")
			}
			if mutation == "none" {
				if e.moduleClaims["target"].poison != "" || native.loaded.RawGetString("target") != lua.LNil {
					t.Fatal("clean source failure blocked retry")
				}
				if err := e.L.DoString(`package.preload.target=function() return 42 end; assert(require("target")==42)`); err != nil {
					t.Fatal(err)
				}
				return
			}
			if e.moduleClaims["target"].poison == "" {
				t.Fatal("outer teardown mutation escaped proof")
			}
			if mutation == "cache" && native.loaded.RawGetString("target") != foreign || mutation == "registry" && registry.RawGetString("_LOADED") != foreign {
				t.Fatal("rollback removed foreign state")
			}
			registry.RawSetString("_LOADED", native.loaded)
			native.loaded.RawSetString("target", lua.LNil)
			if err := e.L.DoString(`local ok,err=pcall(require,"target"); assert(not ok and tostring(err):find("/reload",1,true))`); err != nil {
				t.Fatal(err)
			}
			if cleaned != 1 {
				t.Fatal("poisoned retry activated the provider")
			}
		})
	}
}

// TestModuleRequireRetirementWaitsForPublication preserves exports through rejected unloads and cleanup.
func TestModuleRequireRetirementWaitsForPublication(t *testing.T) {
	e, native := moduleRequireFixture(t)
	if err := e.L.DoString(`
descriptors.target={plugin="provider",check=function() end,
activate=function() wraith.plugins.load("provider",{},{dependency=false}) end}
package.preload.target=function() runs=(runs or 0)+1; return {} end
package.preload.provider=function() return {setup=function() end,
teardown=function() assert(require("target")==original); cleaned=(cleaned or 0)+1 end} end
original=require("target")
`); err != nil {
		t.Fatal(err)
	}
	before, original := e.captureRegistry(), native.loaded.RawGetString("target")
	owner := e.packages["provider"].owner
	if err := e.unloadPlugin("provider"); err != nil {
		t.Fatal(err)
	}
	if err := e.L.DoString(`local ok,err=pcall(require,"target"); assert(not ok and tostring(err):find("retirement",1,true))`); err != nil {
		t.Fatal(err)
	}
	if err := e.disposeRegistry(e.restoreRegistry(before)); err != nil {
		t.Fatal(err)
	}
	if e.packages["provider"].owner != owner || native.loaded.RawGetString("target") != original || e.L.GetGlobal("cleaned") != lua.LNil {
		t.Fatal("rejected unload changed the live generation")
	}
	if err := e.L.DoString(`assert(require("target")==original and runs==1)`); err != nil {
		t.Fatal(err)
	}
	if err := e.unloadPlugin("provider"); err != nil {
		t.Fatal(err)
	}
	disposal := registryDisposal{hooks: e.disposing, modules: e.retiredModuleExecutions()}
	e.disposing = nil
	if err := e.disposeRegistry(disposal); err != nil {
		t.Fatal(err)
	}
	if e.L.GetGlobal("cleaned") != lua.LNumber(1) || native.loaded.RawGetString("target") != lua.LNil {
		t.Fatal("accepted cleanup did not read then retire the export")
	}
	if err := e.L.DoString(`assert(require("target")~=original and runs==2)`); err != nil {
		t.Fatal(err)
	}
	if e.packages["provider"].owner == owner {
		t.Fatal("reactivation reused an old generation")
	}
}

// TestModuleRequireClosingNeverResolves rejects unavailable exports without calling providers or sources.
func TestModuleRequireClosingNeverResolves(t *testing.T) {
	for _, value := range []lua.LValue{lua.LNil, lua.LFalse, lua.LString("foreign")} {
		t.Run(value.String(), func(t *testing.T) {
			e, native := moduleRequireFixture(t)
			release, err := e.claimModule("target")
			if err != nil {
				t.Fatal(err)
			}
			release()
			native.loaded.RawSetString("target", value)
			e.moduleProviders[0].resolve = e.L.NewFunction(func(*lua.LState) int { t.Fatal("cleanup called resolver"); return 0 })
			native.preload.RawSetString("target", e.L.NewFunction(func(*lua.LState) int { t.Fatal("cleanup called source"); return 0 }))
			e.closing = true
			defer func() { e.closing = false }()
			if err := e.L.DoString(`assert(not pcall(require,"target"))`); err != nil {
				t.Fatal(err)
			}
			if native.loaded.RawGetString("target") != value {
				t.Fatal("cleanup modified unverified cache")
			}
		})
	}
}
