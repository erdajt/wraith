package plugin

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

// TestManagerRequireReinitializesOwnImport retries init imports after retirement and rejected publication.
func TestManagerRequireReinitializesOwnImport(t *testing.T) {
	for _, module := range []string{"reentry_fixture", "reentry_fixture.util"} {
		t.Run(module, func(t *testing.T) {
			root := t.TempDir()
			if err := os.MkdirAll(filepath.Join(root, "lua", "reentry_fixture"), 0o700); err != nil {
				t.Fatal(err)
			}
			files := map[string]string{
				"reentry_fixture.lua": `assert(coroutine.running() == expected_thread)
source_runs = source_runs + 1
local generation = source_runs
wraith.commands.create("reentry.owned", function() end)
wraith.events.on("ready", function() wraith.notify("source:" .. generation) end)
return {
 generation = generation,
 setup = function()
  assert(coroutine.running() == expected_thread)
  setup_runs = setup_runs + 1
  wraith.events.on("ready", function() wraith.notify("setup:" .. generation) end)
 end,
}`,
				"reentry_fixture/util.lua": `assert(coroutine.running() == expected_thread)
return { generation = source_runs }`,
			}
			for path, source := range files {
				if err := os.WriteFile(filepath.Join(root, "lua", path), []byte(source), 0o600); err != nil {
					t.Fatal(err)
				}
			}
			r := openManagerRuntime(t, fmt.Sprintf(`local manager = require("shroud")
source_runs, setup_runs, init_runs = 0, 0, 0
local initialized, fail_init
local requested = %q
manager.setup({ {
 name = "reentry-fixture", dir = %q, lazy = true, opts = {},
 init = function()
  assert(coroutine.running() == expected_thread)
  init_runs = init_runs + 1
  local generation = init_runs
  wraith.events.on("ready", function() wraith.notify("init:" .. generation) end)
  initialized = require(requested)
  assert(initialized.generation == init_runs)
  assert(source_runs == init_runs and setup_runs == init_runs)
  if fail_init then
   error("init attempt failed")
  end
 end,
} })
assert(init_runs == 1 and source_runs == 1 and setup_runs == 1)
local original = initialized

wraith.commands.create("operate", function(ctx)
 local action = ctx.argument
 if action == "unload" then
  manager.unload("reentry-fixture")
 elseif action == "retired" then
  assert(package.loaded.reentry_fixture == nil and package.loaded["reentry_fixture.util"] == nil)
  local row = manager.status().packages[1]
  assert(not row.loaded and not row.managed)
 elseif action == "cached" then
  assert(require(requested) == initialized)
  assert(init_runs == 4 and source_runs == 4 and setup_runs == 4)
 else
  fail_init = action == "fail"
  local thread = coroutine.create(function()
   expected_thread = coroutine.running()
   local value = require(requested)
   assert(value == initialized and value ~= original)
   assert(init_runs == source_runs and setup_runs == source_runs)
   local row = manager.status().packages[1]
   assert(row.loaded and row.managed)
  end)
  local ok, err = coroutine.resume(thread)
  expected_thread = nil
  assert(ok, err)
 end
end, "operate", { arguments = true })
`, module, root))
			pending, err := r.Submit("", "ready", nil, r.Manifest().Config)
			if err != nil {
				t.Fatal(err)
			}
			notices := drainManagerEvents(t, r, receiveResult(t, pending))
			var callback string
			for _, command := range r.Manifest().Commands {
				if command.Name == "operate" {
					callback = command.Callback
				}
			}
			if callback == "" {
				t.Fatal("operate command missing")
			}

			for _, action := range []string{"unload", "retired", "fail", "retired", "reject", "retired", "reload", "cached"} {
				pending, err := r.Submit(callback, "", map[string]any{"argument": action}, r.Manifest().Config)
				if err != nil {
					t.Fatal(err)
				}
				result := receiveResult(t, pending)
				if action == "fail" {
					if result.Err == nil || !strings.Contains(result.Err.Error(), "init attempt failed") || result.Manifest != nil || len(result.Effects) != 0 {
						t.Fatalf("failed init published changes: %+v", result)
					}
				} else {
					if result.Err != nil || (result.Manifest != nil) != (action == "unload" || action == "reload" || action == "reject") {
						t.Fatalf("%s publication: %+v", action, result)
					}
					if action == "reject" {
						if err := r.Acknowledge(result.Revision, false); err != nil {
							t.Fatal(err)
						}
					} else {
						notices = append(notices, drainManagerEvents(t, r, result)...)
					}
				}
				owned := false
				for _, command := range r.Manifest().Commands {
					owned = owned || command.Name == "reentry.owned"
				}
				if owned != (action == "reload" || action == "cached") {
					t.Fatalf("%s retained owned registration: %t", action, owned)
				}
			}

			slices.Sort(notices)
			want := []string{"init:1", "init:4", "setup:1", "setup:4", "source:1", "source:4"}
			if !slices.Equal(notices, want) {
				t.Fatalf("readiness escaped rollback or repeated: got %v, want %v", notices, want)
			}
		})
	}
}
