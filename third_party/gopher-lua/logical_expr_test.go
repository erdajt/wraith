package lua

import (
	"fmt"
	"testing"
)

// TestLogicalAssignmentComparisons checks nested boolean results against explicit control flow.
func TestLogicalAssignmentComparisons(t *testing.T) {
	L := NewState()
	defer L.Close()
	if err := L.DoString(`local function check(serial, proof, executed, initial)
 local valid=initial
 valid=valid and (executed and serial>proof or not executed and serial==proof)
 local expected=false
 if initial then
  if executed then expected=serial>proof else expected=serial==proof end
 end
 assert(valid==expected,table.concat({tostring(initial),tostring(executed),serial,proof},"/"))
end
for _,initial in ipairs({false,true}) do
 for _,executed in ipairs({false,true}) do
  for serial=0,2 do for proof=0,2 do check(serial,proof,executed,initial) end end
 end
end`); err != nil {
		t.Fatal(err)
	}
}

// TestLogicalSelectedOperands preserves values and short-circuit calls across expression sources.
func TestLogicalSelectedOperands(t *testing.T) {
	for _, value := range []string{"nil", "false", "true", "0", `""`, "{}", "function() end", "newproxy()"} {
		for _, operand := range []string{"left()", "record.value", "indexed.value", "input", "global_value", "not input"} {
			for _, operator := range []string{"and", "or"} {
				t.Run(value+"/"+operand+"/"+operator, func(t *testing.T) {
					L := NewState()
					defer L.Close()
					source := fmt.Sprintf(`local input=%s
global_value=input
local calls,right_calls,index_calls=0,0,0
local sentinel={}
local record={value=input}
local indexed=setmetatable({},{__index=function(_,key)
 assert(key=="value"); index_calls=index_calls+1; return input
end})
local function left() calls=calls+1; return input end
local function right() right_calls=right_calls+1; return sentinel end
local function exercise()
 local result="stale"
 result=%s %s right()
 return result
end
local actual=exercise()
local selected=input
if %q=="not input" then selected=not input end
local expected,want_right=selected,0
if %q=="and" then
 if selected then expected=sentinel; want_right=1 end
else
 if not selected then expected=sentinel; want_right=1 end
end
assert(rawequal(actual,expected),"selected operand changed")
assert(right_calls==want_right,"right operand evaluation changed")
if %q=="left()" then assert(calls==1) else assert(calls==0) end
if %q=="indexed.value" then assert(index_calls==1) else assert(index_calls==0) end`, value, operand, operator, operand, operator, operand, operand)
					if err := L.DoString(source); err != nil {
						t.Fatal(err)
					}
				})
			}
		}
	}
}

// TestLogicalAssignmentDefersDestinationWrite keeps intermediate operands from changing aliased locals.
func TestLogicalAssignmentDefersDestinationWrite(t *testing.T) {
	for _, expression := range []string{
		"(left() or false) and right()",
		"(left() or false) and (right() or false)",
		"(left() and false) or right()",
		"(left() and true) and right()",
		"(left() or false) and (not false and right())",
	} {
		t.Run(expression, func(t *testing.T) {
			L := NewState()
			defer L.Close()
			if err := L.DoString(fmt.Sprintf(`local result="original"
local trace={}
local function left()
 assert(result=="original","left observed early assignment")
 trace[#trace+1]="left"; return "left"
end
local function right()
 assert(result=="original","right observed early assignment")
 trace[#trace+1]="right"; return "right"
end
result=%s
assert(result=="right" and table.concat(trace,",")=="left,right")`, expression)); err != nil {
				t.Fatal(err)
			}
		})
	}
}

// TestLogicalResultContexts preserves false and nil outside direct local reassignment.
func TestLogicalResultContexts(t *testing.T) {
	for _, context := range []string{
		"local result=left() and right(); return result",
		"return left() and right()",
		"global_result=left() and right(); return global_result",
		"local target={}; target.value=left() and right(); return target.value",
		"local result='stale'; local function assign() result=left() and right() end; assign(); return result",
		"local function identity(value) return value end; return identity(left() and right())",
	} {
		t.Run(context, func(t *testing.T) {
			L := NewState()
			defer L.Close()
			if err := L.DoString(fmt.Sprintf(`local calls=0
local input
local function left() calls=calls+1; return input end
local function right() error("right operand must be skipped") end
local function exercise() %s end
assert(exercise()==nil and calls==1)
input=false
assert(exercise()==false and calls==2)`, context)); err != nil {
				t.Fatal(err)
			}
		})
	}
}
