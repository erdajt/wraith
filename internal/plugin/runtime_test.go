package plugin

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/erdajt/wraith/internal/config"
)

func TestRuntimeLoadsLocalModulesAndCloses(t *testing.T) {
	dir := t.TempDir()
	if err := os.Mkdir(filepath.Join(dir, "plugins"), 0700); err != nil {
		t.Fatal(err)
	}
	for name, source := range map[string]string{"init.lua": `assert(require("plugins.answer") == 42); assert(io and os and debug and dofile and loadfile and print)`, "plugins/answer.lua": `return 42`} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(source), 0600); err != nil {
			t.Fatal(err)
		}
	}
	r, err := Open(t.Context(), dir, config.Default(), nil)
	if err != nil {
		t.Fatal(err)
	}
	r.Close()
	select {
	case <-r.Done():
	case <-time.After(time.Second):
		t.Fatal("runtime did not shut down")
	}
}

func TestRuntimeRejectsSyntax(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "init.lua"), []byte(`function broken(`), 0600); err != nil {
		t.Fatal(err)
	}
	if r, err := Open(t.Context(), dir, config.Default(), nil); err == nil {
		r.Close()
		t.Fatal("invalid candidate accepted")
	}
}

func TestTrustedRuntimeReadsExternalFilesAndModules(t *testing.T) {
	dir, external := t.TempDir(), t.TempDir()
	module := filepath.Join(external, "answer.lua")
	if err := os.WriteFile(module, []byte(`return 42`), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(module, filepath.Join(dir, "linked.lua")); err != nil {
		t.Fatal(err)
	}
	source := fmt.Sprintf(`
assert(wraith.api_version == 2 and wraith.capabilities.trusted_lua)
assert(require("linked") == 42)
assert(dofile(%q) == 42)
assert(assert(loadfile(%q))() == 42)
local f = assert(io.open(%q, "r"))
assert(f:read("*a") == "return 42")
f:close()
assert(type(os.execute) == "function" and type(debug.traceback) == "function")
package.preload.answer = function() return 7 end
assert(require("answer") == 7)
print("loaded", 42)
wraith.commands.create("print", function() print("callback", true) end)
`, module, module, module)
	if err := os.WriteFile(filepath.Join(dir, "init.lua"), []byte(source), 0600); err != nil {
		t.Fatal(err)
	}
	r, err := Open(t.Context(), dir, config.Default(), nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(r.Close)
	manifest := r.Manifest()
	if len(manifest.Diagnostics) != 1 || manifest.Diagnostics[0] != "loaded\t42" {
		t.Fatalf("startup print: %v", manifest.Diagnostics)
	}
	result, err := r.Submit(manifest.Commands[0].Callback, "", nil, manifest.Config)
	if err != nil {
		t.Fatal(err)
	}
	got := <-result
	if got.Err != nil || len(got.Effects) != 1 || got.Effects[0].Name != "notify" || got.Effects[0].Argument != "callback\ttrue" {
		t.Fatalf("callback print: %+v", got)
	}
}

func TestModuleRootsIgnoreWorkingDirectoryAndEnvironment(t *testing.T) {
	dir, external := t.TempDir(), t.TempDir()
	t.Chdir(external)
	t.Setenv("LUA_PATH", filepath.Join(external, "?.lua"))
	if err := os.Mkdir(filepath.Join(dir, "lua"), 0700); err != nil {
		t.Fatal(err)
	}
	files := map[string]string{
		filepath.Join(external, "untrusted.lua"): `return "working directory module executed"`,
		filepath.Join(dir, "answer.lua"):         `return "legacy"`,
		filepath.Join(dir, "lua", "answer.lua"):  `return "lua"`,
		filepath.Join(dir, "init.lua"):           `assert(require("answer") == "lua"); assert(not pcall(require, "untrusted"))`,
	}
	for name, source := range files {
		if err := os.WriteFile(name, []byte(source), 0600); err != nil {
			t.Fatal(err)
		}
	}
	r, err := Open(t.Context(), dir, config.Default(), nil)
	if err != nil {
		t.Fatal(err)
	}
	r.Close()
}
