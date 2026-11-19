package plugin

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

// TestManagerCleanupRemovesSuccessfulPrivateFiles keeps unchanged updates from retaining duplicate repositories.
func TestManagerCleanupRemovesSuccessfulPrivateFiles(t *testing.T) {
	source, commit := managerGitFixture(t)
	directory := t.TempDir()
	specs := fmt.Sprintf(`{{name="app",source={kind="git",url=%q},selector={kind="head"},pin=false}}`, source)
	for _, action := range []string{"install", "update", "restore"} {
		managerExecute(t, directory, specs, action, "installed")
		entries, err := os.ReadDir(filepath.Join(directory, "data", "staging"))
		if err != nil || len(entries) != 0 {
			t.Fatalf("%s retained staging: %v, %v", action, entries, err)
		}
		for _, pattern := range []string{".wraith-baseline.*", ".wraith-candidate.*"} {
			matches, err := filepath.Glob(filepath.Join(directory, pattern))
			if err != nil || len(matches) != 0 {
				t.Fatalf("%s retained publication files: %v, %v", action, matches, err)
			}
		}
		for _, path := range []string{filepath.Join(directory, ".wraith-packages.lock"), filepath.Join(directory, "data", "objects", "app", commit, "plugin.lua")} {
			if _, err := os.Stat(path); err != nil {
				t.Fatalf("cleanup removed persistent path: %v", err)
			}
		}
	}
}

// TestManagerCleanupPreservesConfirmedOutcome keeps cancellation and filesystem errors separate from publication.
func TestManagerCleanupPreservesConfirmedOutcome(t *testing.T) {
	for _, mode := range []string{"cancel", "failure"} {
		t.Run(mode, func(t *testing.T) {
			directory := t.TempDir()
			r := openManagerRuntime(t, fmt.Sprintf(`local mode=%q
 local remove=wraith.fs.remove_async
 if mode=="failure" then
  wraith.fs.remove_async=function(path,options,callback)
   return remove(path..".missing",{recursive=false},callback)
  end
 end
 local store=require("wraith.manager.store").new({}, {root=%q,lockfile=%q})
 store.execute((store.plan("install")),function(progress)
  if mode=="cancel" and progress.phase=="cleaning" then store.cancel(progress.id) end
 end,function(result)
  assert(result.state=="installed" and result.cleanup_warning and result.error==nil)
  local lock=require("wraith.manager.store_lock").read(%q)
  assert(result.generation==lock.generation)
  local report=store.recover()
  assert(#report.staging==1 and #report.publication==1)
  wraith.notify("complete")
 end)`, mode, filepath.Join(directory, "data"), filepath.Join(directory, "lock.json"), filepath.Join(directory, "lock.json")))
			runManagerResources(t, r)
		})
	}
}

// TestManagerCleanupCallbackFailureRetainsInstalledStatus reconciles failed cleanup without denying a committed graph.
func TestManagerCleanupCallbackFailureRetainsInstalledStatus(t *testing.T) {
	directory := t.TempDir()
	r := openManagerRuntime(t, fmt.Sprintf(`local remove=wraith.fs.remove_async
 local failed=false
 wraith.fs.remove_async=function(path,options,callback)
  return remove(path,options,function(ctx)
   callback(ctx)
   if ctx.resource.final and not failed then failed=true; error("cleanup continuation rejected") end
  end)
 end
 local store=require("wraith.manager.store").new({}, {root=%q,lockfile=%q})
 store.execute((store.plan("install")))
 wraith.commands.create("inspect",function()
  local result=store.status()
  assert(result.state=="installed" and result.cleanup_warning and result.error==nil)
  assert(result.generation==require("wraith.manager.store_lock").read(%q).generation)
 end)`, filepath.Join(directory, "data"), filepath.Join(directory, "lock.json"), filepath.Join(directory, "lock.json")))
	if err := r.Activate(r.Manifest().Resources); err != nil {
		t.Fatal(err)
	}
	for {
		event := receiveResource(t, r)
		if !r.ResourceValid(event) {
			continue
		}
		pending, err := r.SubmitResource(event, nil, r.Manifest().Config)
		if err != nil {
			t.Fatal(err)
		}
		result := receiveResult(t, pending)
		if result.Err != nil {
			break
		}
		for _, effect := range result.Effects {
			switch effect.Name {
			case "resource.activate":
				if err := r.Activate([]string{effect.Argument}); err != nil {
					t.Fatal(err)
				}
			case "resource.release":
				r.DiscardResources([]string{effect.Argument})
			default:
				t.Fatalf("unexpected effect: %+v", effect)
			}
		}
	}
	var callback string
	for _, command := range r.Manifest().Commands {
		if command.Name == "inspect" {
			callback = command.Callback
		}
	}
	pending, err := r.Submit(callback, "", nil, r.Manifest().Config)
	if err != nil {
		t.Fatal(err)
	}
	result := receiveResult(t, pending)
	if result.Err != nil {
		t.Fatal(result.Err)
	}
	for _, effect := range result.Effects {
		if effect.Name != "resource.release" {
			t.Fatalf("unexpected effect: %+v", effect)
		}
		r.DiscardResources([]string{effect.Argument})
	}
	r.services.mu.Lock()
	remaining := len(r.services.records)
	r.services.mu.Unlock()
	if remaining != 0 {
		t.Fatalf("failed cleanup retained %d handles", remaining)
	}
}
