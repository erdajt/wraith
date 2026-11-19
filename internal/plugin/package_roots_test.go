package plugin

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/erdajt/wraith/internal/config"
	lua "github.com/yuin/gopher-lua"
)

func TestPackageDiscoveryBoundsUnselectedEntriesAndHonorsCancellation(t *testing.T) {
	root := t.TempDir()
	if err := os.Mkdir(filepath.Join(root, "lua"), 0o700); err != nil {
		t.Fatal(err)
	}
	L := lua.NewState()
	defer L.Close()
	e := &engine{L: L}
	roots := []PackageRoot{{Directory: root, Namespaces: []string{"example"}}}
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	L.SetContext(ctx)
	if err := e.installPackageModules(roots); !errors.Is(err, context.Canceled) {
		t.Fatalf("discovery ignored cancellation: %v", err)
	}

	L.SetContext(t.Context())
	for index := range 8192 {
		path := filepath.Join(root, "lua", fmt.Sprintf("unselected-%d.txt", index))
		if err := os.WriteFile(path, nil, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	if err := e.installPackageModules(roots); err == nil || !strings.Contains(err.Error(), "8192 entries") {
		t.Fatalf("discovery did not bound unselected entries: %v", err)
	}
}

func TestPackageRootsBindNestedImportsAndPreserveRequireCache(t *testing.T) {
	configuration, selected := t.TempDir(), t.TempDir()
	for root, files := range map[string]map[string]string{
		selected: {
			"lua/example/init.lua":  `return {value=require("example.value"), source=debug.getinfo(1,"S").source}`,
			"lua/example/value.lua": `return {selected=true}`,
			"lua/unselected.lua":    `error("outside selected namespace")`,
		},
		configuration: {
			"lua/example.lua":         `error("shadowed entry")`,
			"lua/example/value.lua":   `error("shadowed nested module")`,
			"lua/example/missing.lua": `error("missing module escaped selected package")`,
			"init.lua": fmt.Sprintf(`local module=require("example")
assert(module.value.selected and module==require("example"))
assert(module.source==%q)
assert(not pcall(require,"unselected"))
local ok,err=pcall(require,"example.missing")
assert(not ok and tostring(err):find("absent from its selected package",1,true))
package.preload["example.synthetic"]=function() return "synthetic" end
assert(require("example.synthetic")=="synthetic")
wraith.commands.create("selected",function() wraith.notify(module.source) end)`, "@"+filepath.Join(selected, "lua/example/init.lua")),
		},
	} {
		for path, source := range files {
			path = filepath.Join(root, path)
			if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(path, []byte(source), 0o600); err != nil {
				t.Fatal(err)
			}
		}
	}
	runtime, err := OpenWithOptions(t.Context(), configuration, config.Default(), nil, Options{Packages: []PackageRoot{{Directory: selected, Namespaces: []string{"example"}}}})
	if err != nil {
		t.Fatal(err)
	}
	defer runtime.Close()
	if len(runtime.Manifest().Commands) != 1 {
		t.Fatal("selected package did not initialize")
	}
}

func TestPackageRootsRejectAmbiguousModuleIdentity(t *testing.T) {
	for _, kind := range []string{"overlap", "duplicate", "ambiguous", "relative", "empty", "symlink", "loaded"} {
		t.Run(kind, func(t *testing.T) {
			directory := t.TempDir()
			if err := os.MkdirAll(filepath.Join(directory, "lua/example"), 0o700); err != nil {
				t.Fatal(err)
			}
			path := filepath.Join(directory, "lua/example/init.lua")
			if err := os.WriteFile(path, []byte("return {}"), 0o600); err != nil {
				t.Fatal(err)
			}
			roots := []PackageRoot{{Directory: directory, Namespaces: []string{"example"}}}
			switch kind {
			case "overlap":
				roots[0].Namespaces = append(roots[0].Namespaces, "example.nested")
			case "duplicate":
				roots = append(roots, roots[0])
			case "ambiguous":
				if err := os.WriteFile(filepath.Join(directory, "lua/example.lua"), []byte("return {}"), 0o600); err != nil {
					t.Fatal(err)
				}
			case "relative":
				roots[0].Directory = "relative"
			case "empty":
				roots[0].Namespaces = []string{"missing"}
			case "loaded":
				roots[0].Namespaces = []string{"math"}
				if err := os.WriteFile(filepath.Join(directory, "lua/math.lua"), []byte("return {}"), 0o600); err != nil {
					t.Fatal(err)
				}
			case "symlink":
				if err := os.Symlink(path, filepath.Join(directory, "lua/example/alias.lua")); err != nil {
					t.Fatal(err)
				}
			}
			runtime, err := OpenWithOptions(t.Context(), t.TempDir(), config.Default(), nil, Options{Packages: roots})
			if runtime != nil {
				runtime.Close()
			}
			if err == nil {
				t.Fatal("accepted ambiguous package roots")
			}
		})
	}
}

func TestGenericRuntimeDoesNotInstallOrLoadTheDefaultManager(t *testing.T) {
	directory, data := t.TempDir(), t.TempDir()
	if err := os.WriteFile(filepath.Join(directory, "init.lua"), []byte(`assert(package.preload["wraith.manager"]==nil)
assert(not pcall(require,"shroud"))`), 0o600); err != nil {
		t.Fatal(err)
	}
	runtime, err := Open(t.Context(), directory, config.Default(), nil, config.Paths{Data: data})
	if err != nil {
		t.Fatal(err)
	}
	defer runtime.Close()
	entries, err := os.ReadDir(data)
	if err != nil || len(entries) != 0 {
		t.Fatalf("generic runtime installed packages: %v, %v", entries, err)
	}
}
