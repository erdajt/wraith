package plugin

import (
	"context"
	"fmt"
	"slices"
	"testing"
	"time"

	lua "github.com/yuin/gopher-lua"
)

// moduleResolutionFixture exposes only a test dispatcher while retaining real host guards.
func moduleResolutionFixture(t *testing.T) *engine {
	t.Helper()
	e := moduleProviderFixture(t)
	e.L.SetContext(t.Context())
	e.L.SetGlobal("dispatch", e.L.NewFunction(func(L *lua.LState) int {
		result, err := e.resolveModuleProvider(L, L.CheckString(1))
		if err != nil {
			L.RaiseError("%v", err)
		}
		if result == nil {
			L.Push(lua.LNil)
		} else {
			L.Push(lua.LString(result.plugin))
		}
		return 1
	}))
	return e
}

// TestModuleResolutionDescriptorsRejectCoercion checks raw fields and detached continuations.
func TestModuleResolutionDescriptorsRejectCoercion(t *testing.T) {
	e := moduleResolutionFixture(t)
	for _, source := range []string{
		`false`, `1`, `"plugin"`, `function() end`, `{}`, `{plugin="p"}`, `{plugin=false,activate=f,check=f}`,
		`{plugin="../p",activate=f,check=f}`, `{plugin=string.rep("p",129),activate=f,check=f}`,
		`{plugin="p",activate=false,check=f}`, `{plugin="p",activate=f,check="f"}`,
		`{plugin="p",activate=f,check=f,extra=true}`, `{plugin="p",activate=f,check=f,[1]=true}`,
		`setmetatable({plugin="p",activate=f,check=f},{})`,
		`setmetatable({},{__index={plugin="p",activate=f,check=f}})`,
	} {
		t.Run(source, func(t *testing.T) {
			if err := e.L.DoString(`local f=function() end; descriptor=` + source); err != nil {
				t.Fatal(err)
			}
			if _, err := decodeModuleResolution(e.L, e.L.GetGlobal("descriptor"), moduleProvider{}); err == nil {
				t.Fatal("accepted an invalid descriptor")
			}
		})
	}
	if err := e.L.DoString(`descriptor={plugin="provider",activate=function() end,check=function() end}`); err != nil {
		t.Fatal(err)
	}
	value := e.L.GetGlobal("descriptor").(*lua.LTable)
	result, err := decodeModuleResolution(e.L, value, moduleProvider{id: "selected"})
	if err != nil {
		t.Fatal(err)
	}
	activate := result.activate
	value.RawSetString("plugin", lua.LString("changed"))
	value.RawSetString("activate", lua.LNil)
	if result.plugin != "provider" || result.activate != activate || result.provider.id != "selected" {
		t.Fatal("descriptor mutation changed the captured continuation")
	}
}

// TestModuleResolutionPreservesOrderAndCoroutineScope uses the current lua state without exposing require hooks.
func TestModuleResolutionPreservesOrderAndCoroutineScope(t *testing.T) {
	e := moduleResolutionFixture(t)
	var visited []int
	for index := range 3 {
		e.owner = fmt.Sprintf("resolver%d", index)
		_, err := e.addModuleProvider(e.L.NewFunction(func(L *lua.LState) int {
			visited = append(visited, index)
			if L == e.L || e.owner != fmt.Sprintf("resolver%d", index) || !slices.Contains(e.packageCalls, "consumer") {
				t.Fatal("resolver lost its coroutine, owner or consumer protection")
			}
			if index == 0 {
				L.Push(lua.LNil)
			} else {
				table := L.NewTable()
				table.RawSetString("plugin", lua.LString("provider"))
				table.RawSetString("activate", L.NewFunction(func(*lua.LState) int { t.Fatal("activated during metadata"); return 0 }))
				table.RawSetString("check", L.NewFunction(func(*lua.LState) int { return 0 }))
				L.Push(table)
			}
			return 1
		}))
		if err != nil {
			t.Fatal(err)
		}
	}
	e.owner = "consumer"
	if err := e.L.DoString(`local co=coroutine.create(function() assert(dispatch("target")=="provider") end); assert(coroutine.resume(co))`); err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(visited, []int{0, 1}) || e.owner != "consumer" || len(e.packageCalls) != 0 || len(e.moduleResolving) != 0 {
		t.Fatal("dispatch changed precedence or retained callback state")
	}
}

// TestModuleResolutionGuardsCaughtEffects rejects captured loaders and host effects before they run.
func TestModuleResolutionGuardsCaughtEffects(t *testing.T) {
	for _, operation := range []string{
		`print("blocked")`, `wraith.config.set("theme","nord")`, `wraith.plugins.load("provider")`,
		`wraith.fs.stat_async(root,function() end)`, `wraith.fs.stat(root,{async=true})`,
		`wraith.commands.create("blocked",function() end)`, `captured()`, `emit_direct()`,
	} {
		t.Run(operation, func(t *testing.T) {
			e := moduleResolutionFixture(t)
			e.L.SetGlobal("root", lua.LString(t.TempDir()))
			record := e.moduleRecord(e.L, moduleSource{name: "captured", preload: e.L.NewFunction(func(*lua.LState) int {
				t.Fatal("metadata executed a captured loader")
				return 0
			})})
			e.L.SetGlobal("captured", record.RawGetString("load"))
			e.L.SetGlobal("emit_direct", e.L.NewFunction(func(L *lua.LState) int { e.emit(L, Effect{Name: "notify"}); return 0 }))
			if err := e.L.DoString(`function resolve() assert(not pcall(function() ` + operation + ` end)); return nil end`); err != nil {
				t.Fatal(err)
			}
			if _, err := e.addModuleProvider(e.L.GetGlobal("resolve").(*lua.LFunction)); err != nil {
				t.Fatal(err)
			}
			if _, err := e.resolveModuleProvider(e.L, "target"); err == nil || len(e.effects) != 0 || len(e.manifest.Commands) != 0 || len(e.manifest.Diagnostics) != 0 {
				t.Fatalf("caught prohibited metadata operation escaped: %v", err)
			}
		})
	}
}

// TestModuleResolutionAllowsBoundedMetadataReads keeps source inspection separate from execution.
func TestModuleResolutionAllowsBoundedMetadataReads(t *testing.T) {
	e := moduleResolutionFixture(t)
	e.L.SetGlobal("root", lua.LString(t.TempDir()))
	if err := e.L.DoString(`function resolve(name)
 assert(wraith.fs.stat(root).exists)
 assert(wraith.modules.source(root,name)==nil)
 assert(#wraith.modules.catalog(root)==0)
 assert(not wraith.modules.loading(name))
 assert(type(wraith.plugins.list())=="table")
 assert(wraith.json.decode(wraith.json.encode({ready=true})).ready)
 return nil
end`); err != nil {
		t.Fatal(err)
	}
	if _, err := e.addModuleProvider(e.L.GetGlobal("resolve").(*lua.LFunction)); err != nil {
		t.Fatal(err)
	}
	if result, err := e.resolveModuleProvider(e.L, "missing"); err != nil || result != nil {
		t.Fatalf("metadata reads did not yield a clean miss: %v", err)
	}
}

// TestModuleResolutionUnwindsErrorsAndDeadlines restores state and rejects stale registrations.
func TestModuleResolutionUnwindsErrorsAndDeadlines(t *testing.T) {
	for _, body := range []string{`error("failed")`, `pcall(dispatch,"target")`, `pcall(dispatch,"m"..#name..name)`, `while true do end`} {
		t.Run(body, func(t *testing.T) {
			e := moduleResolutionFixture(t)
			if err := e.L.DoString(`function resolve(name) calls=(calls or 0)+1; ` + body + ` end`); err != nil {
				t.Fatal(err)
			}
			if _, err := e.addModuleProvider(e.L.GetGlobal("resolve").(*lua.LFunction)); err != nil {
				t.Fatal(err)
			}
			ctx, cancel := context.WithTimeout(t.Context(), 50*time.Millisecond)
			defer cancel()
			e.L.SetContext(ctx)
			if _, err := e.resolveModuleProvider(e.L, "target"); err == nil {
				t.Fatal("accepted failed or recursive metadata")
			}
			if e.owner != "config" || len(e.moduleResolving) != 0 || len(e.packageCalls) != 0 || e.moduleViolation != nil || e.L.GetTop() != 0 {
				t.Fatal("failed metadata retained execution state")
			}
			if body == `pcall(dispatch,"m"..#name..name)` && e.L.GetGlobal("calls") != lua.LNumber(32) {
				t.Fatal("metadata nesting did not enforce the 32-call boundary")
			}
		})
	}
	e := moduleResolutionFixture(t)
	var id string
	id, _ = e.addModuleProvider(e.L.NewFunction(func(L *lua.LState) int {
		if err := e.removeModuleProvider(id); err != nil {
			t.Fatal(err)
		}
		L.Push(lua.LNil)
		return 1
	}))
	if _, err := e.resolveModuleProvider(e.L, "target"); err == nil || len(e.moduleProviders) != 1 || e.moduleProviders[0].id != id {
		t.Fatal("stale resolver escaped or rollback lost its registration")
	}
}
