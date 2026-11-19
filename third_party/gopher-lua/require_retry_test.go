package lua

import (
	"context"
	"strings"
	"testing"
)

// requireState keeps native require regressions isolated from application loaders.
func requireState(t *testing.T) *LState {
	t.Helper()
	L := NewState()
	t.Cleanup(L.Close)
	return L
}

func TestRequireRetriesFailedSourcesAndNestedCycles(t *testing.T) {
	L := requireState(t)
	if err := L.DoString(`
local attempts=0
package.preload.retry=function()
 attempts=attempts+1
 if attempts==1 then error("first source failure") end
 return {attempts=attempts}
end
local ok,err=pcall(require,"retry")
assert(not ok and tostring(err):find("first source failure",1,true))
assert(package.loaded.retry==nil)
local result=require("retry")
assert(result.attempts==2 and require("retry")==result)

package.preload.outer=function() return require("inner") end
package.preload.inner=function() return require("outer") end
ok,err=pcall(require,"outer")
assert(not ok and tostring(err):find("loop",1,true))
assert(package.loaded.outer==nil and package.loaded.inner==nil)
package.preload.inner=function() return "fixed" end
assert(require("outer")=="fixed")
`); err != nil {
		t.Fatal(err)
	}
}

func TestRequirePreservesSourceWrittenCacheOnFailure(t *testing.T) {
	L := requireState(t)
	L.SetGlobal("foreign", L.NewUserData())
	if err := L.DoString(`
for _,value in ipairs({false, {}, foreign, "retained", true}) do
 package.loaded.changed=nil
 package.preload.changed=function()
  package.loaded.changed=value
  error("source changed cache")
 end
 local ok,err=pcall(require,"changed")
 assert(not ok and tostring(err):find("source changed cache",1,true))
 assert(rawequal(package.loaded.changed,value))
end
package.loaded.changed=nil
package.preload.changed=function()
 package.loaded.changed=nil
 error("explicit nil")
end
assert(not pcall(require,"changed") and package.loaded.changed==nil)

package.preload.original=function()
 package.loaded={replacement=true}
 error("public cache replaced")
end
local cache=package.loaded
assert(not pcall(require,"original"))
assert(cache.original==nil and package.loaded.replacement)
package.loaded=cache
`); err != nil {
		t.Fatal(err)
	}
}

func TestRequireRetainsNativeSuccessfulCacheSemantics(t *testing.T) {
	L := requireState(t)
	if err := L.DoString(`
local calls=0
package.preload.nil_result=function() calls=calls+1 end
assert(require("nil_result")==true and require("nil_result")==true and calls==1)
package.preload.false_result=function() calls=calls+1; return false end
assert(require("false_result")==false and require("false_result")==false and calls==3)
local assigned={}
package.preload.assignment=function()
 package.loaded.assignment=assigned
 return "ignored"
end
assert(require("assignment")==assigned and package.loaded.assignment==assigned)
package.preload.explicit_nil=function()
 package.loaded.explicit_nil=nil
 return "ignored"
end
assert(require("explicit_nil")==nil and package.loaded.explicit_nil==nil)
package.preload.first=function() return "first", "ignored", nil end
assert(require("first")=="first")
`); err != nil {
		t.Fatal(err)
	}
}

func TestRequirePreservesErrorObjectsAndCallerUpvalues(t *testing.T) {
	L := requireState(t)
	if err := L.DoString(`
local marker={reason="exact object"}
local value="before"
local caller=function() return value end
package.preload.object=function()
 local escaped="callee"
 failed_reader=function() return escaped end
 error(marker)
end
local ok,err=pcall(require,"object")
assert(not ok and err==marker and package.loaded.object==nil)
value="after"
assert(caller()=="after" and failed_reader()=="callee")
local handled=false
ok,err=xpcall(function() require("object") end,function(failure)
 assert(failure==marker)
 handled=true
 return marker
end)
assert(not ok and err==marker and handled and caller()=="after")
`); err != nil {
		t.Fatal(err)
	}
}

func TestRequireClearsItsMarkerAfterGoPanic(t *testing.T) {
	L := requireState(t)
	L.SetGlobal("crash", L.NewFunction(func(L *LState) int {
		panic("native source panic")
	}))
	if err := L.DoString(`
package.preload.native=function() crash() end
local ok,err=pcall(require,"native")
assert(not ok and tostring(err):find("native source panic",1,true))
assert(package.loaded.native==nil)
package.preload.native=function() return "recovered" end
assert(require("native")=="recovered")
`); err != nil {
		t.Fatal(err)
	}
}

func TestRequireClearsItsMarkerAfterCancellation(t *testing.T) {
	L := requireState(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	L.SetGlobal("cancel", L.NewFunction(func(L *LState) int {
		cancel()
		return 0
	}))
	if err := L.DoString(`package.preload.cancelled=function() cancel(); while true do end end`); err != nil {
		t.Fatal(err)
	}
	L.SetContext(ctx)
	err := L.DoString(`require("cancelled")`)
	if err == nil || !strings.Contains(err.Error(), "context canceled") {
		t.Fatalf("require did not propagate cancellation: %v", err)
	}
	cache := L.GetField(L.Get(RegistryIndex), "_LOADED").(*LTable)
	if cache.RawGetString("cancelled") != LNil {
		t.Fatal("cancelled source retained its marker")
	}
	L.SetContext(context.Background())
	if err := L.DoString(`package.preload.cancelled=function() return "retry" end; assert(require("cancelled")=="retry")`); err != nil {
		t.Fatal(err)
	}
}

func TestRequireLoadingQueryUsesOnlyTheNativeRegistryMarker(t *testing.T) {
	L := requireState(t)
	L.SetGlobal("foreign", L.NewUserData())
	L.SetGlobal("loading", L.NewFunction(func(L *LState) int {
		L.Push(LBool(L.IsModuleLoading(L.CheckString(1))))
		return 1
	}))
	if err := L.DoString(`
assert(not loading("module"))
package.loaded.module=foreign
assert(not loading("module"))
package.loaded.module=false
local called=0
package.preload.module=function(name)
 called=called+1
 assert(loading(name)==(called==2))
 if called==2 then
  local cache=package.loaded
  package.loaded={}
  assert(loading(name))
  package.loaded=cache
 end
 return false
end
package.preload.module("module")
assert(not loading("module"))
assert(require("module")==false and called==2)
assert(not loading("module"))
`); err != nil {
		t.Fatal(err)
	}
	registry := L.G.Registry
	cache := registry.RawGetString("_LOADED")
	registry.RawSetString("_LOADED", LNil)
	if L.IsModuleLoading("module") {
		t.Fatal("missing cache looked like an active require")
	}
	registry.RawSetString("_LOADED", cache)
}

func TestRequireFailureCleanupDoesNotInvokeCacheMetamethods(t *testing.T) {
	L := requireState(t)
	if err := L.DoString(`
local marker={}
local cache=package.loaded
package.preload.metatable=function()
 cache.metatable=nil
 setmetatable(cache,{__index=function() error("cache metamethod invoked") end})
 error(marker)
end
local ok,err=pcall(require,"metatable")
setmetatable(cache,nil)
assert(not ok and err==marker and cache.metatable==nil)
`); err != nil {
		t.Fatal(err)
	}
}

func TestRequirePreservesSourceFramesForErrorHandlers(t *testing.T) {
	L := requireState(t)
	if err := L.DoString(`
package.preload.trace=assert(loadstring([[
local function deepest_source() error("source failure") end
deepest_source()
]],"@selected-source.lua"))
local ok,result=xpcall(function() require("trace") end,function(err)
 return debug.traceback(tostring(err))
end)
assert(not ok and package.loaded.trace==nil)
assert(result:find("in function 'deepest_source'",1,true),result)
assert(result:find("in function <@selected-source.lua:0>",1,true),result)
`); err != nil {
		t.Fatal(err)
	}
}
