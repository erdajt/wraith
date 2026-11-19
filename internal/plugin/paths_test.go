package plugin

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/erdajt/wraith/internal/config"
)

func TestRuntimePathsPreserveExplicitPathsAndModuleCache(t *testing.T) {
	dir, external, custom := t.TempDir(), t.TempDir(), t.TempDir()
	files := map[string]string{
		filepath.Join(dir, "answer.lua"):      `return "config"`,
		filepath.Join(external, "answer.lua"): `return "external"`,
		filepath.Join(custom, "custom.lua"):   `return "custom"`,
	}
	for name, source := range files {
		if err := os.WriteFile(name, []byte(source), 0600); err != nil {
			t.Fatal(err)
		}
	}
	source := fmt.Sprintf(`
local root = %q
assert(wraith.capabilities.runtimepath and wraith.capabilities.stdpath)
assert(require("answer") == "config")
package.path = package.path .. ";" .. %q
wraith.runtimepath.prepend(root .. "/./")
assert(require("answer") == "config")
package.loaded.answer = nil
assert(require("answer") == "external")
assert(require("custom") == "custom")
local paths = wraith.runtimepath.list()
assert(paths[1] == root and #paths == 2)
paths[1] = "changed"
assert(wraith.runtimepath.list()[1] == root)
wraith.runtimepath.append(root)
assert(wraith.runtimepath.list()[2] == root)
package.loaded.answer = nil
assert(require("answer") == "config")
wraith.runtimepath.remove(root)
assert(#wraith.runtimepath.list() == 1)
assert(not package.path:find(root, 1, true))
package.loaded.custom = nil
assert(require("custom") == "custom")
assert(not pcall(wraith.runtimepath.prepend, "bad;path"))
assert(not pcall(wraith.runtimepath.append, "bad?path"))
assert(not pcall(wraith.runtimepath.prepend, ""))
`, external, filepath.Join(custom, "?.lua"))
	if err := os.WriteFile(filepath.Join(dir, "init.lua"), []byte(source), 0600); err != nil {
		t.Fatal(err)
	}
	r, err := Open(t.Context(), dir, config.Default(), nil)
	if err != nil {
		t.Fatal(err)
	}
	r.Close()
}

func TestStandardPathsUseEffectiveConfigurationAndXDG(t *testing.T) {
	dir, data, cache, state := t.TempDir(), t.TempDir(), t.TempDir(), t.TempDir()
	t.Setenv("XDG_DATA_HOME", data)
	t.Setenv("XDG_CACHE_HOME", cache)
	t.Setenv("XDG_STATE_HOME", state)
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	source := fmt.Sprintf(`
assert(wraith.paths.stdpath("config") == %q)
assert(wraith.paths.stdpath("data") == %q)
assert(wraith.paths.stdpath("cache") == %q)
assert(wraith.paths.stdpath("state") == %q)
assert(not pcall(wraith.paths.stdpath, "unknown"))
`, dir, filepath.Join(data, "wraith"), filepath.Join(cache, "wraith"), filepath.Join(state, "wraith"))
	if err := os.WriteFile(filepath.Join(dir, "init.lua"), []byte(source), 0600); err != nil {
		t.Fatal(err)
	}
	r, err := Open(t.Context(), dir, config.Default(), nil)
	if err != nil {
		t.Fatal(err)
	}
	r.Close()
}

func TestStandardPathsHonorHostOverrides(t *testing.T) {
	dir, data, cache := t.TempDir(), t.TempDir(), t.TempDir()
	t.Setenv("XDG_DATA_HOME", t.TempDir())
	t.Setenv("XDG_CACHE_HOME", t.TempDir())
	source := fmt.Sprintf(`
assert(wraith.paths.stdpath("config") == %q)
assert(wraith.paths.stdpath("data") == %q)
assert(wraith.paths.stdpath("cache") == %q)
`, dir, data, cache)
	if err := os.WriteFile(filepath.Join(dir, "init.lua"), []byte(source), 0600); err != nil {
		t.Fatal(err)
	}
	r, err := Open(t.Context(), dir, config.Default(), nil, config.Paths{Config: t.TempDir(), Data: data, Cache: cache})
	if err != nil {
		t.Fatal(err)
	}
	r.Close()
	if _, err := Open(t.Context(), dir, config.Default(), nil, config.Paths{Data: "bad\x00path"}); err == nil {
		t.Fatal("invalid host path accepted")
	}
}
