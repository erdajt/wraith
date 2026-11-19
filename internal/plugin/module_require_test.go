package plugin

import (
	"context"
	"strings"
	"testing"

	lua "github.com/yuin/gopher-lua"
)

// moduleRequireFixture installs private routing over a real plugin lifecycle runtime.
func moduleRequireFixture(t *testing.T) (*engine, nativeModules) {
	t.Helper()
	e := moduleResolutionFixture(t)
	native, err := captureNativeModules(e.L)
	if err != nil {
		t.Fatal(err)
	}
	e.L.SetGlobal("descriptors", e.L.NewTable())
	e.owner = "resolver"
	if _, err := e.addModuleProvider(e.L.NewFunction(func(L *lua.LState) int {
		L.Push(L.GetGlobal("descriptors").(*lua.LTable).RawGetString(L.CheckString(1)))
		return 1
	})); err != nil {
		t.Fatal(err)
	}
	e.owner = "config"
	e.L.SetGlobal("require", e.L.NewFunction(func(L *lua.LState) int { return e.requireModule(L, native) }))
	return e, native
}

// TestModuleRequireReentrantSetup keeps the original consumer across nested coroutine imports.
func TestModuleRequireReentrantSetup(t *testing.T) {
	e, _ := moduleRequireFixture(t)
	main := e.L
	if err := main.DoString(`local descriptor={plugin="provider",check=function() end,activate=function()
 activations=(activations or 0)+1
 wraith.plugins.load("provider",{},{dependency=false})
end}
descriptors.target=descriptor; descriptors.extra=descriptor
package.preload.target=function()
 runs=(runs or 0)+1; wraith.commands.create("target-command",function() end); return {ready=true}
end
package.preload.extra=function() wraith.commands.create("extra-command",function() end); return 42 end
package.preload.provider=function() return {setup=function()
 setups=(setups or 0)+1
 local child=coroutine.create(function() assert(require("target").ready); assert(require("extra")==42) end)
 local ok,err=coroutine.resume(child); assert(ok,err)
end} end
package.preload.consumer=function() return {setup=function() result=require("target") end} end
local thread=coroutine.create(function() wraith.plugins.load("consumer") end)
local ok,err=coroutine.resume(thread); assert(ok,err)
assert(result.ready and runs==1 and setups==1 and activations==1)`); err != nil {
		t.Fatal(err)
	}
	owner := e.packages["provider"].owner
	if e.L != main || e.owner != "config" || len(e.moduleRequests) != 0 || len(e.moduleActivating) != 0 || len(e.packageCalls) != 0 {
		t.Fatal("nested import leaked its execution scope")
	}
	for _, command := range e.manifest.Commands {
		if e.callbackOwners[command.Callback] != owner {
			t.Fatal("source command escaped provider ownership")
		}
	}
	if len(e.manifest.Commands) != 2 || len(e.packages["consumer"].dependencies) != 1 || e.packages["consumer"].dependencies[0] != owner {
		t.Fatal("outer consumer lost its provider dependency")
	}
	if err := main.DoString(`assert(require("target")==result and runs==1)`); err != nil {
		t.Fatal(err)
	}
	if err := e.unloadPlugin("provider"); err == nil {
		t.Fatal("cached consumer did not retain provider")
	}
}

// TestModuleRequireSeparatesActivationAndSourceCycles rejects incomplete providers but preserves native cycles.
func TestModuleRequireSeparatesActivationAndSourceCycles(t *testing.T) {
	for _, cycle := range []string{"activation", "source"} {
		t.Run(cycle, func(t *testing.T) {
			e, _ := moduleRequireFixture(t)
			if err := e.L.DoString(`for _,name in ipairs({"a","b"}) do
 local plugin="provider_"..name
 descriptors[name]={plugin=plugin,check=function() end,activate=function() wraith.plugins.load(plugin,{},{dependency=false}) end}
end
package.preload.provider_a=function() return {setup=function() if activation_cycle then require("b") end end} end
package.preload.provider_b=function() return {setup=function() require("a") end} end
package.preload.a=function() if not activation_cycle then return require("a") end; return true end
package.preload.b=function() return true end`); err != nil {
				t.Fatal(err)
			}
			e.L.SetGlobal("activation_cycle", lua.LBool(cycle == "activation"))
			err := e.L.DoString(`require("a")`)
			want := "still loading"
			if cycle == "source" {
				want = "loop or previous error"
			}
			if err == nil || !strings.Contains(err.Error(), want) {
				t.Fatalf("cycle diagnostic: %v", err)
			}
			if len(e.packages) != 0 || len(e.moduleRequests) != 0 || len(e.packageCalls) != 0 || e.moduleClaims["a"].pending != 0 || e.moduleClaims["a"].poison != "" {
				t.Fatal("cycle retained partial lifecycle or poisoned clean source")
			}
		})
	}
}

// TestModuleRequireBoundsRequestsAndRetainsClaims rejects recursion and missing resolvers before execution.
func TestModuleRequireBoundsRequestsAndRetainsClaims(t *testing.T) {
	e, _ := moduleRequireFixture(t)
	if err := e.L.DoString(`descriptors.target={plugin="provider",check=function() end,activate=function()
 activations=(activations or 0)+1; wraith.plugins.load("provider",{},{dependency=false}) end}
package.preload.provider=function() return {setup=function() end} end
package.preload.target=function() runs=(runs or 0)+1; return true end`); err != nil {
		t.Fatal(err)
	}
	for range 32 {
		e.moduleRequests = append(e.moduleRequests, &moduleRequest{name: "other"})
	}
	if err := e.L.DoString(`require("target")`); err == nil || e.L.GetGlobal("activations") != lua.LNil {
		t.Fatal("request depth reached activation")
	}
	e.moduleRequests = nil
	if err := e.L.DoString(`require("target")`); err != nil {
		t.Fatal(err)
	}
	e.moduleProviders = nil
	if err := e.L.DoString(`require("target")`); err == nil || !strings.Contains(err.Error(), "no resolver") || e.L.GetGlobal("runs") != lua.LNumber(1) {
		t.Fatal("claimed cached source bypassed its missing resolver")
	}
}

// TestModuleRequireNestedProviderRetainsConsumer routes another provider without a global resolver bypass.
func TestModuleRequireNestedProviderRetainsConsumer(t *testing.T) {
	e, _ := moduleRequireFixture(t)
	if err := e.L.DoString(`for _,name in ipairs({"a","b"}) do
 local plugin="provider_"..name
 descriptors[name]={plugin=plugin,check=function() end,activate=function() wraith.plugins.load(plugin,{},{dependency=false}) end}
 package.preload[name]=function() return name end
end
package.preload.provider_a=function() return {setup=function() assert(require("b")=="b") end} end
package.preload.provider_b=function() return {setup=function() end} end
assert(require("a")=="a")`); err != nil {
		t.Fatal(err)
	}
	dependencies := e.packages["provider_a"].dependencies
	if len(dependencies) != 1 || dependencies[0] != e.packages["provider_b"].owner {
		t.Fatal("nested foreign import lost its original consumer")
	}
	if err := e.unloadPlugin("provider_b"); err == nil {
		t.Fatal("nested consumer did not retain its provider")
	}
}

// TestModuleRequireCancellationUnwindsRequests restores outer frames after activation or source cancellation.
func TestModuleRequireCancellationUnwindsRequests(t *testing.T) {
	for _, phase := range []string{"activation", "source"} {
		t.Run(phase, func(t *testing.T) {
			e, native := moduleRequireFixture(t)
			main := e.L
			if err := main.DoString(`descriptors.target={plugin="provider",check=function() end,activate=function()
 wraith.plugins.load("provider",{},{dependency=false}) end}
package.preload.provider=function() return {setup=function() if phase=="activation" then cancel_request() end end} end
package.preload.target=function() wraith.commands.create("discarded",function() end); cancel_request(); return true end`); err != nil {
				t.Fatal(err)
			}
			ctx, cancel := context.WithCancel(t.Context())
			defer cancel()
			main.SetGlobal("phase", lua.LString(phase))
			main.SetGlobal("cancel_request", main.NewFunction(func(*lua.LState) int { cancel(); return 0 }))
			main.SetContext(ctx)
			if err := main.DoString(`require("target")`); err == nil || !strings.Contains(err.Error(), "context canceled") {
				t.Fatalf("cancelled request: %v", err)
			}
			if e.L != main || e.owner != "config" || len(e.moduleRequests) != 0 || len(e.moduleActivating) != 0 || len(e.packageCalls) != 0 || len(e.packages) != 0 || len(e.manifest.Commands) != 0 || e.moduleClaims["target"].pending != 0 {
				t.Fatal("cancelled request retained lifecycle or execution frames")
			}
			main.SetContext(t.Context())
			main.SetGlobal("cancel_request", main.NewFunction(func(*lua.LState) int { return 0 }))
			if err := main.DoString(`assert(require("target"))`); err != nil || native.loaded.RawGetString("target") != lua.LTrue {
				t.Fatalf("clean cancellation retry: %v", err)
			}
		})
	}
}

// TestModuleRequireLifecycleNameMatchesSource routes the lifecycle loader through its own request.
func TestModuleRequireLifecycleNameMatchesSource(t *testing.T) {
	e, _ := moduleRequireFixture(t)
	if err := e.L.DoString(`descriptors.target={plugin="target",check=function() end,activate=function()
 wraith.plugins.load("target",{},{dependency=false}) end}
package.preload.target=function()
 runs=(runs or 0)+1
 return {setup=function() setups=(setups or 0)+1; assert(type(require("target"))=="table") end}
end
assert(type(require("target"))=="table" and runs==1 and setups==1)`); err != nil {
		t.Fatal(err)
	}
	if e.moduleExecutions["target"].owner != e.packages["target"].owner || len(e.moduleRequests) != 0 {
		t.Fatal("lifecycle source lost its request owner")
	}
}
