package plugin

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

// TestManagerDevelopmentFallbackUsesFilesystemResults checks real filesystem outcomes through public setup.
func TestManagerDevelopmentFallbackUsesFilesystemResults(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "file"), []byte("ordinary file"), 0600); err != nil {
		t.Fatal(err)
	}
	for name, target := range map[string]string{"valid-link": root, "dangling": "missing", "loop": "loop"} {
		if err := os.Symlink(target, filepath.Join(root, name)); err != nil {
			t.Fatal(err)
		}
	}
	locked := filepath.Join(root, "locked")
	if err := os.Mkdir(locked, 0); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := os.Chmod(locked, 0700); err != nil {
			t.Error(err)
		}
	})

	for _, tc := range []struct {
		name, path  string
		local, fail bool
	}{
		{"directory", root, true, false},
		{"file", filepath.Join(root, "file"), false, false},
		{"missing", filepath.Join(root, "missing"), false, false},
		{"file ancestor", filepath.Join(root, "file", "child"), false, false},
		{"dangling", filepath.Join(root, "dangling"), false, false},
		{"symlink directory", filepath.Join(root, "valid-link"), true, false},
		{"symlink loop", filepath.Join(root, "loop"), false, true},
		{"permission", filepath.Join(locked, "child"), false, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if tc.name == "permission" && os.Geteuid() == 0 {
				t.Skip("root bypasses directory permissions")
			}
			storage := t.TempDir()
			openManagerRuntime(t, fmt.Sprintf(`local schema=require("wraith.manager.spec")
local manager=require("shroud")
local path=%q
local input={{name="tool",url="/missing/development-source",dev=true,branch="work",pin=true,lazy=true}}
local dev={path=function() return path end,fallback=true}
local ok,graph=pcall(schema.normalize,input,nil,dev)
if %t then
 assert(not ok)
 assert(not pcall(manager.setup,input,{dev=dev}))
 assert(not pcall(manager.status))
 return
end
assert(ok,graph)
local spec=graph.by_name.tool
assert(spec.declared_selector.value=="work" and spec.declared_pin)
if %t then
 assert(spec.dev and spec.source.kind=="local" and spec.source.dir==path)
 assert(spec.selector.kind=="head" and not spec.pin)
else
 assert(spec.dev==false and spec.source.kind=="git" and spec.source.url==input[1].url)
 assert(spec.selector.value=="work" and spec.pin)
end
manager.setup(input,{dev=dev,storage={root=%q,lockfile=%q}})
local row=manager.status().packages[1]
assert(row.source.kind==spec.source.kind and row.dev==spec.dev and not row.loaded)
assert(row.installed==%t and row.commit==nil)
assert(row.declared_selector.value=="work" and row.declared_pin)`, tc.path, tc.fail, tc.local,
				filepath.Join(storage, "packages"), filepath.Join(storage, "lock.json"), tc.local))
			if _, err := os.Stat(filepath.Join(storage, "packages", "objects")); !os.IsNotExist(err) {
				t.Fatalf("selection created git objects: %v", err)
			}
		})
	}
}

// TestManagerDevelopmentMissingWithoutFallbackDoesNotFetch keeps unavailable development roots local.
func TestManagerDevelopmentMissingWithoutFallbackDoesNotFetch(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "absent")
	openManagerRuntime(t, fmt.Sprintf(`local manager=require("shroud")
local input={{name="tool",url="/missing/development-source",dev=true,lazy=true}}
local dev={path=function() return %q end,fallback=false}
local graph=require("wraith.manager.spec").normalize(input,nil,dev)
assert(graph.by_name.tool.source.dir==%q and graph.by_name.tool.dev)
manager.setup(input,{dev=dev,storage={root=%q,lockfile=%q}})
local row=manager.status().packages[1]
assert(row.source.kind=="local" and row.source.dir==%q and row.dev)
assert(not row.installed and row.error and row.commit==nil)
assert(row.error:find(row.source.dir,1,true),row.error)
local ok,failure=pcall(manager.load,"tool")
assert(not ok and tostring(failure):find(row.source.dir,1,true),tostring(failure))
assert(manager.status().operation==nil)`, path, path, filepath.Join(root, "packages"), filepath.Join(root, "lock.json"), path))
	if _, err := os.Stat(filepath.Join(root, "packages", "objects")); !os.IsNotExist(err) {
		t.Fatalf("missing development root fetched remotely: %v", err)
	}
}

// TestManagerDevelopmentFallbackInstallsDeclaredRevision verifies the checkout and persisted pin.
func TestManagerDevelopmentFallbackInstallsDeclaredRevision(t *testing.T) {
	source, commit := managerGitFixture(t)
	root := t.TempDir()
	lockfile := filepath.Join(root, "lock.json")
	r := openManagerRuntime(t, fmt.Sprintf(`local manager=require("shroud")
manager.setup({{name="tool",url=%q,branch="main",pin=true,dev=true,lazy=true}},
 {dev={path=%q,fallback=true},storage={root=%q,lockfile=%q}})
local row=manager.status().packages[1]
assert(row.source.kind=="git" and row.dev==false and row.selector.value=="main" and row.pin)
local plan=manager.plan("install",{"tool"})
assert(plan.summary.packages[1].selector.value=="main" and plan.summary.packages[1].pin)
manager.execute(plan.id)
local timer
timer=wraith.timer.start(5,function()
 local operation=manager.status().operation
 if operation.state=="installed" or operation.state=="failed" or operation.state=="cancelled" then
  assert(operation.state=="installed",operation.error)
  local entry=require("wraith.manager.store_lock").read(%q).packages.tool
  assert(entry.commit==%q and entry.source.url==%q)
  assert(entry.selector.kind=="branch" and entry.selector.value=="main" and entry.pin)
  local installed=manager.status().packages[1]
  assert(installed.installed and installed.commit==entry.commit and installed.dev==false)
  wraith.resource.cancel(timer)
  wraith.resource.release(timer)
  wraith.notify("complete")
 end
end,{repeat_ms=5})`, source, filepath.Join(root, "absent"), filepath.Join(root, "packages"), lockfile, lockfile, commit, source))
	runManagerResources(t, r)
	checkout := filepath.Join(root, "packages", "objects", "tool", commit, "plugin.lua")
	if data, err := os.ReadFile(checkout); err != nil || string(data) != "return { answer = 42 }\n" {
		t.Fatalf("fallback checkout: %q, %v", data, err)
	}
}
