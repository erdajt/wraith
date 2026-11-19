package plugin

import (
	"fmt"
	"testing"
)

// TestManagerPlanFreezesDependencySelection exercises the shipped Lua planner without filesystem mutation.
func TestManagerPlanFreezesDependencySelection(t *testing.T) {
	path := managerModule(t, "store_plan.lua")
	openManagerRuntime(t, fmt.Sprintf(`local planner = dofile(%q)
local specs = {
 {name="app",source={kind="git",url="https://example.com/app"},selector={kind="branch",value="main"},pin=false,dependencies={"lib"}},
 {name="lib",source={kind="git",url="https://example.com/lib"},selector={kind="tag",value="v1"},pin=true,dependencies={}},
}
local lock = {version=1,generation="before",packages={
 lib={source=specs[2].source,selector=specs[2].selector,pin=true,commit=string.rep("a",40)},
}}
local plan = planner.create(specs,lock,"update",{"app"})
specs[2].source.url="https://changed.invalid/lib"
lock.packages.lib.commit=string.rep("b",40)
assert(#plan.packages==2 and plan.packages[1].name=="lib" and plan.packages[2].name=="app")
assert(plan.packages[1].commit==string.rep("a",40))
assert(plan.packages[1].source.url=="https://example.com/lib")
assert(plan.packages[2].state=="resolve")
assert(plan.baseline.packages.lib.commit==string.rep("a",40))`, path))
}

// TestManagerPlanRejectsConflictsBeforeInstallation keeps invalid graphs from reaching process execution.
func TestManagerPlanRejectsConflictsBeforeInstallation(t *testing.T) {
	path := managerModule(t, "store_plan.lua")
	openManagerRuntime(t, fmt.Sprintf(`local planner = dofile(%q)
local spec={name="app",source={kind="git",url="https://example.com/app"},selector={kind="branch",value="main"},pin=false,dependencies={}}
local lock={version=1,generation="before",packages={}}
assert(not pcall(planner.create,{spec},lock,"remove"))
assert(not pcall(planner.create,{spec},lock,"restore"))
assert(not pcall(planner.create,{spec},lock,"install",{"unknown"}))
assert(not pcall(planner.create,{spec},lock,"install",{}))
assert(not pcall(planner.create,{spec},lock,"install",{[1]="app",unexpected="app"}))
spec.dependencies={"app"}
assert(not pcall(planner.create,{spec},lock,"install"))
spec.dependencies={}
lock.packages.app={source={kind="git",url="https://other.invalid/app"},selector=spec.selector,pin=false,commit=string.rep("a",40)}
assert(not pcall(planner.create,{spec},lock,"update"))
lock.packages.app.source=spec.source
lock.packages.app.selector={kind="tag",value="v1"}
assert(not pcall(planner.create,{spec},lock,"install"))
assert(planner.create({spec},lock,"restore").packages[1].commit==string.rep("a",40))`, path))
}

// TestManagerPlanSeparatesLocalPinnedAndMovingSelections covers each installer revision policy.
func TestManagerPlanSeparatesLocalPinnedAndMovingSelections(t *testing.T) {
	path := managerModule(t, "store_plan.lua")
	openManagerRuntime(t, fmt.Sprintf(`local planner = dofile(%q)
local specs={
 {name="app",source={kind="git",url="https://example.com/app"},selector={kind="branch",value="main"},pin=false,dependencies={"local-lib"}},
 {name="local-lib",source={kind="local",dir="/explicit/development"},selector={kind="head"},pin=false,dependencies={}},
}
local lock={version=1,generation="before",packages={app={source=specs[1].source,selector=specs[1].selector,pin=false,commit=string.rep("a",40)}}}
local install=planner.create(specs,lock,"install",{"app"})
assert(install.packages[1].state=="local" and install.packages[1].path=="/explicit/development")
assert(install.packages[2].commit==string.rep("a",40))
local update=planner.create(specs,lock,"update",{"app"})
assert(update.packages[2].state=="resolve" and update.packages[2].commit==nil)
specs[1].selector={kind="commit",value=string.rep("b",40)}
specs[1].pin=true
update=planner.create(specs,lock,"update",{"app"})
assert(update.packages[2].commit==string.rep("b",40))
assert(update.packages[2].previous==string.rep("a",40))`, path))
}

// TestManagerPlanListsRetiredSelectionsWithoutChangingBaseline makes declarative lock pruning explicit.
func TestManagerPlanListsRetiredSelectionsWithoutChangingBaseline(t *testing.T) {
	path := managerModule(t, "store_plan.lua")
	openManagerRuntime(t, fmt.Sprintf(`local planner=dofile(%q)
local source={kind="git",url="https://example.com/repo"}
local selector={kind="head"}
local entry={source=source,selector=selector,pin=false,commit=string.rep("a",40)}
local specs={
 {name="selected",source=source,selector=selector,pin=false,dependencies={}},
 {name="unchanged",source=source,selector=selector,pin=false,dependencies={}},
}
local lock={version=1,generation="before",packages={selected=entry,unchanged=entry,disabled=entry,removed=entry}}
local plan=planner.create(specs,lock,"update",{"selected"})
assert(#plan.packages==1 and plan.packages[1].name=="selected")
assert(#plan.removed==2 and plan.removed[1]=="disabled" and plan.removed[2]=="removed")
assert(plan.baseline.packages.unchanged and plan.baseline.packages.disabled)
assert(#planner.create(specs,lock,"restore",{"selected"}).removed==0)
specs[2].source={kind="local",dir="/explicit/development"}
plan=planner.create(specs,lock,"install",{"selected"})
assert(#plan.removed==3 and plan.removed[3]=="unchanged")
assert(not pcall(planner.create,specs,lock,"restore",{"unchanged"}))`, path))
}
