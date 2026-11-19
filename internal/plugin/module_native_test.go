package plugin

import (
	"slices"
	"strings"
	"testing"

	lua "github.com/yuin/gopher-lua"
)

// moduleNativeFixture binds a real loaded provider to captured native loading state.
func moduleNativeFixture(t *testing.T) (*engine, nativeModules, *moduleResolution, string) {
	t.Helper()
	e, _, owner := moduleExecutionFixture(t)
	native, err := captureNativeModules(e.L)
	if err != nil {
		t.Fatal(err)
	}
	e.owner = "resolver"
	if _, err := e.addModuleProvider(e.L.NewFunction(func(*lua.LState) int { return 0 })); err != nil {
		t.Fatal(err)
	}
	resolution := &moduleResolution{provider: e.moduleProviders[0], plugin: "provider", check: e.L.NewFunction(func(*lua.LState) int { return 0 })}
	e.owner = "config"
	return e, native, resolution, owner
}

// TestOwnedModulePreservesNativeValues distinguishes cached exports from false and explicit nil results.
func TestOwnedModulePreservesNativeValues(t *testing.T) {
	for _, tc := range []struct {
		name, expression, body string
		repeated               bool
	}{
		{"true", "true", "return expected", false},
		{"false", "false", "return expected", true},
		{"number", "42", "return expected", false},
		{"string", `"export"`, "return expected", false},
		{"table", "{}", "return expected", false},
		{"function", "function() return 42 end", "return expected", false},
		{"userdata", "newproxy()", "return expected", false},
		{"implicit true", "true", "return nil", false},
		{"written nil", "nil", "package.loaded.target=nil; return {}", true},
		{"written false", "false", "package.loaded.target=false; return {}", true},
		{"written table", "{}", "package.loaded.target=expected; return false", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			e, native, resolution, owner := moduleNativeFixture(t)
			if err := e.L.DoString(`expected=` + tc.expression + `; package.preload.target=function()
runs=(runs or 0)+1; ` + tc.body + ` end`); err != nil {
				t.Fatal(err)
			}
			var first uint64
			for index := range 2 {
				value, err := e.invokeOwnedModule(e.L, "target", resolution, "config", owner, native)
				if err != nil || value != e.L.GetGlobal("expected") || native.loaded.RawGetString("target") != value {
					t.Fatalf("native result or cache changed: %v, %v", value, err)
				}
				serial := e.moduleExecutions["target"].serial
				if index == 0 {
					first = serial
				} else if tc.repeated && serial <= first || !tc.repeated && serial != first {
					t.Fatal("receipt did not distinguish execution from cached access")
				}
			}
			want := lua.LNumber(1)
			if tc.repeated {
				want = 2
			}
			if e.L.GetGlobal("runs") != want || len(e.moduleExecutions) != 1 || e.moduleClaims["target"].pending != 0 {
				t.Fatal("native execution count or claim lifecycle changed")
			}
		})
	}
}

// TestOwnedModuleRegistersUnderProvider assigns source-created commands to the loaded provider.
func TestOwnedModuleRegistersUnderProvider(t *testing.T) {
	e, native, resolution, owner := moduleNativeFixture(t)
	if err := e.L.DoString(`package.preload.target=function()
wraith.commands.create("from-module",function() end)
return {ready=true}
end`); err != nil {
		t.Fatal(err)
	}
	if _, err := e.invokeOwnedModule(e.L, "target", resolution, "config", owner, native); err != nil {
		t.Fatal(err)
	}
	if len(e.manifest.Commands) != 1 || e.callbackOwners[e.manifest.Commands[0].Callback] != owner || e.owner != "config" {
		t.Fatal("module command was registered under its consumer or resolver")
	}
	if err := e.unloadPlugin("provider"); err != nil || len(e.manifest.Commands) != 0 {
		t.Fatalf("provider unload retained its source command: %v", err)
	}
}

// TestOwnedModuleCachedImportsRetainProvider records dependencies even when native require uses its cache.
func TestOwnedModuleCachedImportsRetainProvider(t *testing.T) {
	e, native, resolution, owner := moduleNativeFixture(t)
	if err := e.L.DoString(`package.preload.target=function() runs=(runs or 0)+1; return {} end
package.preload.consumer=function() return {setup=function() end} end
wraith.plugins.load("consumer")`); err != nil {
		t.Fatal(err)
	}
	first, err := e.invokeOwnedModule(e.L, "target", resolution, "config", owner, native)
	if err != nil {
		t.Fatal(err)
	}
	serial := e.moduleExecutions["target"].serial
	e.owner = e.packages["consumer"].owner
	value, err := e.invokeOwnedModule(e.L, "target", resolution, e.owner, owner, native)
	if err != nil || value != first || e.L.GetGlobal("runs") != lua.LNumber(1) || e.moduleExecutions["target"].serial != serial {
		t.Fatalf("cached import executed its source again: %v", err)
	}
	if !slices.Contains(e.packages["consumer"].dependencies, owner) {
		t.Fatal("cached import did not retain its provider")
	}
	e.owner = "config"
	if err := e.unloadPlugin("provider"); err == nil {
		t.Fatal("provider unloaded while a cached consumer still depended on it")
	}
	if err := e.unloadPlugin("consumer"); err != nil {
		t.Fatal(err)
	}
	if err := e.unloadPlugin("provider"); err != nil {
		t.Fatal(err)
	}
}

// TestOwnedModuleRetriesCleanSourceFailures preserves native retry after source errors and recursive loads.
func TestOwnedModuleRetriesCleanSourceFailures(t *testing.T) {
	for _, source := range []string{`error("source failed")`, `return require("target")`} {
		t.Run(source, func(t *testing.T) {
			e, native, resolution, owner := moduleNativeFixture(t)
			if err := e.L.DoString(`package.preload.target=function()
attempts=(attempts or 0)+1
if attempts==1 then
 wraith.commands.create("transient",function() end)
 ` + source + `
end
return 42
end`); err != nil {
				t.Fatal(err)
			}
			if _, err := e.invokeOwnedModule(e.L, "target", resolution, "config", owner, native); err == nil {
				t.Fatal("source failure was accepted")
			}
			if native.loaded.RawGetString("target") != lua.LNil || len(e.manifest.Commands) != 0 || e.moduleClaims["target"].poison != "" {
				t.Fatal("clean failure retained registrations, cache or poison")
			}
			value, err := e.invokeOwnedModule(e.L, "target", resolution, "config", owner, native)
			if err != nil || value != lua.LNumber(42) || e.L.GetGlobal("attempts") != lua.LNumber(2) {
				t.Fatalf("clean source could not retry: %v", err)
			}
		})
	}
}

// TestOwnedModulePreservesUnverifiedFailedExports poisons source-written cache without deleting it.
func TestOwnedModulePreservesUnverifiedFailedExports(t *testing.T) {
	e, native, resolution, owner := moduleNativeFixture(t)
	if err := e.L.DoString(`foreign={}
package.preload.target=function()
 attempts=(attempts or 0)+1
 package.loaded.target=foreign
 error("source failed after cache write")
end`); err != nil {
		t.Fatal(err)
	}
	if _, err := e.invokeOwnedModule(e.L, "target", resolution, "config", owner, native); err == nil {
		t.Fatal("accepted failed source")
	}
	if native.loaded.RawGetString("target") != e.L.GetGlobal("foreign") || e.moduleClaims["target"].poison == "" || len(e.moduleExecutions) != 0 {
		t.Fatal("failed source cache was erased or recorded as verified")
	}
	if _, err := e.invokeOwnedModule(e.L, "target", resolution, "config", owner, native); err == nil || !strings.Contains(err.Error(), "use /reload") {
		t.Fatal("poisoned source did not require reload")
	}
	if e.L.GetGlobal("attempts") != lua.LNumber(1) || e.moduleClaims["target"].pending != 0 {
		t.Fatal("poisoned source ran again or retained its reservation")
	}
}
