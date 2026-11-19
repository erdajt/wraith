package plugin

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/erdajt/wraith/internal/config"
)

// TestManagerModuleSelectionPreservesConfigPrecedence exercises selection through real metadata callbacks.
func TestManagerModuleSelectionPreservesConfigPrecedence(t *testing.T) {
	for _, test := range []struct {
		name     string
		file     string
		later    string
		link     string
		selected bool
		failure  string
	}{
		{name: "lua-file", file: "lua/probe/child.lua"},
		{name: "lua-init", file: "lua/probe/child/init.lua"},
		{name: "file-before-init", file: "lua/probe/child.lua", later: "lua/probe/child/init.lua"},
		{name: "lua-before-legacy", file: "lua/probe/child/init.lua", later: "probe/child.lua"},
		{name: "legacy-file", file: "probe/child.lua"},
		{name: "legacy-init", file: "probe/child/init.lua"},
		{name: "external-symlink", link: "external"},
		{name: "missing-parents", selected: true},
		{name: "dangling-symlink", link: "missing", selected: true},
		{name: "module-ancestor-file", file: "lua/probe", failure: "not a directory"},
		{name: "lua-ancestor-file", file: "lua", failure: "not a directory"},
		{name: "symlink-loop", link: "child.lua", failure: "too many levels"},
	} {
		t.Run(test.name, func(t *testing.T) {
			directory, managed := t.TempDir(), t.TempDir()
			files := map[string]string{
				filepath.Join(managed, "lua/probe/child.lua"): `error("managed source ran during selection")`,
			}
			if test.file != "" {
				files[filepath.Join(directory, test.file)] = `return "config"`
			}
			if test.later != "" {
				files[filepath.Join(directory, test.later)] = `error("later config source ran")`
			}
			var external string
			if test.link == "external" {
				external = filepath.Join(t.TempDir(), "external.lua")
				files[external] = `return "config"`
			}
			for path, content := range files {
				if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
					t.Fatal(err)
				}
			}
			if test.link != "" {
				path := filepath.Join(directory, "lua/probe/child.lua")
				if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
					t.Fatal(err)
				}
				target := test.link
				if target == "external" {
					target = external
				}
				if err := os.Symlink(target, path); err != nil {
					t.Fatal(err)
				}
			}

			source := fmt.Sprintf(`
local graph = require("wraith.manager.spec").normalize({{name="selection",dir=%q,lazy=true}})
local root = graph.by_name.selection.source.dir
local choose = require("wraith.manager.loader_modules").new(graph,
  {selection=%q}, {}, {selection={activation="selection.active"}}, wraith.paths.stdpath("config"))
local seen, observed = 0, nil
wraith.modules.provider(function(name)
  if name == "probe.child" then
    seen = seen + 1
    local selected = choose(name)
    observed = selected ~= nil
    if selected then
      assert(selected.name == "selection" and selected.path == root)
      assert(selected.entry.name == name and selected.entry.path == root .. "/lua/probe/child.lua")
    end
  end
  return nil
end)
table.insert(package.loaders, function(name)
  assert(name == "probe.child")
  return function() return "native fallback" end
end)
local ok, value = pcall(require, "probe.child")
local expected_failure = %q
assert(seen == 1)
if expected_failure ~= "" then
  assert(not ok and tostring(value):find(expected_failure, 1, true), tostring(value))
else
  assert(ok, value)
  assert(value == (%t and "native fallback" or "config"), tostring(value))
end
if expected_failure == "too many levels" then assert(observed == nil)
else assert(observed == %t, "wrong module precedence") end
assert(package.preload["probe.child"] == nil and #wraith.plugins.list() == 0)
`, managed, managed, test.failure, test.selected, test.selected)
			if err := os.WriteFile(filepath.Join(directory, "init.lua"), []byte(source), 0o600); err != nil {
				t.Fatal(err)
			}
			runtime, err := OpenWithOptions(t.Context(), directory, config.Default(), nil, managerOptions(t))
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(runtime.Close)
		})
	}
}
