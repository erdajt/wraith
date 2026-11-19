package plugin

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

// TestManagerFilesChainOwnedAtomicWrites uses the same async lifecycle as installer staging.
func TestManagerFilesChainOwnedAtomicWrites(t *testing.T) {
	path := managerModule(t, "")
	target := filepath.Join(t.TempDir(), "candidate.json")
	r := openManagerRuntime(t, fmt.Sprintf(`package.preload["wraith.manager.store_wait"]=assert(loadfile(%q))
local files=dofile(%q)
files.start("write",{%q,"candidate",{overwrite=false}},function(write)
 assert(write.state=="completed" and write.result.bytes==9)
 files.start("read",{%q},function(read)
  assert(read.state=="completed" and read.result.data=="candidate")
  wraith.notify("complete")
 end)
end)`, filepath.Join(path, "store_wait.lua"), filepath.Join(path, "store_files.lua"), target, target))
	if _, err := os.Stat(target); !os.IsNotExist(err) {
		t.Fatalf("staging started before acceptance: %v", err)
	}
	runManagerResources(t, r)
}

// TestManagerFilesPreservesExistingCandidate reports no-replace failures without discarding earlier data.
func TestManagerFilesPreservesExistingCandidate(t *testing.T) {
	path := managerModule(t, "")
	target := filepath.Join(t.TempDir(), "candidate.json")
	if err := os.WriteFile(target, []byte("keep"), 0600); err != nil {
		t.Fatal(err)
	}
	r := openManagerRuntime(t, fmt.Sprintf(`package.preload["wraith.manager.store_wait"]=assert(loadfile(%q))
local files=dofile(%q)
files.start("write",{%q,"replace",{overwrite=false}},function(result)
 assert(result.state=="failed" and result.error)
 wraith.notify("complete")
end)`, filepath.Join(path, "store_wait.lua"), filepath.Join(path, "store_files.lua"), target))
	runManagerResources(t, r)
	data, err := os.ReadFile(target)
	if err != nil || string(data) != "keep" {
		t.Fatalf("failed candidate write changed original: %q, %v", data, err)
	}
}

// TestManagerFilesCancelsUnacceptedWrites leaves no staged file when cancellation precedes activation.
func TestManagerFilesCancelsUnacceptedWrites(t *testing.T) {
	path := managerModule(t, "")
	target := filepath.Join(t.TempDir(), "candidate.json")
	r := openManagerRuntime(t, fmt.Sprintf(`package.preload["wraith.manager.store_wait"]=assert(loadfile(%q))
local files=dofile(%q)
local operation=files.start("write",{%q,"cancelled"},function(result)
 assert(result.state=="cancelled" and result.error)
 wraith.notify("complete")
end)
operation.cancel()`, filepath.Join(path, "store_wait.lua"), filepath.Join(path, "store_files.lua"), target))
	runManagerResources(t, r)
	if _, err := os.Stat(target); !os.IsNotExist(err) {
		t.Fatalf("cancelled write changed filesystem: %v", err)
	}
}
