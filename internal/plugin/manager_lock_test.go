package plugin

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

// TestManagerLockPreservesExactBaselineAndRejectsCorruption validates stored data through the same source policy as specs.
func TestManagerLockPreservesExactBaselineAndRejectsCorruption(t *testing.T) {
	directory := t.TempDir()
	lock := filepath.Join(directory, "wraith-lock.json")
	path := managerModule(t, "")
	openManagerRuntime(t, fmt.Sprintf(`package.preload["wraith.manager.spec"]=assert(loadfile(%q))
local codec=dofile(%q)
local empty,baseline=codec.read(%q)
assert(empty.version==1 and next(empty.packages)==nil and baseline==nil)
local value={version=1,generation="txn.example",packages={lib={source={kind="git",url="https://example.com/lib"},selector={kind="tag",value="v1"},pin=true,commit=string.rep("a",40)}}}
local encoded="  "..wraith.json.encode(value).."\n"
wraith.fs.write(%q,encoded)
local decoded,bytes=codec.read(%q)
assert(bytes==encoded and decoded.packages.lib.commit==string.rep("a",40))
for _,bad in ipairs({
 '{"version":2,"generation":"next","packages":{}}',
 '{"version":1,"generation":"next","packages":[]}',
 '{"version":1,"generation":"next","packages":{},"extra":true}',
 '{"version":1,"generation":"bad/path","packages":{}}',
 "not json",
}) do assert(not pcall(codec.decode,bad)) end
value.packages.lib.commit="main"
assert(not pcall(codec.decode,wraith.json.encode(value)))
value.packages.lib.commit=string.rep("a",40)
value.packages.lib.source.url="https://user:secret@example.com/lib"
assert(not pcall(codec.decode,wraith.json.encode(value)))
value.packages.lib.source={kind="local",dir="/explicit/local"}
value.packages.lib.selector={kind="head"}
value.packages.lib.pin=false
assert(not pcall(codec.decode,wraith.json.encode(value)))`, filepath.Join(path, "spec.lua"), filepath.Join(path, "store_lock.lua"), lock, lock, lock))
}

// TestManagerLockRejectsSymlinkAndOversizedInputs preserves unexpected local state instead of resetting it.
func TestManagerLockRejectsSymlinkAndOversizedInputs(t *testing.T) {
	directory := t.TempDir()
	target := filepath.Join(directory, "large")
	if err := os.WriteFile(target, make([]byte, (1<<20)+1), 0600); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(directory, "linked")
	if err := os.Symlink(target, link); err != nil {
		t.Fatal(err)
	}
	path := managerModule(t, "")
	openManagerRuntime(t, fmt.Sprintf(`package.preload["wraith.manager.spec"]=assert(loadfile(%q))
local codec=dofile(%q)
assert(not pcall(codec.read,%q))
assert(not pcall(codec.read,%q))`, filepath.Join(path, "spec.lua"), filepath.Join(path, "store_lock.lua"), link, target))
}
