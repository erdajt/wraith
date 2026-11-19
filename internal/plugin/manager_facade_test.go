package plugin

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

// TestManagerStorageFreezesPlansAndIsolatesReentrantObservers keeps authorization detached from mutable UI data.
func TestManagerStorageFreezesPlansAndIsolatesReentrantObservers(t *testing.T) {
	source, commit := managerGitFixture(t)
	directory := t.TempDir()
	r := openTestRuntime(t, fmt.Sprintf(`local store=require("wraith.manager.store").new({
 {name="app",source={kind="git",url=%q},selector={kind="head"},pin=false}
 },{root=%q,lockfile=%q})
 local first,summary=store.plan("install")
 summary.packages[1].source.url="/missing/modified-plan"
 summary.packages[1].name="unexpected"
 local first_id,second_id
 first_id=store.execute(first,function(result)
  if result.state~="installed" then return end
  local second=store.plan("restore")
  second_id=store.execute(second,nil,function(restored)
   assert(restored.id==second_id and restored.state=="installed",restored.error)
   assert(store.inspect()[1].commit==%q)
   wraith.notify("complete")
  end)
 end,function(installed)
  assert(installed.id==first_id and installed.state=="installed")
  assert(store.status().id==second_id and second_id~=first_id)
 end)
 assert(not pcall(store.execute,first))`, source, filepath.Join(directory, "data"), filepath.Join(directory, "lock.json"), commit))
	runManagerResources(t, r)
}

// TestManagerStorageCancelsFromInitialProgress remembers cancellation before its controller exists.
func TestManagerStorageCancelsFromInitialProgress(t *testing.T) {
	directory := t.TempDir()
	r := openTestRuntime(t, fmt.Sprintf(`local store=require("wraith.manager.store").new({}, {root=%q,lockfile=%q})
 local id=store.plan("install")
 store.execute(id,function(progress)
  if progress.state=="preparing" then assert(store.cancel(progress.id)) end
 end,function(result)
  assert(result.state=="cancelled",result.error)
  assert(not store.cancel(result.id))
  wraith.notify("complete")
 end)`, filepath.Join(directory, "data"), filepath.Join(directory, "lock.json")))
	runManagerResources(t, r)
	if _, err := os.Stat(filepath.Join(directory, "lock.json")); !os.IsNotExist(err) {
		t.Fatalf("cancelled setup wrote lock: %v", err)
	}
}

// TestManagerStorageRecoversReadOnlyNames bounds recovery and never classifies a guard as disposable.
func TestManagerStorageRecoversReadOnlyNames(t *testing.T) {
	directory := t.TempDir()
	root := filepath.Join(directory, "data")
	staging := filepath.Join(root, "staging")
	if err := os.MkdirAll(staging, 0700); err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{filepath.Join(staging, "txn.example"), filepath.Join(directory, ".wraith-candidate.example"), filepath.Join(directory, ".wraith-baseline.example"), filepath.Join(directory, ".wraith-packages.lock")} {
		if err := os.WriteFile(path, []byte("preserve"), 0600); err != nil {
			t.Fatal(err)
		}
	}
	openTestRuntime(t, fmt.Sprintf(`local store=require("wraith.manager.store").new({}, {root=%q,lockfile=%q})
 assert(store.status()==nil)
 local report=store.recover()
 assert(#report.staging==1 and report.staging[1]=="txn.example")
 assert(#report.publication==2 and #report.errors==0)
 assert(not report.truncated.staging and not report.truncated.publication)
 assert(wraith.fs.read(%q).data=="preserve")`, root, filepath.Join(directory, "lock.json"), filepath.Join(directory, ".wraith-packages.lock")))
}
