package plugin

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestManagerRequireRollsBackFailedAndRejectedActivation retries real sources through coroutine callers.
func TestManagerRequireRollsBackFailedAndRejectedActivation(t *testing.T) {
	for _, mode := range []string{"source", "setup", "rejected"} {
		t.Run(mode, func(t *testing.T) {
			root := t.TempDir()
			source := `assert(coroutine.running() == expected_thread)
source_runs = source_runs + 1
wraith.commands.create("source-owned", function() end)
if fail_stage == "source" then
	error("source attempt failed")
end

return {
	setup = function()
		assert(coroutine.running() == expected_thread)
		setup_runs = setup_runs + 1
		wraith.commands.create("setup-owned", function() end)
		if fail_stage == "setup" then
			error("setup attempt failed")
		end
	end,
}`
			if err := os.WriteFile(filepath.Join(root, "require_retry.lua"), []byte(source), 0o600); err != nil {
				t.Fatal(err)
			}
			r := openManagerRuntime(t, fmt.Sprintf(`local manager = require("shroud")
source_runs, setup_runs = 0, 0
fail_stage = %q
manager.setup({ { name = "require-retry", main = "require_retry", dir = %q, opts = {}, lazy = true } })

wraith.commands.create("operate", function(ctx)
	if ctx.argument == "inspect" then
		assert(not manager.status().packages[1].loaded)
		assert(package.loaded.require_retry == nil and source_runs == 1)
		fail_stage = nil
		return
	end

	if ctx.argument == "cached" then
		assert(require("require_retry") == accepted and source_runs == 2)
		assert(setup_runs == (%q == "source" and 1 or 2))
		return
	end

	local thread = coroutine.create(function()
		expected_thread = coroutine.running()
		local value = require("require_retry")
		assert(package.loaded.require_retry == value and coroutine.running() == expected_thread)
		return value
	end)
	local ok, value = coroutine.resume(thread)
	assert(ok, value)
	accepted = value
end, "operate", { arguments = true })`, mode, root, mode))
			var callback string
			for _, command := range r.Manifest().Commands {
				if command.Name == "operate" {
					callback = command.Callback
				}
			}
			if callback == "" {
				t.Fatal("operate command missing")
			}
			submit := func(argument string) Result {
				t.Helper()
				pending, err := r.Submit(callback, "", map[string]any{"argument": argument}, r.Manifest().Config)
				if err != nil {
					t.Fatal(err)
				}
				return receiveResult(t, pending)
			}

			first := submit("load")
			if mode == "rejected" {
				if first.Err != nil || first.Manifest == nil {
					t.Fatalf("activation did not reach publication: %+v", first)
				}
				if err := r.Acknowledge(first.Revision, false); err != nil {
					t.Fatal(err)
				}
			} else if first.Err == nil || !strings.Contains(first.Err.Error(), mode+" attempt failed") || first.Manifest != nil || len(first.Effects) != 0 {
				t.Fatalf("failed %s published activation: %+v", mode, first)
			}
			for _, command := range r.Manifest().Commands {
				if command.Name == "source-owned" || command.Name == "setup-owned" {
					t.Fatalf("rollback retained %s", command.Name)
				}
			}
			if result := submit("inspect"); result.Err != nil || result.Manifest != nil || len(result.Effects) != 0 {
				t.Fatalf("rollback state: %+v", result)
			}

			retry := submit("load")
			if retry.Err != nil || retry.Manifest == nil {
				t.Fatalf("clean retry failed: %+v", retry)
			}
			if err := r.Acknowledge(retry.Revision, true); err != nil {
				t.Fatal(err)
			}
			owned := 0
			for _, command := range r.Manifest().Commands {
				if command.Name == "source-owned" || command.Name == "setup-owned" {
					owned++
				}
			}
			if owned != 2 {
				t.Fatalf("retry published %d owned commands", owned)
			}
			if result := submit("cached"); result.Err != nil || result.Manifest != nil || len(result.Effects) != 0 {
				t.Fatalf("cached require repeated activation: %+v", result)
			}
		})
	}
}

// TestManagerRequireRetirementPreservesDynamicDependencies retires real exports only after publication.
func TestManagerRequireRetirementPreservesDynamicDependencies(t *testing.T) {
	consumer, dependency := t.TempDir(), t.TempDir()
	if err := os.Mkdir(filepath.Join(dependency, "retire_dependency"), 0o700); err != nil {
		t.Fatal(err)
	}
	files := map[string]string{
		filepath.Join(dependency, "retire_dependency", "util.lua"): `dependency_runs = dependency_runs + 1
wraith.commands.create("dependency-owned", function() end)
return { generation = dependency_runs }`,
		filepath.Join(consumer, "retire_consumer.lua"): `consumer_runs = consumer_runs + 1
local dependency = require("retire_dependency.util")
local result = { dependency = dependency }

function result.setup()
	wraith.commands.create("consumer-owned", function() end)
end

function result.teardown()
	assert(require("retire_consumer") == result)
	assert(require("retire_dependency.util") == dependency)
	cleaned = cleaned + 1
end

return result`,
	}
	for path, source := range files {
		if err := os.WriteFile(path, []byte(source), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	r := openManagerRuntime(t, fmt.Sprintf(`local manager = require("shroud")
consumer_runs, dependency_runs, cleaned = 0, 0, 0
manager.setup({
	{ name = "retire-consumer", main = "retire_consumer", dir = %q, opts = {}, lazy = true },
	{ name = "retire-dependency", dir = %q, lazy = true },
})

local original
wraith.commands.create("operate", function(ctx)
	local action = ctx.argument
	if action == "load" then
		original = require("retire_consumer")
		return
	end

	if action == "guard" then
		local ok, err = pcall(manager.unload, "retire-dependency")
		assert(not ok and tostring(err):find("required by", 1, true), tostring(err))
		for _, row in ipairs(manager.status().packages) do
			assert(row.loaded)
		end
		assert(require("retire_consumer") == original and consumer_runs == 1 and dependency_runs == 1 and cleaned == 0)
	elseif action == "unload" then
		manager.unload_all()
	elseif action == "retired" then
		assert(
			cleaned == 1 and package.loaded.retire_consumer == nil and package.loaded["retire_dependency.util"] == nil
		)
		for _, row in ipairs(manager.status().packages) do
			assert(not row.loaded and not row.managed)
		end
	elseif action == "reload" then
		local current = require("retire_consumer")
		assert(current ~= original and current.dependency ~= original.dependency)
		assert(consumer_runs == 2 and dependency_runs == 2 and cleaned == 1)

		local ok, err = pcall(manager.unload, "retire-dependency")
		assert(not ok and tostring(err):find("required by", 1, true), tostring(err))
	end
end, "operate", { arguments = true })`, consumer, dependency))
	var callback string
	for _, command := range r.Manifest().Commands {
		if command.Name == "operate" {
			callback = command.Callback
		}
	}
	if callback == "" {
		t.Fatal("operate command missing")
	}
	submit := func(argument string) Result {
		t.Helper()
		pending, err := r.Submit(callback, "", map[string]any{"argument": argument}, r.Manifest().Config)
		if err != nil {
			t.Fatal(err)
		}
		result := receiveResult(t, pending)
		if result.Err != nil || len(result.Effects) != 0 {
			t.Fatalf("%s: %+v", argument, result)
		}
		return result
	}

	for _, step := range []struct {
		action  string
		publish bool
		accept  bool
	}{
		{"load", true, true},
		{"guard", false, false},
		{"unload", true, false},
		{"guard", false, false},
		{"unload", true, true},
		{"retired", false, false},
		{"reload", true, true},
	} {
		result := submit(step.action)
		if (result.Manifest != nil) != step.publish {
			t.Fatalf("%s publication: %+v", step.action, result)
		}
		if step.publish {
			if err := r.Acknowledge(result.Revision, step.accept); err != nil {
				t.Fatal(err)
			}
		}
		owned := 0
		for _, command := range r.Manifest().Commands {
			if command.Name == "consumer-owned" || command.Name == "dependency-owned" {
				owned++
			}
		}
		want := 2
		if step.action == "retired" || step.action == "unload" && step.accept {
			want = 0
		}
		if owned != want {
			t.Fatalf("%s retained %d owned commands, want %d", step.action, owned, want)
		}
	}
}
