package plugin

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/erdajt/wraith/internal/config"
	lua "github.com/yuin/gopher-lua"
)

// TestPublicModuleProviderActivatesCapturedRequire routes imports captured before resolver registration.
func TestPublicModuleProviderActivatesCapturedRequire(t *testing.T) {
	r := openTestRuntime(t, `
assert(wraith.api_version==2 and wraith.capabilities.module_providers)
package.preload.captured=function()
 local import=require
 return function() return import("target") end
end
local captured=require("captured")
local proof={}
local descriptor={plugin="provider",activate=function()
 wraith.plugins.load("provider",{},{dependency=false})
end,check=function(name,phase,token,executed)
 assert(name=="target")
 if phase=="before" then return proof end
 if phase=="after" then assert(token==proof and type(executed)=="boolean") end
end}
wraith.modules.provider(function(name) if name=="target" then return descriptor end end)
package.preload.target=function()
 runs=(runs or 0)+1
 wraith.commands.create("source",function() wraith.notify("source") end)
 return {ready=true}
end
package.preload.provider=function() return {
 setup=function()
  local child=coroutine.create(function() assert(require("target").ready) end)
  local ok,err=coroutine.resume(child); assert(ok,err)
 end,
 teardown=function() assert(require("target").ready) end,
} end
package.preload.consumer=function() return {setup=function() assert(captured().ready) end} end
local caller=coroutine.create(function() wraith.plugins.load("consumer") end)
local ok,err=coroutine.resume(caller); assert(ok,err)
assert(runs==1)
assert(not pcall(wraith.plugins.unload,"provider"))
`)
	manifest := r.Manifest()
	if len(manifest.Commands) != 1 || manifest.Commands[0].Name != "source" {
		t.Fatal("startup did not publish the lazily loaded source")
	}
	pending, err := r.Submit(manifest.Commands[0].Callback, "", nil, manifest.Config)
	if err != nil {
		t.Fatal(err)
	}
	result := receiveResult(t, pending)
	if result.Err != nil || len(result.Effects) != 1 || !strings.HasPrefix(result.Effects[0].Owner, "plugin:provider:") {
		t.Fatalf("source callback lost provider ownership: %+v", result)
	}
	r.Close()
	select {
	case <-r.Done():
	case <-time.After(3 * time.Second):
		t.Fatal("module cleanup did not finish")
	}
	if diagnostics := r.Diagnostics(); len(diagnostics) != 0 {
		t.Fatalf("closing cached import: %v", diagnostics)
	}
}

// TestPublicModuleProviderStartupFailureDisposesLifecycle keeps failed registrations out of a fresh runtime.
func TestPublicModuleProviderStartupFailureDisposesLifecycle(t *testing.T) {
	directory := t.TempDir()
	source := `
local marker=wraith.paths.stdpath("config").."/cleaned"
local descriptor={plugin="provider",check=function() end,
activate=function() wraith.plugins.load("provider",{},{dependency=false}) end}
wraith.modules.provider(function(name) if name=="target" then return descriptor end end)
package.preload.provider=function() return {
setup=function() wraith.commands.create("discarded",function() end) end,
teardown=function() local file=assert(io.open(marker,"w")); file:write("cleaned"); file:close() end} end
package.preload.target=function() error("startup source failed") end
require("target")
`
	path := filepath.Join(directory, "init.lua")
	if err := os.WriteFile(path, []byte(source), 0600); err != nil {
		t.Fatal(err)
	}
	r, err := Open(t.Context(), directory, config.Default(), nil)
	if r != nil {
		r.Close()
	}
	if err == nil || !strings.Contains(err.Error(), "startup source failed") {
		t.Fatalf("startup failure: %v", err)
	}
	data, err := os.ReadFile(filepath.Join(directory, "cleaned"))
	if err != nil || string(data) != "cleaned" {
		t.Fatalf("failed startup did not dispose its lifecycle: %q, %v", data, err)
	}
	if err := os.WriteFile(path, []byte(`assert(package.loaded.target==nil); package.preload.target=function() return 42 end; assert(require("target")==42)`), 0600); err != nil {
		t.Fatal(err)
	}
	r, err = Open(t.Context(), directory, config.Default(), nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(r.Close)
	if len(r.Manifest().Commands) != 0 {
		t.Fatal("fresh runtime inherited failed registrations")
	}
}

// TestPublicModuleProviderPublication restores removed resolvers when the host rejects their mutation.
func TestPublicModuleProviderPublication(t *testing.T) {
	for _, accepted := range []bool{false, true} {
		t.Run(map[bool]string{false: "rejected", true: "accepted"}[accepted], func(t *testing.T) {
			r := openTestRuntime(t, `
local descriptor={plugin="provider",check=function() end,
activate=function() wraith.plugins.load("provider",{},{dependency=false}) end}
local id=wraith.modules.provider(function(name) if name=="target" then return descriptor end end)
package.preload.provider=function() return {setup=function() end} end
package.preload.target=function() runs=(runs or 0)+1; return {} end
local original=require("target")
wraith.commands.create("remove",function() wraith.modules.remove_provider(id) end)
wraith.commands.create("probe",function(ctx)
 local ok,value=pcall(require,"target")
 if ctx.accepted then
  assert(not ok and tostring(value):find("no resolver",1,true))
  assert(not pcall(wraith.modules.remove_provider,id))
 else assert(ok and value==original) end
 assert(runs==1)
end)
`)
			manifest := r.Manifest()
			commands := map[string]string{}
			for _, command := range manifest.Commands {
				commands[command.Name] = command.Callback
			}
			pending, err := r.Submit(commands["remove"], "", nil, manifest.Config)
			if err != nil {
				t.Fatal(err)
			}
			change := receiveResult(t, pending)
			if change.Err != nil || change.Manifest == nil {
				t.Fatalf("resolver removal did not publish: %+v", change)
			}
			if err := r.Acknowledge(change.Revision, accepted); err != nil {
				t.Fatal(err)
			}
			pending, err = r.Submit(commands["probe"], "", map[string]any{"accepted": accepted}, manifest.Config)
			if err != nil {
				t.Fatal(err)
			}
			result := receiveResult(t, pending)
			if result.Err != nil {
				t.Fatalf("resolver state after publication: %+v", result)
			}
			if result.Manifest != nil {
				if err := r.Acknowledge(result.Revision, true); err != nil {
					t.Fatal(err)
				}
			}
		})
	}
}

// TestPublicModuleProviderCapturedFunctionsKeepGuards rejects registration through saved api references.
func TestPublicModuleProviderCapturedFunctionsKeepGuards(t *testing.T) {
	for _, scope := range []string{"metadata", "pure", "closing"} {
		t.Run(scope, func(t *testing.T) {
			e := moduleProviderFixture(t)
			e.L.SetContext(t.Context())
			if err := e.L.DoString(`local resolve=function() end
saved_add=wraith.modules.provider; saved_remove=wraith.modules.remove_provider
saved_id=saved_add(resolve)
function mutate() assert(not pcall(saved_add,resolve)); assert(not pcall(saved_remove,saved_id)) end`); err != nil {
				t.Fatal(err)
			}
			e.pureHook, e.closing = scope == "pure", scope == "closing"
			if scope == "metadata" {
				e.moduleResolving = []string{"outer"}
			}
			err := e.L.DoString(`mutate()`)
			e.pureHook, e.closing, e.moduleResolving = false, false, nil
			if err != nil || len(e.moduleProviders) != 1 {
				t.Fatalf("captured registration escaped %s: %v", scope, err)
			}
			if scope == "metadata" && e.moduleViolation == nil || scope == "pure" && e.hookViolation == nil {
				t.Fatal("caught registration lost its sticky violation")
			}
			if _, ok := e.L.GetGlobal("saved_id").(lua.LString); !ok {
				t.Fatal("provider did not return an opaque registration id")
			}
		})
	}
}
