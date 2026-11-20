package plugin

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestManagerDevelopmentPathChangesRequireReload preserves exposed code when a development path changes.
func TestManagerDevelopmentPathChangesRequireReload(t *testing.T) {
	roots := []string{t.TempDir(), t.TempDir()}
	for index, root := range roots {
		module := fmt.Sprintf(`return {version=%d,setup=function() end}`, index)
		if err := os.WriteFile(filepath.Join(root, "app.lua"), []byte(module), 0600); err != nil {
			t.Fatal(err)
		}
	}
	openManagerRuntime(t, fmt.Sprintf(`local manager=require("shroud")
local current=%q
local spec={"owner/app",main="app",opts={},lazy=true,dev=true}
local options={dev={path=function() return current end}}
manager.setup({spec},options)
manager.load("app")
local original=require("app")
assert(original.version==0)
manager.unload_all()
wraith.plugins.unload("wraith.manager.runtime")
current=%q
local initialized=false
spec.init=function() initialized=true end
local ok,err=pcall(manager.setup,{spec},options)
assert(not ok and tostring(err):find("/reload",1,true),tostring(err))
assert(not initialized and original.version==0 and package.loaded.app==original)
local imported,import_err=pcall(require,"app")
assert(not imported and tostring(import_err):find("no resolver",1,true),tostring(import_err))`, roots[0], roots[1]))
}

// TestManagerDevelopmentDeadlineDiscardsPublication keeps interrupted path callbacks outside the package lifecycle.
func TestManagerDevelopmentDeadlineDiscardsPublication(t *testing.T) {
	r := openManagerRuntime(t, fmt.Sprintf(`local manager=require("shroud")
local entered=false
local spec={"owner/app",dev=true,lazy=true}
wraith.commands.create("timeout",function()
 manager.setup({spec},{dev={path=function()
  entered=true
  wraith.commands.create("unpublished",function() end)
  wraith.notify("unpublished")
  while true do end
 end}})
end)
wraith.commands.create("verify",function()
 assert(entered and not pcall(manager.status))
 for _,name in ipairs(wraith.plugins.list()) do assert(name~="wraith.manager.runtime") end
 manager.setup({spec},{dev={path=function() return %q end}})
 local row=manager.status().packages[1]
 assert(row.dev and row.installed and row.managed and not row.loaded)
 wraith.notify("fresh setup")
end)`, t.TempDir()))
	manifest := r.Manifest()
	var callback string
	for _, command := range manifest.Commands {
		if command.Name == "timeout" {
			callback = command.Callback
		}
	}
	if callback == "" {
		t.Fatal("timeout command is missing")
	}
	pending, err := r.Submit(callback, "", nil, manifest.Config)
	if err != nil {
		t.Fatal(err)
	}
	result := receiveResult(t, pending)
	if result.Err == nil || !strings.Contains(result.Err.Error(), "context deadline exceeded") {
		t.Fatalf("development path did not reach the callback deadline: %v", result.Err)
	}
	if result.Manifest != nil || len(result.Effects) != 0 {
		t.Fatalf("interrupted development path published changes: %+v", result)
	}
	managerCommand(t, r, "verify", "")
	for _, command := range r.Manifest().Commands {
		if command.Name == "unpublished" {
			t.Fatal("interrupted development path retained its command")
		}
	}
}
