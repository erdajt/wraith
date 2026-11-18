package plugin

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestPackageUnloadRestoresCoveredCommandsAndRemovesCopiedCallbacks(t *testing.T) {
	r := openTestRuntime(t, `
 package.preload.first = function() return {setup=function()
  wraith.commands.create("shared",function() wraith.notify("first") end)
 end} end
 package.preload.second = function() return {setup=function()
  wraith.commands.create("shared",function() wraith.notify("second") end,"",{replace=true})
  wraith.keymap.set("prompt.normal","z",function() wraith.notify("second") end)
 end} end
 wraith.plugins.load("first")
 wraith.plugins.load("second")
 local maps = wraith.config.get("keymaps")
 for _, map in ipairs(maps) do
  if map.keys == "z" then wraith.keymap.set("prompt.normal","x",map.action) end
 end
 wraith.commands.create("unload",function() wraith.plugins.unload("second") end)
 `)
	m := r.Manifest()
	var id string
	for _, command := range m.Commands {
		if command.Name == "unload" {
			id = command.Callback
		}
	}
	ch, err := r.Submit(id, "", nil, m.Config)
	if err != nil {
		t.Fatal(err)
	}
	got := receiveResult(t, ch)
	if got.Err != nil || got.Manifest == nil {
		t.Fatalf("unload: %+v", got)
	}
	for _, binding := range got.Manifest.Config.Keymaps {
		if binding.Keys == "z" || binding.Keys == "x" {
			t.Fatalf("unloaded callback survived: %+v", binding)
		}
	}
	if err := r.Acknowledge(got.Revision, true); err != nil {
		t.Fatal(err)
	}
	m = r.Manifest()
	for _, command := range m.Commands {
		if command.Name == "shared" {
			id = command.Callback
		}
	}
	ch, err = r.Submit(id, "", nil, m.Config)
	if err != nil {
		t.Fatal(err)
	}
	got = receiveResult(t, ch)
	if got.Err != nil || len(got.Effects) != 1 || got.Effects[0].Argument != "first" {
		t.Fatalf("underlying owner was not restored: %+v", got)
	}
}

func TestPackageDependenciesAndCleanupOrder(t *testing.T) {
	marker := filepath.Join(t.TempDir(), "cleanup")
	r := openTestRuntime(t, fmt.Sprintf(`
 local function record(text) local f=assert(io.open(%q,"a")); f:write(text); f:close() end
 package.preload.child=function() return {setup=function() end,teardown=function() record("child\n") end} end
 package.preload.parent=function() return {
  setup=function() wraith.plugins.load("child") end,
  teardown=function() record("parent\n") end
 } end
 wraith.plugins.load("parent")
 assert(table.concat(wraith.plugins.list(),",")=="child,parent")
 wraith.commands.create("unload",function()
  local ok,err=pcall(wraith.plugins.unload,"child")
  assert(not ok and string.find(err,"required by parent"))
 end)
 `, marker))
	m := r.Manifest()
	ch, err := r.Submit(m.Commands[0].Callback, "", nil, m.Config)
	if err != nil {
		t.Fatal(err)
	}
	got := receiveResult(t, ch)
	if got.Err != nil {
		t.Fatal(got.Err)
	}
	if got.Manifest != nil {
		if err := r.Acknowledge(got.Revision, true); err != nil {
			t.Fatal(err)
		}
	}
	r.Close()
	<-r.Done()
	data, err := os.ReadFile(marker)
	if err != nil || string(data) != "parent\nchild\n" {
		t.Fatalf("dependency teardown order: %q, %v", data, err)
	}
}

func TestRejectedUnloadKeepsResourcesAndRegistrations(t *testing.T) {
	marker := filepath.Join(t.TempDir(), "closed")
	r := openTestRuntime(t, fmt.Sprintf(`
 package.preload.resource=function() return {
 setup=function() wraith.commands.create("resource",function() wraith.notify("alive") end) end,
 teardown=function() local f=assert(io.open(%q,"w")); f:write("closed"); f:close() end
 } end
 wraith.plugins.load("resource")
 wraith.commands.create("unload",function() wraith.plugins.unload("resource") end)
 `, marker))
	m := r.Manifest()
	ch, err := r.Submit(m.Commands[1].Callback, "", nil, m.Config)
	if err != nil {
		t.Fatal(err)
	}
	got := receiveResult(t, ch)
	if got.Err != nil || got.Manifest == nil {
		t.Fatalf("unload: %+v", got)
	}
	if err := r.Acknowledge(got.Revision, false); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(marker); !os.IsNotExist(err) {
		t.Fatalf("rejected unload disposed resource: %v", err)
	}
	m = r.Manifest()
	ch, err = r.Submit(m.Commands[0].Callback, "", nil, m.Config)
	if err != nil {
		t.Fatal(err)
	}
	got = receiveResult(t, ch)
	if got.Err != nil || len(got.Effects) != 1 || !strings.Contains(got.Effects[0].Argument, "alive") {
		t.Fatalf("rejected unload lost callback: %+v", got)
	}
}

func TestFailedParentSetupCleansParentBeforeDependency(t *testing.T) {
	marker := filepath.Join(t.TempDir(), "cleanup")
	r := openTestRuntime(t, fmt.Sprintf(`
 local function record(text) local f=assert(io.open(%q,"a")); f:write(text); f:close() end
 package.preload.child=function() return {setup=function() end,teardown=function() record("child\n") end} end
 package.preload.parent=function() return {
  setup=function() wraith.plugins.load("child"); error("setup failed") end,
  teardown=function() record("parent\n") end
 } end
 assert(not pcall(wraith.plugins.load,"parent"))
 assert(#wraith.plugins.list()==0)
 `, marker))
	defer r.Close()
	data, err := os.ReadFile(marker)
	if err != nil || string(data) != "parent\nchild\n" {
		t.Fatalf("failed setup cleanup order: %q, %v", data, err)
	}
}
