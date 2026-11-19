package plugin

import (
	"context"
	"slices"
	"testing"
	"time"

	lua "github.com/yuin/gopher-lua"
)

// TestModuleCheckRestoresCoroutineAfterRollback keeps cleanup on the calling state.
func TestModuleCheckRestoresCoroutineAfterRollback(t *testing.T) {
	e := moduleResolutionFixture(t)
	main := e.L
	e.owner = "resolver"
	if _, err := e.addModuleProvider(main.NewFunction(func(*lua.LState) int { return 0 })); err != nil {
		t.Fatal(err)
	}
	cleaned := false
	resolution := &moduleResolution{provider: e.moduleProviders[0], plugin: "provider"}
	resolution.check = main.NewFunction(func(L *lua.LState) int {
		if L == main || e.L != L || e.owner != "resolver" || L.CheckString(1) != "target" || !slices.Contains(e.packageCalls, "consumer") {
			t.Fatal("check lost caller state or ownership")
		}
		e.addCleanup(L.NewFunction(func(cleanup *lua.LState) int {
			if cleanup != L || e.L != L || e.owner != "resolver" {
				t.Fatal("rollback cleanup left the coroutine")
			}
			cleaned = true
			return 0
		}))
		L.RaiseError("check failed")
		return 0
	})
	main.SetGlobal("check", main.NewFunction(func(L *lua.LState) int {
		if _, err := e.checkModuleResolution(L, "target", resolution, moduleVerification{phase: "before"}); err != nil {
			L.RaiseError("%v", err)
		}
		return 0
	}))
	e.owner = "consumer"
	if err := main.DoString(`local co=coroutine.create(function() assert(not pcall(check)) end); assert(coroutine.resume(co))`); err != nil {
		t.Fatal(err)
	}
	if !cleaned || e.L != main || e.owner != "consumer" || len(e.cleanup) != 0 || len(e.packageCalls) != 0 || len(e.moduleResolving) != 0 || main.GetTop() != 0 {
		t.Fatal("failed check retained coroutine state or cleanup")
	}
}

// TestModuleCheckRejectsChangedRegistrations checks the captured owner and callback identity.
func TestModuleCheckRejectsChangedRegistrations(t *testing.T) {
	for _, when := range []string{"before", "after"} {
		t.Run(when, func(t *testing.T) {
			e := moduleResolutionFixture(t)
			if _, err := e.addModuleProvider(e.L.NewFunction(func(*lua.LState) int { return 0 })); err != nil {
				t.Fatal(err)
			}
			original := e.moduleProviders[0]
			calls := 0
			resolution := &moduleResolution{provider: original, plugin: "provider", check: e.L.NewFunction(func(*lua.LState) int {
				calls++
				e.moduleProviders[0].owner = "changed"
				return 0
			})}
			if when == "before" {
				e.moduleProviders[0].resolve = e.L.NewFunction(func(*lua.LState) int { return 0 })
			}
			before := e.moduleProviders[0]
			if _, err := e.checkModuleResolution(e.L, "target", resolution, moduleVerification{phase: "before"}); err == nil {
				t.Fatal("accepted changed registration")
			}
			if e.moduleProviders[0] != before || when == "before" && calls != 0 || when == "after" && calls != 1 {
				t.Fatal("check ran a stale callback or lost rollback identity")
			}
		})
	}
}

// TestModuleCheckPreservesNestedViolations does not hide caught child metadata errors.
func TestModuleCheckPreservesNestedViolations(t *testing.T) {
	e := moduleResolutionFixture(t)
	if err := e.L.DoString(`function resolve() pcall(print,"blocked"); return nil end
function verify(name) assert(name=="target"); pcall(dispatch,"child"); return false end`); err != nil {
		t.Fatal(err)
	}
	if _, err := e.addModuleProvider(e.L.GetGlobal("resolve").(*lua.LFunction)); err != nil {
		t.Fatal(err)
	}
	resolution := &moduleResolution{provider: e.moduleProviders[0], plugin: "provider", check: e.L.GetGlobal("verify").(*lua.LFunction)}
	if _, err := e.checkModuleResolution(e.L, "target", resolution, moduleVerification{phase: "before"}); err == nil || len(e.manifest.Diagnostics) != 0 || e.moduleViolation != nil {
		t.Fatal("caught nested violation escaped or leaked")
	}
	resolution.check = e.L.NewFunction(func(L *lua.LState) int { L.Push(lua.LFalse); return 1 })
	if _, err := e.checkModuleResolution(e.L, "target", resolution, moduleVerification{phase: "before"}); err != nil {
		t.Fatalf("check treated its ignored return as a rejection: %v", err)
	}
}

// TestModuleCheckUnwindsDeadline restores state even when lua cannot finish its callback.
func TestModuleCheckUnwindsDeadline(t *testing.T) {
	e := moduleResolutionFixture(t)
	if err := e.L.DoString(`function verify() while true do end end`); err != nil {
		t.Fatal(err)
	}
	if _, err := e.addModuleProvider(e.L.NewFunction(func(*lua.LState) int { return 0 })); err != nil {
		t.Fatal(err)
	}
	resolution := &moduleResolution{provider: e.moduleProviders[0], plugin: "provider", check: e.L.GetGlobal("verify").(*lua.LFunction)}
	ctx, cancel := context.WithTimeout(t.Context(), 20*time.Millisecond)
	defer cancel()
	e.L.SetContext(ctx)
	if _, err := e.checkModuleResolution(e.L, "target", resolution, moduleVerification{phase: "before"}); err == nil {
		t.Fatal("check exceeded deadline without failure")
	}
	if e.owner != "config" || len(e.moduleResolving) != 0 || len(e.packageCalls) != 0 || e.moduleViolation != nil || e.L.GetTop() != 0 {
		t.Fatal("deadline retained metadata state")
	}
}
