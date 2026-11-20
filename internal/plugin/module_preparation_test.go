package plugin

import (
	"fmt"
	"testing"

	lua "github.com/yuin/gopher-lua"
)

// TestModulePreparationReentersThroughItsInitializer keeps one activation and the real caller owner.
func TestModulePreparationReentersThroughItsInitializer(t *testing.T) {
	for _, imported := range []string{"target", "extra"} {
		t.Run(imported, func(t *testing.T) {
			e, _ := moduleRequireFixture(t)
			main := e.L
			var caller *lua.LState
			main.SetGlobal("audit", main.NewFunction(func(L *lua.LState) int {
				if caller == nil {
					caller = L
				}
				phase := L.CheckString(1)
				owner := "resolver"
				if phase != "prepare" {
					owner = e.packages["provider"].owner
				}
				if L == main || L != caller || e.L != L || e.owner != owner {
					t.Fatalf("%s lost caller or owner", phase)
				}
				return 0
			}))
			if err := main.DoString(fmt.Sprintf(`local descriptor = {
 plugin = "provider", check = function() end,
 prepare = function()
  preparations = (preparations or 0) + 1
  audit("prepare")
  wraith.commands.create("prepared", function() end)
  wraith.plugins.load("initializer", {}, {dependency=false})
 end,
 activate = function()
  activations = (activations or 0) + 1
  wraith.plugins.load("provider", {}, {dependency=false})
 end,
}
descriptors.target, descriptors.extra = descriptor, descriptor
package.preload.initializer = function() return {setup=function() assert(require(%q)) end} end
package.preload.provider = function() return {setup=function() audit("setup") end} end
package.preload.target = function()
 audit("source")
 wraith.commands.create("source", function() end)
 runs = (runs or 0) + 1
 return {}
end
package.preload.extra = function() audit("extra"); return true end
local thread = coroutine.create(function()
 local value = require("target")
 assert(require("target") == value and require("extra"))
end)
local ok, err = coroutine.resume(thread)
assert(ok, err)
assert(preparations == 1 and activations == 1 and runs == 1)
`, imported)); err != nil {
				t.Fatal(err)
			}

			owner := e.packages["provider"].owner
			dependencies := e.packages["initializer"].dependencies
			if len(dependencies) != 1 || dependencies[0] != owner {
				t.Fatal("initializer lost its actual provider dependency")
			}
			for _, command := range e.manifest.Commands {
				want := owner
				if command.Name == "prepared" {
					want = "resolver"
				}
				if e.callbackOwners[command.Callback] != want {
					t.Fatalf("%s has the wrong owner", command.Name)
				}
			}
			if e.L != main || e.owner != "config" || len(e.modulePreparing) != 0 || len(e.moduleActivating) != 0 || len(e.moduleRequests) != 0 || len(e.packageCalls) != 0 {
				t.Fatal("preparation leaked execution frames")
			}
		})
	}
}

// TestModulePreparationDescriptorRejectsInvalidValues keeps the optional callback detached and strict.
func TestModulePreparationDescriptorRejectsInvalidValues(t *testing.T) {
	e := moduleResolutionFixture(t)
	for _, value := range []string{"nil", "false", "true", "1", `"prepare"`, "{}", "function() end"} {
		t.Run(value, func(t *testing.T) {
			if err := e.L.DoString(`descriptor = {plugin="provider", activate=function() end, check=function() end, prepare=` + value + `}`); err != nil {
				t.Fatal(err)
			}
			resolution, err := decodeModuleResolution(e.L, e.L.GetGlobal("descriptor"), moduleProvider{})
			valid := value == "nil" || value == "function() end"
			if (err == nil) != valid {
				t.Fatalf("prepare %s: %v", value, err)
			}
			if valid {
				captured := resolution.prepare
				e.L.GetGlobal("descriptor").(*lua.LTable).RawSetString("prepare", lua.LFalse)
				if resolution.prepare != captured || (captured == nil) != (value == "nil") {
					t.Fatal("descriptor mutation redirected preparation")
				}
			}
		})
	}
}
