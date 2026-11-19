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
	"golang.org/x/sys/unix"
)

func TestModuleCatalogBindsOnlyImmediateRootSources(t *testing.T) {
	e, root := moduleFixture(t, map[string]string{
		"lua/alpha/init.lua":  `return "alpha"`,
		"lua/alpha/deep.lua":  `error("recursive catalog")`,
		"lua/deeper/file.lua": `error("directory without init")`,
		"lua/flat.name.lua":   `return "physical"`,
		"lua/init.lua":        `return "init"`,
		"lua/zeta.lua":        `return ..., nil, "last", nil`,
		"lua/ignored.txt":     "not a module",
		"legacy.lua":          `error("legacy catalog candidate")`,
	})
	e.L.SetGlobal("root", lua.LString(root))
	if err := e.L.DoString(`
assert(wraith.capabilities.module_sources)
local records=wraith.modules.catalog(root)
assert(#records==4)
assert(records[1].name=="alpha" and records[2].name=="flat.name")
assert(records[3].name=="init" and records[4].name=="zeta")
assert(records[2].load()=="physical" and records[3].load()=="init")
assert(wraith.modules.source(root,"flat.name")==nil)
local exact=wraith.modules.source(root,"zeta")
assert(exact.identity==records[4].identity and exact.kind=="file")
exact.name="wrong"; exact.path="/wrong"; exact.identity="wrong"
local function check(...)
 assert(select("#",...)==4 and select(1,...)=="zeta" and select(3,...)=="last")
end
check(exact.load("ignored"))
assert(package.loaded.zeta==nil)
`); err != nil {
		t.Fatal(err)
	}
}

func TestModuleSourcesIgnoreGlobalResolutionAndHonorExactLayouts(t *testing.T) {
	_, selected := moduleFixture(t, map[string]string{"lua/tool.lua": `error("selected namespace")`})
	e, root := moduleFixture(t, map[string]string{
		"lua/tool.lua":             `return "root"`,
		"tool.lua":                 `error("legacy shadow")`,
		"lua/nested/tool/init.lua": `return "nested"`,
		"lua/fallback/ignored.txt": "empty module directory",
		"fallback.lua":             `return "legacy"`,
	}, PackageRoot{Directory: selected, Namespaces: []string{"tool"}})
	e.L.SetGlobal("root", lua.LString(root))
	if err := e.L.DoString(`
package.preload.tool=function() error("global preload") end
package.loaded.tool={stale=true}
package.path="/unrelated/?.lua"
assert(wraith.modules.source(root,"tool").load()=="root")
assert(wraith.modules.catalog(root)[1].load()=="root")
assert(wraith.modules.source(root,"nested.tool").load()=="nested")
assert(wraith.modules.source(root,"fallback").load()=="legacy")
assert(wraith.modules.source(root,"absent")==nil)
assert(package.loaded.tool.stale)
`); err != nil {
		t.Fatal(err)
	}
	alias := filepath.Join(t.TempDir(), "alias")
	if err := os.Symlink(root, alias); err != nil {
		t.Fatal(err)
	}
	if err := e.L.DoString(fmt.Sprintf(`
assert(wraith.modules.source(%q,"tool").identity==wraith.modules.source(root,"tool").identity)
`, alias)); err != nil {
		t.Fatal(err)
	}
}

func TestModuleSourcesRejectAmbiguityAndInvalidRoots(t *testing.T) {
	for _, name := range []string{"tool", "init"} {
		t.Run(name, func(t *testing.T) {
			e, root := moduleFixture(t, map[string]string{
				"lua/" + name + ".lua":      `error("ambiguous file")`,
				"lua/" + name + "/init.lua": `error("ambiguous init")`,
			})
			if err := e.L.DoString(fmt.Sprintf(`
local ok,err=pcall(wraith.modules.catalog,%q)
assert(not ok and tostring(err):find("ambiguous",1,true))
ok,err=pcall(wraith.modules.source,%q,%q)
assert(not ok and tostring(err):find("ambiguous",1,true))
`, root, root, name)); err != nil {
				t.Fatal(err)
			}
		})
	}
	e, root := moduleFixture(t, map[string]string{"file": "not a directory"})
	for _, path := range []string{"", ".", "relative", root + "\x00", root + ";", root + "?", strings.Repeat("/a", 2049), filepath.Join(root, "missing"), filepath.Join(root, "file")} {
		if err := e.L.DoString(fmt.Sprintf(`assert(not pcall(wraith.modules.catalog,%q)); assert(not pcall(wraith.modules.source,%q,"tool"))`, path, path)); err != nil {
			t.Fatal(err)
		}
	}
	if err := e.L.DoString(fmt.Sprintf(`assert(#wraith.modules.catalog(%q)==0); assert(wraith.modules.source(%q,"tool")==nil)`, root, root)); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "lua"), nil, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := e.L.DoString(fmt.Sprintf(`assert(not pcall(wraith.modules.catalog,%q)); assert(not pcall(wraith.modules.source,%q,"tool"))`, root, root)); err != nil {
		t.Fatal(err)
	}
}

func TestModuleCatalogRejectsTruncationAndInvalidEntries(t *testing.T) {
	for _, test := range []struct {
		name, suffix, expected string
		count                  int
	}{{"sources", ".lua", "256", 257}, {"entries", ".txt", "1024", 1025}} {
		t.Run(test.name, func(t *testing.T) {
			files := make(map[string]string, test.count)
			for i := range test.count {
				files[fmt.Sprintf("lua/item%d%s", i, test.suffix)] = "return true"
			}
			e, root := moduleFixture(t, files)
			if err := e.L.DoString(fmt.Sprintf(`local ok,err=pcall(wraith.modules.catalog,%q); assert(not ok and tostring(err):find(%q,1,true))`, root, test.expected)); err != nil {
				t.Fatal(err)
			}
		})
	}
	for _, name := range []string{"bad name", "trailing.", strings.Repeat("x", 129)} {
		t.Run(name, func(t *testing.T) {
			e, root := moduleFixture(t, map[string]string{"lua/" + name + ".lua": "return true"})
			if err := e.L.DoString(fmt.Sprintf(`assert(not pcall(wraith.modules.catalog,%q)); assert(not pcall(wraith.modules.source,%q,%q))`, root, root, name)); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestModuleSourcesRejectChildSymlinksAndSpecialFiles(t *testing.T) {
	for _, kind := range []string{"file symlink", "directory symlink", "fifo", "directory source"} {
		t.Run(kind, func(t *testing.T) {
			e, root := moduleFixture(t, map[string]string{"lua/ignored.txt": "placeholder"})
			_, outside := moduleFixture(t, map[string]string{"init.lua": `error("outside")`})
			path := filepath.Join(root, "lua", "tool.lua")
			var err error
			switch kind {
			case "file symlink":
				err = os.Symlink(filepath.Join(outside, "init.lua"), path)
			case "directory symlink":
				err = os.Symlink(outside, filepath.Join(root, "lua", "tool"))
			case "fifo":
				err = unix.Mkfifo(path, 0o600)
			case "directory source":
				err = os.Mkdir(path, 0o700)
			}
			if err != nil {
				t.Fatal(err)
			}
			catalog := fmt.Sprintf(`assert(not pcall(wraith.modules.catalog,%q))`, root)
			if kind == "directory source" {
				catalog = fmt.Sprintf(`assert(#wraith.modules.catalog(%q)==0)`, root)
			}
			if err := e.L.DoString(catalog + fmt.Sprintf(`; assert(not pcall(wraith.modules.source,%q,"tool"))`, root)); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestModuleSourceLoadHonorsGuardsAndChangedFiles(t *testing.T) {
	e, root := moduleFixture(t, map[string]string{"lua/tool.lua": `executed=true; return "tool"`})
	if err := e.L.DoString(fmt.Sprintf(`source=wraith.modules.source(%q,"tool"); assert(executed==nil)`, root)); err != nil {
		t.Fatal(err)
	}
	e.pureHook = true
	if err := e.L.DoString(`assert(not pcall(source.load)); assert(executed==nil)`); err != nil || e.hookViolation == nil {
		t.Fatalf("captured source escaped pure hook guard: %v", err)
	}
	e.pureHook = false
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	if _, err := catalogModules(ctx, root); !errors.Is(err, context.Canceled) {
		t.Fatalf("catalog ignored cancellation: %v", err)
	}
	if _, err := resolveModuleSource(ctx, root, "tool"); !errors.Is(err, context.Canceled) {
		t.Fatalf("exact lookup ignored cancellation: %v", err)
	}
	e.L.SetContext(ctx)
	if err := e.L.DoString(`source.load()`); err == nil {
		t.Fatal("ignored cancellation")
	}
	e.L.SetContext(t.Context())
	for _, contents := range []string{strings.Repeat(" ", 1<<20+1), "\x1bLua\x00"} {
		if err := os.WriteFile(filepath.Join(root, "lua", "tool.lua"), []byte(contents), 0o600); err != nil {
			t.Fatal(err)
		}
		if err := e.L.DoString(`assert(not pcall(source.load)); assert(executed==nil)`); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.Remove(filepath.Join(root, "lua", "tool.lua")); err != nil {
		t.Fatal(err)
	}
	if err := e.L.DoString(`assert(not pcall(source.load))`); err != nil {
		t.Fatal(err)
	}
}

func TestModuleSourcesAreAvailableDuringConfiguration(t *testing.T) {
	_, selected := moduleFixture(t, map[string]string{"lua/tool.lua": `return "selected"`})
	_, root := moduleFixture(t, map[string]string{
		"init.lua": fmt.Sprintf(`
assert(wraith.capabilities.module_sources)
local source=wraith.modules.source(%q,"tool")
assert(source.load()=="selected")
assert(wraith.modules.catalog(%q)[1].identity==source.identity)
`, selected, selected),
		"lua/tool.lua": `error("config root shadow")`,
	})
	runtime, err := Open(t.Context(), root, config.Default(), nil)
	if err != nil {
		t.Fatal(err)
	}
	runtime.Close()
}
