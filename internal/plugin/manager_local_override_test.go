package plugin

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

// TestManagerLocalSourcesLoadWithoutFetchingAndRemoveTheirPins follows both local selection paths through publication.
func TestManagerLocalSourcesLoadWithoutFetchingAndRemoveTheirPins(t *testing.T) {
	for _, mode := range []string{"explicit", "development"} {
		t.Run(mode, func(t *testing.T) {
			directory := t.TempDir()
			root, lockfile := filepath.Join(directory, "packages"), filepath.Join(directory, "lock.json")
			module := `return {setup=function(opts)
 wraith.commands.create("review",function() wraith.notify(opts.message) end)
end}`
			path := filepath.Join(directory, "review.lua")
			if err := os.WriteFile(path, []byte(module), 0600); err != nil {
				t.Fatal(err)
			}
			r := openManagerRuntime(t, fmt.Sprintf(`local manager=require("shroud")
local codec=require("wraith.manager.store_lock")
local baseline=codec.empty()
baseline.packages.review={source={kind="git",url="/missing/local-override-repository"},
 selector={kind="branch",value="work"},pin=true,commit=string.rep("a",40)}
wraith.fs.write(%q,wraith.json.encode(baseline))
local initialized=0
local mode=%q
manager.setup({
 {name="review",url="/missing/local-override-repository",branch="work",pin=true,main="review",dev=mode=="development",
  cmd="review",opts={message="local checkout"},init=function(spec)
   initialized=initialized+1
   assert(spec.source.kind=="local" and spec.source.dir==%q)
   assert(spec.selector.kind=="head" and not spec.pin)
   assert(spec.declared_source.url=="/missing/local-override-repository")
   assert(spec.declared_selector.value=="work" and spec.declared_pin)
   assert(spec.dev==(mode=="development"))
  end},
 {name="review",dir=mode=="explicit" and %q or nil},
},{storage={root=%q,lockfile=%q},dev={path=function(spec)
 assert(mode=="development" and spec.dev and spec.branch=="work" and spec.pin)
 return %q
end}})
local row=manager.status().packages[1]
assert(initialized==1 and row.installed and row.source.kind=="local" and row.commit==nil)
local plan=manager.plan("install",{"review"})
assert(#plan.summary.packages==1 and plan.summary.packages[1].source.kind=="local")
assert(#plan.summary.pruned==1 and plan.summary.pruned[1]=="review")
manager.execute(plan.id)
local timer
timer=wraith.timer.start(5,function()
 local operation=manager.status().operation
 if operation.state=="installed" or operation.state=="failed" or operation.state=="cancelled" then
  assert(operation.state=="installed",operation.error)
  assert(codec.read(%q).packages.review==nil)
  wraith.resource.cancel(timer)
  wraith.resource.release(timer)
  wraith.notify("complete")
 end
end,{repeat_ms=5})`, lockfile, mode, directory, directory, root, lockfile, directory, lockfile))
			runManagerResources(t, r)
			result := managerCommand(t, r, "review", "")
			if len(result.Effects) != 1 || result.Effects[0].Argument != "local checkout" {
				t.Fatalf("local override did not run: %+v", result)
			}
			for _, absent := range []string{filepath.Join(root, "objects"), filepath.Join(directory, ".git")} {
				if _, err := os.Stat(absent); !os.IsNotExist(err) {
					t.Fatalf("local override created git storage %s: %v", absent, err)
				}
			}
			data, err := os.ReadFile(path)
			if err != nil || string(data) != module {
				t.Fatalf("local package changed: %q, %v", data, err)
			}
		})
	}
}
