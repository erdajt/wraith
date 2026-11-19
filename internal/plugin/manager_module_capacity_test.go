package plugin

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

// TestManagerModuleLedgerCapacity checks lifetime reservations using installed source and real files.
func TestManagerModuleLedgerCapacity(t *testing.T) {
	for _, test := range []struct {
		name     string
		limit    int
		packages bool
	}{
		{name: "packages", limit: 63, packages: true},
		{name: "modules", limit: 1024},
	} {
		t.Run(test.name, func(t *testing.T) {
			directory := t.TempDir()
			for index := 1; index <= test.limit+1; index++ {
				name := fmt.Sprintf("capacity_%04d.lua", index)
				if err := os.WriteFile(filepath.Join(directory, name), []byte("return true"), 0o600); err != nil {
					t.Fatal(err)
				}
			}

			runtime := openManagerRuntime(t, fmt.Sprintf(`
local sources = require("wraith.manager.loader_sources")
local schema = require("wraith.manager.spec")
local root, limit, separate = %q, %d, %t
local specs, bindings = {}, {}
local function spec(index)
  local name = separate and "capacity_" .. index or "capacity"
  if not specs[name] then
    specs[name] = schema.normalize({{name=name,dir=root,lazy=true}}).by_name[name]
  end
  return specs[name]
end
local function prepare(index)
  local name = string.format("capacity_%%04d", index)
  local entry = assert(wraith.modules.source(root, name))
  return sources.prepare_module(spec(index), root, "first", entry)
end
local pending = prepare(limit + 1)
local invalid = assert(wraith.modules.source(root, "capacity_0001"))
invalid.identity = "changed"
assert(not pcall(sources.prepare_module, spec(limit + 1), root, "first", invalid))
assert(sources.lookup(invalid.name) == nil and package.preload[invalid.name] == nil)

wraith.commands.create("reserve", function(ctx)
  local first = assert(tonumber(ctx.argument))
  for index = first, math.min(first + 15, limit) do
    local binding = prepare(index)
    assert(sources.lookup(binding.entry.name) == nil)
    assert(sources.reserve_module(spec(index), root, "first", binding) == binding)
    local owner, canonical = sources.lookup(binding.entry.name)
    assert(owner == spec(index).name and canonical == binding)
    bindings[index] = binding
  end
end, "reserve", {arguments=true})

wraith.commands.create("verify", function()
  local overflow = string.format("capacity_%%04d", limit + 1)
  local expected = separate and "package source identities exceed runtime capacity" or "package module identities exceed runtime capacity"
  local ok, err = pcall(prepare, limit + 1)
  assert(not ok and tostring(err):find(expected, 1, true), tostring(err))
  ok, err = pcall(sources.reserve_module, spec(limit + 1), root, "first", pending)
  assert(not ok and tostring(err):find(expected, 1, true), tostring(err))
  assert(sources.lookup(overflow) == nil and package.preload[overflow] == nil)

  for _, index in ipairs({1, limit}) do
    local binding = prepare(index)
    assert(binding == bindings[index])
    assert(sources.reserve_module(spec(index), root, "first", binding) == binding)
    assert(binding.require() == true)
    local changed, failure = pcall(sources.prepare_module, spec(index), root, "changed", binding.entry)
    assert(not changed and tostring(failure):find("source changed", 1, true), tostring(failure))
    changed, failure = pcall(sources.prepare_module, spec(index), root .. "/other", "first", binding.entry)
    assert(not changed and tostring(failure):find("source changed", 1, true), tostring(failure))
    local owner, canonical = sources.lookup(binding.entry.name)
    assert(owner == spec(index).name and canonical == binding)
  end
end)
`, directory, test.limit, test.packages))
			for first := 1; first <= test.limit; first += 16 {
				managerCommand(t, runtime, "reserve", fmt.Sprint(first))
			}
			managerCommand(t, runtime, "verify", "")
		})
	}
}
