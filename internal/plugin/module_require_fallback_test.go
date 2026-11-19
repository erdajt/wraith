package plugin

import (
	"testing"

	lua "github.com/yuin/gopher-lua"
)

// moduleFallbackFixture captures native require before installing the private coordinator.
func moduleFallbackFixture(t *testing.T, resolver bool) (*engine, nativeModules) {
	t.Helper()
	e := moduleResolutionFixture(t)
	native := e.nativeModules
	if resolver {
		if err := e.L.DoString(`function resolve() resolves=(resolves or 0)+1; return nil end`); err != nil {
			t.Fatal(err)
		}
		if _, err := e.addModuleProvider(e.L.GetGlobal("resolve").(*lua.LFunction)); err != nil {
			t.Fatal(err)
		}
	}
	e.L.SetGlobal("require", e.L.NewFunction(func(L *lua.LState) int { return e.requireModule(L, native) }))
	return e, native
}

// TestModuleRequireFallbackPreservesErrors keeps native error objects and deepest frames in both lua states.
func TestModuleRequireFallbackPreservesErrors(t *testing.T) {
	for _, mode := range []string{"native", "miss", "outside-grammar"} {
		for _, coroutine := range []bool{false, true} {
			t.Run(mode+map[bool]string{false: "/main", true: "/coroutine"}[coroutine], func(t *testing.T) {
				e, _ := moduleFallbackFixture(t, mode != "native")
				main := e.L
				name := "broken"
				if mode == "outside-grammar" {
					name = "odd/name"
				}
				main.SetGlobal("module_name", lua.LString(name))
				main.SetGlobal("use_coroutine", lua.LBool(coroutine))
				if err := main.DoString(`local sentinel={failure=true}
package.preload[module_name]=function()
 runs=(runs or 0)+1
 local function deepest_source() error(sentinel); return false end
 deepest_source()
end
local function exercise()
 local ok,err=pcall(require,module_name); assert(not ok and err==sentinel)
 local success,trace=xpcall(function() require(module_name) end,function(err)
  assert(err==sentinel); return debug.traceback()
 end)
 assert(not success and trace:find("deepest_source",1,true),trace)
 assert(package.loaded[module_name]==nil and runs==2)
end
if use_coroutine then
 local thread=coroutine.create(exercise); local ok,err=coroutine.resume(thread); assert(ok,err)
else exercise() end`); err != nil {
					t.Fatal(err)
				}
				want := lua.LValue(lua.LNil)
				if mode == "miss" {
					want = lua.LNumber(2)
				}
				if main.GetGlobal("resolves") != want || e.L != main || e.owner != "config" || len(e.packageCalls) != 0 || len(e.moduleClaims) != 0 || len(e.moduleRequests) != 0 {
					t.Fatal("native fallback retained routing state or changed resolver dispatch")
				}
			})
		}
	}
}

// TestModuleRequireFallbackKeepsNativeCacheSemantics preserves false reloads, source cache writes and argument coercion.
func TestModuleRequireFallbackKeepsNativeCacheSemantics(t *testing.T) {
	for _, resolver := range []bool{false, true} {
		t.Run(map[bool]string{false: "native", true: "miss"}[resolver], func(t *testing.T) {
			e, _ := moduleFallbackFixture(t, resolver)
			if err := e.L.DoString(`local cases={
 {name="false_result",source=function() return false end,want=false,runs=2},
 {name="nil_result",source=function() end,want=true,runs=1},
 {name="nil_cache",source=function(name) package.loaded[name]=nil end,want=nil,runs=2},
 {name="false_cache",source=function(name) package.loaded[name]=false end,want=false,runs=2},
 {name="table_cache",source=function(name) local value={}; package.loaded[name]=value end,runs=1},
}
for _,case in ipairs(cases) do
 local runs=0
 package.preload[case.name]=function(name) runs=runs+1; return case.source(name) end
 local first=require(case.name,"ignored"); local second=require(case.name)
 if case.name=="table_cache" then assert(type(first)=="table" and first==second)
 else assert(first==case.want and second==case.want,case.name) end
 assert(runs==case.runs,case.name)
end
package.preload["42"]=function(name) assert(name=="42"); return 42 end
assert(require(42)==42)
local searches=0
table.insert(package.loaders,1,function(name)
 searches=searches+1; assert(name=="custom")
 return function() return {custom=true} end
end)
local first=require("custom"); assert(first.custom and require("custom")==first and searches==1)`); err != nil {
				t.Fatal(err)
			}
			if len(e.moduleClaims) != 0 || len(e.moduleExecutions) != 0 {
				t.Fatal("native fallback claimed or receipted an unmanaged source")
			}
		})
	}
}

// TestModuleRequireHookCacheBoundary rejects intercepted, claimed and uncached helpers before executing lua.
func TestModuleRequireHookCacheBoundary(t *testing.T) {
	for _, mode := range []string{"pure", "metadata"} {
		for _, state := range []string{"truthy", "claimed", "missing", "false", "metatable", "registry"} {
			t.Run(mode+"/"+state, func(t *testing.T) {
				e, native := moduleFallbackFixture(t, true)
				value := e.L.NewTable()
				native.loaded.RawSetString("helper", value)
				native.preload.RawSetString("helper", e.L.NewFunction(func(*lua.LState) int {
					t.Fatal("hook executed an uncached source")
					return 0
				}))
				switch state {
				case "claimed":
					release, err := e.claimModule("helper")
					if err != nil {
						t.Fatal(err)
					}
					release()
				case "missing":
					native.loaded.RawSetString("helper", lua.LNil)
				case "false":
					native.loaded.RawSetString("helper", lua.LFalse)
				case "metatable":
					e.L.SetMetatable(native.loaded, e.L.NewTable())
				case "registry":
					e.L.Get(lua.RegistryIndex).(*lua.LTable).RawSetString("_LOADED", e.L.NewTable())
				}
				e.pureHook = mode == "pure"
				if mode == "metadata" {
					e.moduleResolving = []string{"outer"}
				}
				err := e.L.DoString(`ok,result=pcall(require,"helper")`)
				violation := e.hookViolation
				if mode == "metadata" {
					violation = e.moduleViolation
				}
				e.pureHook, e.moduleResolving = false, nil
				e.L.SetMetatable(native.loaded, lua.LNil)
				e.L.Get(lua.RegistryIndex).(*lua.LTable).RawSetString("_LOADED", native.loaded)
				if err != nil {
					t.Fatal(err)
				}
				if state == "truthy" {
					if e.L.GetGlobal("ok") != lua.LTrue || e.L.GetGlobal("result") != value || violation != nil {
						t.Fatal("hook rejected an unchanged unmanaged cached helper")
					}
				} else if e.L.GetGlobal("ok") != lua.LFalse || violation == nil {
					t.Fatal("caught require bypassed the sticky hook violation")
				}
				if e.L.GetGlobal("resolves") != lua.LNil || len(e.packageCalls) != 0 {
					t.Fatal("hook executed resolver metadata or retained caller protection")
				}
			})
		}
	}
}

// TestModuleRequireHookRejectsNativeMarker prevents an in-progress source from becoming a cached helper.
func TestModuleRequireHookRejectsNativeMarker(t *testing.T) {
	for _, mode := range []string{"pure", "metadata"} {
		t.Run(mode, func(t *testing.T) {
			e, native := moduleFallbackFixture(t, true)
			native.preload.RawSetString("helper", e.L.NewFunction(func(L *lua.LState) int {
				e.pureHook = mode == "pure"
				if mode == "metadata" {
					e.moduleResolving = []string{"outer"}
				}
				err := L.CallByParam(lua.P{Fn: L.GetGlobal("require"), Protect: true, NRet: 1}, lua.LString("helper"))
				e.pureHook, e.moduleResolving = false, nil
				if err == nil || mode == "pure" && e.hookViolation == nil || mode == "metadata" && e.moduleViolation == nil {
					t.Fatal("hook returned the native loading marker")
				}
				L.Push(lua.LTrue)
				return 1
			}))
			if err := e.L.CallByParam(lua.P{Fn: native.require, Protect: true, NRet: 1}, lua.LString("helper")); err != nil {
				t.Fatal(err)
			}
			if e.L.Get(-1) != lua.LTrue || e.L.GetGlobal("resolves") != lua.LNil {
				t.Fatal("native source lost its result or invoked metadata")
			}
		})
	}
}

// TestModuleRequirePureHookWithoutProvidersPreservesNativeLoading keeps existing config-only behavior.
func TestModuleRequirePureHookWithoutProvidersPreservesNativeLoading(t *testing.T) {
	e, _ := moduleFallbackFixture(t, false)
	e.pureHook = true
	err := e.L.DoString(`package.preload.helper=function() return 42 end; assert(require("helper")==42)`)
	e.pureHook = false
	if err != nil || e.hookViolation != nil {
		t.Fatalf("config-only native loading changed: %v", err)
	}
}

// TestModuleRequireCaptureRejectsNonNativeFunctions prevents direct calls through a lua replacement.
func TestModuleRequireCaptureRejectsNonNativeFunctions(t *testing.T) {
	for _, source := range []string{`require=function() end`, `require=false`} {
		t.Run(source, func(t *testing.T) {
			e := moduleResolutionFixture(t)
			if err := e.L.DoString(source); err != nil {
				t.Fatal(err)
			}
			if _, err := captureNativeModules(e.L); err == nil {
				t.Fatal("accepted a non-native require")
			}
		})
	}
}
