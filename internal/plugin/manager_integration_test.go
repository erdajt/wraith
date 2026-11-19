package plugin

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/erdajt/wraith/internal/config"
)

// managerCommand applies publication before dispatching a lazy command or activating its managed work.
func managerCommand(t *testing.T, r *Runtime, name, argument string) Result {
	t.Helper()
	var callback string
	for _, command := range r.Manifest().Commands {
		if command.Name == name {
			callback = command.Callback
		}
	}
	if callback == "" {
		t.Fatalf("missing manager command %q", name)
	}
	pending, err := r.Submit(callback, "", map[string]any{"argument": argument}, r.Manifest().Config)
	if err != nil {
		t.Fatal(err)
	}
	result := receiveResult(t, pending)
	if result.Err != nil {
		t.Fatal(result.Err)
	}
	if result.Manifest != nil {
		if err := r.Acknowledge(result.Revision, true); err != nil {
			t.Fatal(err)
		}
	}
	for _, effect := range result.Effects {
		switch {
		case strings.HasPrefix(effect.Name, "command."):
			result = managerCommand(t, r, strings.TrimPrefix(effect.Name, "command."), effect.Argument)
		case effect.Name == "resource.activate":
			if err := r.Activate([]string{effect.Argument}); err != nil {
				t.Fatal(err)
			}
		case effect.Name != "notify":
			t.Fatalf("unexpected manager command effect: %+v", effect)
		}
	}
	return result
}

// TestManagerLoadsImportedLocalDependenciesRelativeToConfig exercises ordinary user files without preloaded fixtures.
func TestManagerLoadsImportedLocalDependenciesRelativeToConfig(t *testing.T) {
	directory, storage := t.TempDir(), filepath.Join(t.TempDir(), "absent")
	files := map[string]string{
		"init.lua": fmt.Sprintf(`local manager=require("wraith.manager")
 manager.setup({require("plugins")},{storage={root=%q}})
 wraith.commands.create("remove",function()
  assert(not pcall(manager.unload,"dep"))
  manager.unload("app"); manager.unload("dep")
  for _,row in ipairs(manager.status().packages) do assert(not row.managed and not row.loaded) end
 end)`, storage),
		"lua/plugins/init.lua": `return {{import="plugins.app"}}`,
		"lua/plugins/app.lua": `return {{name="app",dir="packages/app",cmd="localrun",opts={increment=1},
 dependencies={{name="dep",dir="packages/dep",lazy=true}}}}`,
		"packages/dep/dep.lua": `return {value=41}`,
		"packages/app/app.lua": `return {setup=function(opts)
 local answer=require("dep").value+opts.increment
 wraith.commands.create("localrun",function() wraith.notify(answer) end)
 end}`,
	}
	for name, contents := range files {
		path := filepath.Join(directory, name)
		if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(contents), 0600); err != nil {
			t.Fatal(err)
		}
	}
	r, err := OpenWithOptions(t.Context(), directory, config.Default(), nil, managerOptions(t))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(r.Close)
	result := managerCommand(t, r, "localrun", "")
	if len(result.Effects) != 1 || result.Effects[0].Argument != "42" {
		t.Fatalf("local imported dependency did not execute: %+v", result)
	}
	managerCommand(t, r, "remove", "")
	for _, command := range r.Manifest().Commands {
		if command.Name == "localrun" {
			t.Fatal("unloaded local package retained its command")
		}
	}
	for _, path := range []string{storage, filepath.Join(directory, "wraith-lock.json")} {
		if _, err := os.Stat(path); !os.IsNotExist(err) {
			t.Fatalf("local activation changed managed storage: %v", err)
		}
	}
}

// TestManagerPublicInstallAndUpdateKeepRunningCodeUntilFreshRuntime uses real git, lock files and Lua module loading.
func TestManagerPublicInstallAndUpdateKeepRunningCodeUntilFreshRuntime(t *testing.T) {
	source, _ := managerGitFixture(t)
	version := func(label string) string {
		t.Helper()
		text := fmt.Sprintf(`return {setup=function()
   wraith.commands.create("version",function() wraith.notify(%q) end)
  end}`, label)
		if err := os.WriteFile(filepath.Join(source, "plugin.lua"), []byte(text), 0600); err != nil {
			t.Fatal(err)
		}
		managerGit(t, source, "add", "plugin.lua")
		managerGit(t, source, "commit", "--quiet", "-m", label)
		return managerGit(t, source, "rev-parse", "HEAD")
	}
	first := version("first")
	directory := t.TempDir()
	root, lockfile := filepath.Join(directory, "packages"), filepath.Join(directory, "wraith-lock.json")
	setup := fmt.Sprintf(`local manager=require("wraith.manager")
 manager.setup({{name="app",url=%q,main="plugin",opts={},cmd="version"}}, {storage={root=%q,lockfile=%q}})
 wraith.commands.create("operation",function(ctx)
  manager[ctx.argument]({"app"})
  local timer
  timer=wraith.timer.start(5,function()
   local operation=manager.status().operation
   if operation and (operation.state=="installed" or operation.state=="failed" or operation.state=="cancelled") then
    assert(operation.state=="installed",operation.error)
    wraith.resource.cancel(timer); wraith.resource.release(timer)
    wraith.notify("complete")
   end
  end,{repeat_ms=5})
 end,"operation",{arguments=true})
 wraith.commands.create("expect",function(ctx)
  local row=manager.status().packages[1]
  assert(row.commit==ctx.argument and row.installed)
  wraith.notify((row.loaded_commit or "none")..":"..tostring(row.restart_required))
 end,"expected commit",{arguments=true})
 wraith.commands.create("recover",function()
  local report=manager.recover()
  assert(report.root==%q and type(report.staging)=="table" and type(report.publication)=="table")
 end)
 `, source, root, lockfile, root)
	before := openManagerRuntime(t, setup)
	for _, path := range []string{root, lockfile} {
		if _, err := os.Stat(path); !os.IsNotExist(err) {
			t.Fatalf("passive setup created %s: %v", path, err)
		}
	}
	managerCommand(t, before, "operation", "install")
	runManagerResources(t, before)
	result := managerCommand(t, before, "expect", first)
	if len(result.Effects) != 1 || result.Effects[0].Argument != "none:true" {
		t.Fatalf("initial install changed frozen selection: %+v", result)
	}
	before.Close()

	running := openManagerRuntime(t, setup)
	result = managerCommand(t, running, "version", "")
	if len(result.Effects) != 1 || result.Effects[0].Argument != "first" {
		t.Fatalf("locked initial code did not load: %+v", result)
	}
	second := version("second")
	managerCommand(t, running, "operation", "update")
	runManagerResources(t, running)
	result = managerCommand(t, running, "expect", second)
	if len(result.Effects) != 1 || result.Effects[0].Argument != first+":true" {
		t.Fatalf("update changed loaded revision metadata: %+v", result)
	}
	result = managerCommand(t, running, "version", "")
	if len(result.Effects) != 1 || result.Effects[0].Argument != "first" {
		t.Fatalf("update replaced running code: %+v", result)
	}
	managerCommand(t, running, "recover", "")

	fresh := openManagerRuntime(t, setup)
	result = managerCommand(t, fresh, "version", "")
	if len(result.Effects) != 1 || result.Effects[0].Argument != "second" {
		t.Fatalf("fresh runtime ignored updated lock: %+v", result)
	}
	result = managerCommand(t, fresh, "expect", second)
	if len(result.Effects) != 1 || result.Effects[0].Argument != second+":false" {
		t.Fatalf("fresh revision metadata differs: %+v", result)
	}
}
