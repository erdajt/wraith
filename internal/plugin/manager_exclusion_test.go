package plugin

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

// TestManagerExcludedPackagesRetainOnlyEligiblePins exercises setup, status and publication together.
func TestManagerExcludedPackagesRetainOnlyEligiblePins(t *testing.T) {
	directory := t.TempDir()
	root := filepath.Join(directory, "data")
	lockfile := filepath.Join(directory, "lock.json")
	r := openManagerRuntime(t, fmt.Sprintf(`local codec=require("wraith.manager.store_lock")
local baseline=codec.empty()
local entry={source={kind="git",url="/missing/excluded-repository"},selector={kind="head"},pin=false,commit=string.rep("a",40)}
for _,name in ipairs({"disabled","optional","scoped"}) do baseline.packages[name]=entry end
wraith.fs.write(%q,wraith.json.encode(baseline))
local manager=require("wraith.manager")
local calls=0
manager.setup({
 {name="active",dir=%q,lazy=true,enabled=function() calls=calls+1; return true end},
 {"owner/disabled",enabled=false,specs={{"owner/scoped"}}},
 {"owner/optional",optional=true},
 {"owner/absent",enabled=false},
},{storage={root=%q,lockfile=%q}})
assert(calls==1)
local function rows()
 local result={}
 for _,row in ipairs(manager.status().packages) do result[row.name]=row end
 return result
end
local current=rows()
assert(current.active.managed and current.active.installed)
assert(current.disabled.retained_commit==entry.commit and not current.absent.retained_commit)
assert(not current.optional.retained_commit and not current.scoped.retained_commit)
assert(current.optional.exclusion=="optional" and current.scoped.exclusion=="scope")
for _,name in ipairs({"disabled","optional","scoped","absent"}) do
 local row=current[name]
 assert(row.state=="excluded" and not row.loaded and not row.managed)
 for _,field in ipairs({"installed","path","source","commit","loaded_commit","restart_required"}) do
  assert(row[field]==nil,name..":"..field)
 end
 for _,action in ipairs({"load","unload","install","update","restore"}) do
  local ok,err=pcall(manager[action],name)
  assert(not ok and tostring(err):find("excluded ("..row.exclusion..")",1,true),tostring(err))
 end
end
current.disabled.origins[1].index=-1
assert(rows().disabled.origins[1].index>0)
for _,names in ipairs({false,"disabled",{}, {[1]="active",[3]="disabled"}, {extra="disabled"}}) do
 assert(not pcall(manager.plan,"install",names))
end
assert(manager.status().operation==nil)
local duplicates={}
for index=1,64 do duplicates[index]="active" end
assert(#manager.plan("install",duplicates).summary.packages==1)
local plan=manager.plan("install")
assert(#plan.summary.packages==1 and plan.summary.packages[1].name=="active")
assert(table.concat(plan.summary.pruned,",")=="optional,scoped")
manager.execute(plan.id)
local timer
timer=wraith.timer.start(5,function()
 local operation=manager.status().operation
 if operation.state=="installed" or operation.state=="failed" or operation.state=="cancelled" then
  assert(operation.state=="installed",operation.error)
  wraith.resource.cancel(timer)
  wraith.resource.release(timer)
  wraith.notify("complete")
 end
end,{repeat_ms=5})
wraith.commands.create("check",function()
 local status=manager.status()
 assert(status.operation.state=="installed",status.operation.error)
 local lock=codec.read(%q)
 assert(lock.packages.disabled.commit==entry.commit)
 assert(not lock.packages.optional and not lock.packages.scoped and not lock.packages.absent)
 assert(not rows().disabled.installed and calls==1)
end)`, lockfile, directory, root, lockfile, lockfile))
	runManagerResources(t, r)
	managerCommand(t, r, "check", "")
	if _, err := os.Stat(filepath.Join(root, "objects")); !os.IsNotExist(err) {
		t.Fatalf("excluded packages created object storage: %v", err)
	}
}
