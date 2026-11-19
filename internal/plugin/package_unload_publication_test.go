package plugin

import (
	"strings"
	"testing"
)

// TestGroupedUnloadPublication keeps verified exports readable until accepted cleanup finishes.
func TestGroupedUnloadPublication(t *testing.T) {
	for _, test := range []struct {
		name     string
		accepted bool
		foreign  bool
		failure  bool
	}{
		{name: "rejected"},
		{name: "accepted", accepted: true},
		{name: "foreign-cache", accepted: true, foreign: true},
		{name: "cleanup-error", accepted: true, failure: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			runtime := openTestRuntime(t, `
local cleaned, runs, original, registrations = {}, {}, {}, {}
local foreign, changed, fail_cleanup = {}, false, false
for _, name in ipairs({"alpha", "beta"}) do
  local module = name .. "_exports"
  package.preload[name] = function() return {
    setup = function()
      registrations[name] = wraith.modules.provider(function() return nil end)
    end,
    teardown = function()
      if name == "beta" and changed then
        assert(not pcall(require, module))
      else
        assert(require(module) == original[name])
      end
      if name == "alpha" and not changed then assert(require("beta_exports") == original.beta) end
      cleaned[#cleaned + 1] = name
      if name == "alpha" and fail_cleanup then error("alpha cleanup failed") end
    end,
  } end
  package.preload[module] = function()
    runs[name] = (runs[name] or 0) + 1
    if name == "alpha" then require("beta_exports") end
    wraith.commands.create("export-" .. name, function() end)
    return {}
  end
  wraith.modules.provider(function(requested)
    if requested == module then return {
      plugin = name, check = function() end,
      activate = function() wraith.plugins.load(name, {}, {dependency=false}) end,
    } end
  end)
end
original.alpha, original.beta = require("alpha_exports"), require("beta_exports")
wraith.commands.create("remove", function(ctx)
  changed, fail_cleanup = ctx.foreign, ctx.failure
  if changed then package.loaded.beta_exports = foreign end
  wraith.plugins.unload_many({"beta", "alpha"})
  assert(#cleaned == 0 and package.loaded.alpha_exports == original.alpha)
  assert(package.loaded.beta_exports == (changed and foreign or original.beta))
end)
wraith.commands.create("probe", function(ctx)
  assert(runs.alpha == 1 and runs.beta == 1)
  if ctx.accepted then
    assert(table.concat(cleaned, ",") == "alpha,beta")
    assert(#wraith.plugins.list() == 0 and package.loaded.alpha_exports == nil)
    if changed then assert(package.loaded.beta_exports == foreign)
    else assert(package.loaded.beta_exports == nil) end
    for _, id in pairs(registrations) do assert(not pcall(wraith.modules.remove_provider, id)) end
  else
    assert(#cleaned == 0 and #wraith.plugins.list() == 2)
    assert(require("alpha_exports") == original.alpha and require("beta_exports") == original.beta)
    assert(not pcall(wraith.plugins.unload, "beta"))
  end
end)
`)
			manifest := runtime.Manifest()
			commands := map[string]string{}
			for _, command := range manifest.Commands {
				commands[command.Name] = command.Callback
			}
			pending, err := runtime.Submit(commands["remove"], "", map[string]any{
				"foreign": test.foreign, "failure": test.failure,
			}, manifest.Config)
			if err != nil {
				t.Fatal(err)
			}
			change := receiveResult(t, pending)
			if change.Err != nil || change.Manifest == nil {
				t.Fatalf("group unload did not stage: %+v", change)
			}
			if err := runtime.Acknowledge(change.Revision, test.accepted); err != nil {
				t.Fatal(err)
			}

			pending, err = runtime.Submit(commands["probe"], "", map[string]any{"accepted": test.accepted}, manifest.Config)
			if err != nil {
				t.Fatal(err)
			}
			result := receiveResult(t, pending)
			if result.Err != nil {
				t.Fatalf("state after group publication: %v", result.Err)
			}
			if result.Manifest != nil {
				if err := runtime.Acknowledge(result.Revision, true); err != nil {
					t.Fatal(err)
				}
			}
			for _, command := range runtime.Manifest().Commands {
				if strings.HasPrefix(command.Name, "export-") && test.accepted {
					t.Fatal("accepted unload retained source registrations")
				}
			}
			if !test.accepted && len(runtime.Manifest().Commands) != 4 {
				t.Fatal("rejected unload lost source registrations")
			}
			diagnostics := strings.Join(runtime.Diagnostics(), "\n")
			if strings.Contains(diagnostics, "alpha cleanup failed") != test.failure {
				t.Fatalf("cleanup diagnostics: %s", diagnostics)
			}
		})
	}
}
