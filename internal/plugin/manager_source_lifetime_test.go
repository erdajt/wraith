package plugin

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// managerSourceFixture keeps storage metadata controllable while loading actual package code from disk.
func managerSourceFixture(t *testing.T) string {
	t.Helper()
	roots := []string{t.TempDir(), t.TempDir()}
	for index, root := range roots {
		source := fmt.Sprintf(`return {version=%q,setup=function(opts)
   if opts.fail then error("setup failed") end
   wraith.commands.create("version",function() wraith.notify(%q..":"..opts.value) end)
  end}`, string(rune('a'+index)), string(rune('a'+index)))
		if err := os.WriteFile(filepath.Join(root, "app.lua"), []byte(source), 0600); err != nil {
			t.Fatal(err)
		}
	}
	return fmt.Sprintf(`local root_a,root_b=%q,%q
 local current_path,current_commit=root_a,"first"
 package.preload["wraith.manager.store"]=function() return {new=function(specs)
  return {inspect=function()
   local rows={}; for _,spec in ipairs(specs) do rows[#rows+1]={name=spec.name,source=spec.source,path=current_path,commit=current_commit,state="installed"} end
   return rows
  end,status=function() end,cancel=function() end}
 end} end
 local manager=require("wraith.manager")
 local spec={name="app",main="app",url="/repo",lazy=true,opts={value="first"}}
 local function reset() manager.unload_all(); wraith.plugins.unload("wraith.manager.runtime") end
 `, roots[0], roots[1])
}

// TestManagerRejectsChangedCachedSources prevents metadata from claiming a revision whose code was not loaded.
func TestManagerRejectsChangedCachedSources(t *testing.T) {
	for _, change := range []string{
		`current_path=root_b; current_commit="second"`,
		`current_commit="second"`,
		`spec.url="/other"`,
		`spec.main="renamed"`,
		`spec.name="renamed"`,
	} {
		t.Run(change, func(t *testing.T) {
			openManagerRuntime(t, managerSourceFixture(t)+`
    manager.setup({spec}); manager.load("app")
    local original=require("app")
    assert(original.version=="a")
    reset()
    `+change+`
    local initialized=false; spec.init=function() initialized=true end
    local ok,err=pcall(manager.setup,{spec})
    assert(not ok and tostring(err):find("/reload",1,true),tostring(err))
    assert(not initialized and original.version=="a" and package.loaded.app==original)
    local imported,import_err=pcall(require,"app")
    assert(not imported and tostring(import_err):find("no resolver",1,true),tostring(import_err))
   `)
		})
	}
}

// TestManagerReconfigurationDistinguishesUnexposedAndCachedModules preserves options changes without inventing hot reload.
func TestManagerReconfigurationDistinguishesUnexposedAndCachedModules(t *testing.T) {
	for _, mode := range []string{"unexposed", "cached", "failed"} {
		t.Run(mode, func(t *testing.T) {
			r := openManagerRuntime(t, managerSourceFixture(t)+fmt.Sprintf(`
local mode = %q
if mode == "failed" then
 spec.opts.fail = true
end
manager.setup({spec})
if mode == "cached" then
 manager.load("app")
elseif mode == "failed" then
 assert(not pcall(manager.load, "app"))
 assert(package.loaded.app == nil)
end

wraith.commands.create("reconfigure", function(ctx)
 if ctx.argument == "retire" then
  reset()
  spec.opts = {value = "second"}
  if mode ~= "cached" then
   current_path, current_commit = root_b, "second"
  end
  return
 end

 assert(package.loaded.app == nil)
 if mode == "failed" then
  local ok, err = pcall(manager.setup, {spec})
  assert(not ok and tostring(err):find("/reload", 1, true), tostring(err))
  assert(package.loaded.app == nil)
  return
 end

 manager.setup({spec})
 manager.load("app")
 assert(manager.status().packages[1].loaded_commit == (mode == "cached" and "first" or "second"))
end, "reconfigure", {arguments = true})
`, mode))
			managerCommand(t, r, "reconfigure", "retire")
			managerCommand(t, r, "reconfigure", "load")
			if mode == "failed" {
				return
			}
			var callback string
			for _, command := range r.Manifest().Commands {
				if command.Name == "version" {
					callback = command.Callback
				}
			}
			pending, err := r.Submit(callback, "", nil, r.Manifest().Config)
			if err != nil {
				t.Fatal(err)
			}
			result := receiveResult(t, pending)
			want := "a:second"
			if mode == "unexposed" {
				want = "b:second"
			}
			if result.Err != nil || len(result.Effects) != 1 || result.Effects[0].Argument != want {
				t.Fatalf("executed wrong cached module: %+v", result)
			}
		})
	}
}

// TestManagerRejectedActivationStillReservesItsModuleSource retains provenance after verified cache rollback.
func TestManagerRejectedActivationStillReservesItsModuleSource(t *testing.T) {
	r := openManagerRuntime(t, managerSourceFixture(t)+`
  manager.setup({spec})
  wraith.commands.create("operate",function(ctx)
   if ctx.argument=="load" then manager.load("app"); return end
   reset(); current_path=root_b; current_commit="second"
   local ok,err=pcall(manager.setup,{spec})
   assert(not ok and tostring(err):find("/reload",1,true))
   assert(package.loaded.app==nil)
  end,"operate",{arguments=true})
 `)
	var callback string
	for _, command := range r.Manifest().Commands {
		if command.Name == "operate" {
			callback = command.Callback
		}
	}
	for _, operation := range []string{"load", "reconfigure"} {
		pending, err := r.Submit(callback, "", map[string]any{"argument": operation}, r.Manifest().Config)
		if err != nil {
			t.Fatal(err)
		}
		result := receiveResult(t, pending)
		if result.Err != nil || result.Manifest == nil {
			t.Fatalf("%s: %+v", operation, result)
		}
		if err := r.Acknowledge(result.Revision, operation != "load"); err != nil {
			t.Fatal(err)
		}
	}
	for _, command := range r.Manifest().Commands {
		if strings.HasPrefix(command.Name, "packages") {
			t.Fatal("failed reconfiguration retained manager commands")
		}
	}
}
