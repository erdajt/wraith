package plugin_test

import (
	"context"
	"strings"
	"testing"

	lua "github.com/yuin/gopher-lua"
)

func TestProtectedCallsPreserveClosures(t *testing.T) {
	cases := map[string]string{
		"caller": `local value=0
local get=function() return value end
assert(not pcall(error, "caught"))
value=1; assert(get()==1)`,
		"escaped callee": `local escaped
assert(not pcall(function()
 local value=2; escaped=function() return value end
 value=3; error("caught")
end))
local replacement=99
assert(escaped()==3)`,
		"nested handlers": `local outer=0
local get=function() return outer end
local escaped, handled
local ok, result=xpcall(function()
 assert(not pcall(function() error("inner") end))
 local value=4; escaped=function() return value end
 error("outer")
end, function(message)
 assert(message:find("outer"))
 assert(not pcall(error,"handler inner"))
 handled=true
 return "handled"
end)
assert(not ok and result=="handled" and handled)
outer=1; assert(get()==1 and escaped()==4)`,
		"failed handler": `local outer=0
local get=function() return outer end
local escaped, handler_local
assert(not xpcall(function()
 local value=5; escaped=function() return value end
 error("callee")
end, function()
 local value=6; handler_local=function() return value end
 error("handler")
end))
outer=1
assert(get()==1 and escaped()==5 and handler_local()==6)`,
		"error object": `local value=0
local get=function() return value end
local object={reason="caught"}
local ok, result=pcall(error,object)
assert(not ok and result==object)
value=1; assert(get()==1)`,
		"go panic": `local value=0
local get=function() return value end
local escaped
assert(not pcall(function()
 local inner=7; escaped=function() return inner end
 host_panic()
end))
value=1; assert(get()==1 and escaped()==7)`,
		"tail recursion": `local value=0
local get=function() return value end
local captures={}
local function descend(n)
 local inner=n; captures[#captures+1]=function() return inner end
 if n>0 then return descend(n-1) end
 error("tail")
end
assert(not pcall(descend,32))
value=1; assert(get()==1)
for i,capture in ipairs(captures) do assert(capture()==33-i) end`,
		"nested host call": `local value=0
local get=function() return value end
local escaped
assert(not pcall(host_nested,function()
 local inner=13; escaped=function() return inner end
 error("nested")
end))
value=1; assert(get()==1 and escaped()==13)`,
		"coroutine": `local value=0
local get=function() return value end
local escaped
local thread=coroutine.create(function()
 local inner=8; escaped=function() return inner end
 assert(not pcall(error,"caught"))
 inner=9; host_panic()
end)
assert(not coroutine.resume(thread))
value=1; assert(get()==1 and escaped()==9)`,
		"handler observes failed frame": `local escaped
local ok=xpcall(function()
 local value=10; escaped=function() return value end
 error("caught")
end, function()
 local found=false
 for level=2,8 do
  if not debug.getinfo(level) then break end
  for index=1,20 do
   local name,value=debug.getlocal(level,index)
   if not name then break end
   if name=="value" and value==10 then
    found=true; debug.setlocal(level,index,11)
   end
  end
 end
 assert(found and escaped()==11)
 return "handled"
end)
assert(not ok and escaped()==11)`,
	}
	for _, stack := range []struct {
		name    string
		growing bool
	}{{"fixed", false}, {"growing", true}} {
		for name, source := range cases {
			t.Run(name+"/"+stack.name, func(t *testing.T) {
				state := lua.NewState(lua.Options{MinimizeStackMemory: stack.growing})
				t.Cleanup(state.Close)
				state.SetGlobal("host_panic", state.NewFunction(func(*lua.LState) int { panic("host failure") }))
				state.SetGlobal("host_nested", state.NewFunction(func(state *lua.LState) int {
					err := state.CallByParam(lua.P{Fn: state.CheckFunction(1), NRet: 0, Protect: true})
					if err != nil {
						state.RaiseError("nested: %s", err)
					}
					return 0
				}))
				if err := state.DoString(source); err != nil {
					t.Fatal(err)
				}
				if err := state.DoString(`assert(2+2==4)`); err != nil {
					t.Fatalf("state reuse: %v", err)
				}
			})
		}
	}
}

func TestProtectedCallCancellationClosesEscapedLocals(t *testing.T) {
	state := lua.NewState()
	t.Cleanup(state.Close)
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	state.SetContext(ctx)
	state.SetGlobal("cancel", state.NewFunction(func(*lua.LState) int { cancel(); return 0 }))
	err := state.DoString(`local value=12; escaped=function() return value end; cancel(); while true do end`)
	if err == nil || !strings.Contains(err.Error(), "context canceled") {
		t.Fatalf("cancellation: %v", err)
	}
	state.RemoveContext()
	if err := state.DoString(`local replacement=99; assert(escaped()==12)`); err != nil {
		t.Fatal(err)
	}
}
