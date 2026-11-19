package plugin

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

// TestManagerRetainsInactiveLocks carries graph retention through real lock publication.
func TestManagerRetainsInactiveLocks(t *testing.T) {
	directory := t.TempDir()
	lockfile := filepath.Join(directory, "lock.json")
	r := openManagerRuntime(t, fmt.Sprintf(`local lockfile=%q
local codec=require("wraith.manager.store_lock")
local baseline=codec.empty()
local entry={source={kind="git",url="/missing/inactive-repository"},selector={kind="head"},pin=false,commit=string.rep("a",40)}
baseline.packages.disabled=entry
baseline.packages.conditional=entry
baseline.packages.retired=entry
baseline.packages.development=entry
wraith.fs.write(lockfile,wraith.json.encode(baseline))
local retained={disabled=true,conditional=true,absent=true,development=true}
local store=require("wraith.manager.store").new({
 {name="development",source={kind="local",dir=%q},selector={kind="head"},pin=false}
},{root=%q,lockfile=lockfile},retained)
retained.disabled=false
retained.conditional=false
local id,summary=store.plan("install")
assert(#summary.packages==1 and summary.packages[1].state=="local")
assert(table.concat(summary.pruned,",")=="development,retired")
store.execute(id,nil,function(result)
 assert(result.state=="installed",result.error)
 local current=codec.read(lockfile)
 assert(current.packages.disabled.commit==entry.commit)
 assert(current.packages.conditional.source.url==entry.source.url)
 assert(not current.packages.absent and not current.packages.development and not current.packages.retired)
 assert(baseline.packages.retired and current.generation~=baseline.generation)
 wraith.notify("complete")
end)`, lockfile, directory, filepath.Join(directory, "data")))
	runManagerResources(t, r)
	if _, err := os.Stat(filepath.Join(directory, "data", "objects")); !os.IsNotExist(err) {
		t.Fatalf("inactive locks created an object directory: %v", err)
	}
}
