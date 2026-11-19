package plugin

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// managerExecute builds a frozen plan and runs the real storage pipeline without UI substitutes.
func managerExecute(t *testing.T, directory, specs, action, expected string) {
	t.Helper()
	r := openManagerRuntime(t, fmt.Sprintf(`local options=require("wraith.manager.store_options").normalize({root=%q,lockfile=%q})
 local codec=require("wraith.manager.store_lock")
 local baseline,raw=codec.read(options.lockfile)
 local plan=require("wraith.manager.store_plan").create(%s,baseline,%q)
 plan.baseline_raw=raw
 require("wraith.manager.store_execute").start(plan,options,function() end,function(result)
  assert(result.state==%q,result.error)
  if plan.action=="restore" and result.state=="installed" then assert(result.generation==baseline.generation) end
  wraith.notify("complete")
 end)`, filepath.Join(directory, "data"), filepath.Join(directory, "lock.json"), specs, action, expected))
	runManagerResources(t, r)
}

// TestManagerExecutionInstallsUpdatesAndRestoresExactLocks follows a real offline repository across revisions.
func TestManagerExecutionInstallsUpdatesAndRestoresExactLocks(t *testing.T) {
	source, first := managerGitFixture(t)
	directory := t.TempDir()
	specs := fmt.Sprintf(`{{name="app",source={kind="git",url=%q},selector={kind="branch",value="main"},pin=false}}`, source)
	managerExecute(t, directory, specs, "install", "installed")
	firstObject := filepath.Join(directory, "data", "objects", "app", first)
	if _, err := os.Stat(filepath.Join(firstObject, "plugin.lua")); err != nil {
		t.Fatal(err)
	}

	if err := os.WriteFile(filepath.Join(source, "plugin.lua"), []byte("return { answer = 43 }\n"), 0600); err != nil {
		t.Fatal(err)
	}
	managerGit(t, source, "commit", "--quiet", "-am", "next")
	second := managerGit(t, source, "rev-parse", "HEAD")
	managerExecute(t, directory, specs, "update", "installed")
	lockfile := filepath.Join(directory, "lock.json")
	before, err := os.ReadFile(lockfile)
	if err != nil || !strings.Contains(string(before), second) {
		t.Fatalf("updated lock: %s, %v", before, err)
	}
	if _, err := os.Stat(firstObject); err != nil {
		t.Fatalf("previous immutable checkout removed: %v", err)
	}

	secondObject := filepath.Join(directory, "data", "objects", "app", second)
	if err := os.RemoveAll(secondObject); err != nil {
		t.Fatal(err)
	}
	changedSpec := fmt.Sprintf(`{{name="app",source={kind="git",url=%q},selector={kind="commit",value=%q},pin=true}}`, source, first)
	managerExecute(t, directory, changedSpec, "restore", "installed")
	after, err := os.ReadFile(lockfile)
	if err != nil || string(after) != string(before) {
		t.Fatalf("restore rewrote selected metadata: %s, %v", after, err)
	}
	if _, err := os.Stat(filepath.Join(secondObject, "plugin.lua")); err != nil {
		t.Fatal(err)
	}
}

// TestManagerExecutionPreservesGraphOnDependencyOrDirtyFailure refuses partial lock publication.
func TestManagerExecutionPreservesGraphOnDependencyOrDirtyFailure(t *testing.T) {
	source, commit := managerGitFixture(t)
	directory := t.TempDir()
	specs := fmt.Sprintf(`{{name="app",source={kind="git",url=%q},selector={kind="head"},pin=false}}`, source)
	managerExecute(t, directory, specs, "install", "installed")
	lockfile := filepath.Join(directory, "lock.json")
	before, err := os.ReadFile(lockfile)
	if err != nil {
		t.Fatal(err)
	}
	failed := fmt.Sprintf(`{{name="app",source={kind="git",url=%q},selector={kind="head"},pin=false,dependencies={"missing"}},
 {name="missing",source={kind="git",url="/missing/wraith-fixture"},selector={kind="head"},pin=false}}`, source)
	managerExecute(t, directory, failed, "update", "failed")
	object := filepath.Join(directory, "data", "objects", "app", commit, "plugin.lua")
	if err := os.WriteFile(object, []byte("preserve local changes"), 0600); err != nil {
		t.Fatal(err)
	}
	managerExecute(t, directory, specs, "update", "failed")
	after, err := os.ReadFile(lockfile)
	if err != nil || string(after) != string(before) {
		t.Fatalf("failed update changed graph: %s, %v", after, err)
	}
	data, err := os.ReadFile(object)
	if err != nil || string(data) != "preserve local changes" {
		t.Fatalf("failed update changed checkout: %q, %v", data, err)
	}
}

// TestManagerRestoreRejectsAChangedBaseline keeps a concurrent graph selection intact.
func TestManagerRestoreRejectsAChangedBaseline(t *testing.T) {
	source, _ := managerGitFixture(t)
	directory := t.TempDir()
	specs := fmt.Sprintf(`{{name="app",source={kind="git",url=%q},selector={kind="head"},pin=false}}`, source)
	managerExecute(t, directory, specs, "install", "installed")
	lockfile := filepath.Join(directory, "lock.json")
	r := openManagerRuntime(t, fmt.Sprintf(`local options=require("wraith.manager.store_options").normalize({root=%q,lockfile=%q})
 local baseline,raw=require("wraith.manager.store_lock").read(options.lockfile)
 local plan=require("wraith.manager.store_plan").create(%s,baseline,"restore")
 plan.baseline_raw=raw
 require("wraith.manager.store_execute").start(plan,options,function(phase,name)
  if phase=="verifying" and name==nil then
   baseline.generation="concurrent-selection"
   wraith.fs.write(options.lockfile,wraith.json.encode(baseline),{overwrite=true})
  end
 end,function(result)
  assert(result.state=="conflict",result.error)
  assert(require("wraith.manager.store_lock").read(options.lockfile).generation=="concurrent-selection")
  wraith.notify("complete")
 end)`, filepath.Join(directory, "data"), lockfile, specs))
	runManagerResources(t, r)
}
