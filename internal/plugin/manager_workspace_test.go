package plugin

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

// TestManagerWorkspaceCreatesPrivateSiblingsWithoutChangingTheLock prepares transaction files through real workers.
func TestManagerWorkspaceCreatesPrivateSiblingsWithoutChangingTheLock(t *testing.T) {
	directory := filepath.Join(t.TempDir(), "literal ' $value")
	if err := os.Mkdir(directory, 0700); err != nil {
		t.Fatal(err)
	}
	root, lockfile := filepath.Join(directory, "data"), filepath.Join(directory, "lock.json")
	baseline := `{"version":1,"generation":"old","packages":{}}`
	if err := os.WriteFile(lockfile, []byte(baseline), 0600); err != nil {
		t.Fatal(err)
	}
	r := openTestRuntime(t, fmt.Sprintf(`local operation
 operation=require("wraith.manager.store_workspace").prepare({root=%q,lockfile=%q},%q,function(result)
 assert(result.state=="completed",result.error)
 local work=result.workspace
 assert(work.lockfile==%q and not work.absent)
 assert(wraith.fs.read(work.baseline).data==%q and wraith.fs.read(work.candidate).data=="")
 assert(wraith.fs.stat(work.stage).type=="directory" and wraith.fs.stat(work.guard).type=="file")
 assert(wraith.fs.stat(work.baseline).mode==384 and wraith.fs.stat(work.candidate).mode==384)
 assert(work.generation:match("^txn%%.[A-Za-z0-9]+$"))
 assert(wraith.resource.status(operation.resource()).done)
 wraith.notify("complete")
end)`, root, lockfile, baseline, lockfile, baseline))
	runManagerResources(t, r)
	data, err := os.ReadFile(lockfile)
	if err != nil || string(data) != baseline {
		t.Fatalf("workspace changed lock: %s, %v", data, err)
	}
}

// TestManagerWorkspaceChecksToolsBeforeWriting refuses an incomplete process environment without staging files.
func TestManagerWorkspaceChecksToolsBeforeWriting(t *testing.T) {
	t.Setenv("PATH", t.TempDir())
	directory := t.TempDir()
	root := filepath.Join(directory, "not-created")
	r := openTestRuntime(t, fmt.Sprintf(`require("wraith.manager.store_workspace").prepare({root=%q,lockfile=%q},nil,function(result)
 assert(result.state=="failed" and result.error:find("requires git",1,true),result.error)
 wraith.notify("complete")
end)`, root, filepath.Join(directory, "lock.json")))
	runManagerResources(t, r)
	if _, err := os.Stat(root); !os.IsNotExist(err) {
		t.Fatalf("tool failure created staging root: %v", err)
	}
}

// TestManagerLockRequiresExistingConfigurationParent reports missing configuration ancestry before mutations.
func TestManagerLockRequiresExistingConfigurationParent(t *testing.T) {
	path := filepath.Join(t.TempDir(), "missing", "nested", "lock.json")
	r := openTestRuntime(t, fmt.Sprintf(`local ok,err=pcall(require("wraith.manager.store_lock").read,%q)
 assert(not ok and tostring(err):find("parent must exist and be readable",1,true),tostring(err))`, path))
	if len(r.Manifest().Resources) != 0 {
		t.Fatal("passive lock inspection reserved resources")
	}
	if _, err := os.Stat(filepath.Dir(path)); !os.IsNotExist(err) {
		t.Fatalf("inspection created configuration ancestry: %v", err)
	}
}

// TestManagerWorkspaceCancellationLeavesNoFiles stops the initial reserved preflight before activation.
func TestManagerWorkspaceCancellationLeavesNoFiles(t *testing.T) {
	directory := t.TempDir()
	root := filepath.Join(directory, "absent")
	r := openTestRuntime(t, fmt.Sprintf(`local operation=require("wraith.manager.store_workspace").prepare({root=%q,lockfile=%q},nil,function(result)
 assert(result.state=="cancelled",result.error)
 wraith.notify("complete")
end)
operation.cancel()`, root, filepath.Join(directory, "lock.json")))
	runManagerResources(t, r)
	if _, err := os.Stat(root); !os.IsNotExist(err) {
		t.Fatalf("cancelled workspace created files: %v", err)
	}
}

// TestManagerWorkspacePreservesGuardInode keeps concurrent managers in the same advisory lock domain.
func TestManagerWorkspacePreservesGuardInode(t *testing.T) {
	directory := t.TempDir()
	guard := filepath.Join(directory, ".wraith-packages.lock")
	if err := os.WriteFile(guard, []byte("retained"), 0600); err != nil {
		t.Fatal(err)
	}
	before, err := os.Stat(guard)
	if err != nil {
		t.Fatal(err)
	}
	for range 2 {
		r := openTestRuntime(t, fmt.Sprintf(`require("wraith.manager.store_workspace").prepare({root=%q,lockfile=%q},nil,function(result)
 assert(result.state=="completed",result.error)
 wraith.notify("complete")
end)`, filepath.Join(directory, "data"), filepath.Join(directory, "lock.json")))
		runManagerResources(t, r)
	}
	after, err := os.Stat(guard)
	if err != nil || !os.SameFile(before, after) {
		t.Fatalf("guard inode replaced: %v", err)
	}
	data, err := os.ReadFile(guard)
	if err != nil || string(data) != "retained" {
		t.Fatalf("guard content changed: %q, %v", data, err)
	}
}

// TestManagerWorkspaceRejectsManagedDirectorySymlinks refuses root and staging aliases before writing through them.
func TestManagerWorkspaceRejectsManagedDirectorySymlinks(t *testing.T) {
	for _, kind := range []string{"root", "staging"} {
		t.Run(kind, func(t *testing.T) {
			directory, outside := t.TempDir(), t.TempDir()
			root := filepath.Join(directory, "data")
			alias := root
			if kind == "staging" {
				if err := os.Mkdir(root, 0700); err != nil {
					t.Fatal(err)
				}
				alias = filepath.Join(root, "staging")
			}
			if err := os.Symlink(outside, alias); err != nil {
				t.Fatal(err)
			}
			r := openTestRuntime(t, fmt.Sprintf(`require("wraith.manager.store_workspace").prepare({root=%q,lockfile=%q},nil,function(result)
 assert(result.state=="failed" and result.error:find("not a symlink",1,true),result.error)
 wraith.notify("complete")
end)`, root, filepath.Join(directory, "lock.json")))
			runManagerResources(t, r)
			entries, err := os.ReadDir(outside)
			if err != nil || len(entries) != 0 {
				t.Fatalf("wrote through managed alias: %v, %v", entries, err)
			}
		})
	}
}
