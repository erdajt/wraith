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

// moduleFixture creates source files and an isolated discovery host without loading user configuration.
func moduleFixture(t *testing.T, files map[string]string, packages ...PackageRoot) (*engine, string) {
	t.Helper()
	root := t.TempDir()
	for name, source := range files {
		path := filepath.Join(root, name)
		if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(source), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	L := lua.NewState()
	t.Cleanup(L.Close)
	L.SetContext(t.Context())
	e := &engine{L: L, runtimeRoots: []string{root}}
	if err := e.installPackageModules(packages); err != nil {
		t.Fatal(err)
	}
	api := L.NewTable()
	api.RawSetString("capabilities", L.NewTable())
	L.SetGlobal("wraith", api)
	e.installModuleDiscovery(api)
	return e, root
}

func TestModuleDiscoveryOrdersImmediateSourcesAndBindsLoaders(t *testing.T) {
	e, _ := moduleFixture(t, map[string]string{
		"lua/plugins.lua":            `return ..., nil, "last", nil`,
		"lua/plugins/zeta.lua":       `return "zeta"`,
		"lua/plugins/alpha/init.lua": `return "alpha"`,
		"lua/plugins/alpha/deep.lua": `error("recursive discovery")`,
		"lua/plugins/flat.name.lua":  `return "dotted filename"`,
		"lua/plugins/ignored.txt":    `not lua`,
	})
	err := e.L.DoString(`
local sources=wraith.modules.discover("plugins")
assert(#sources==4 and sources[1].name=="plugins" and sources[2].name=="plugins.alpha")
assert(sources[3].name=="plugins.flat.name" and sources[4].name=="plugins.zeta")
assert(sources[1].kind=="file" and sources[1].path)
local identity=sources[1].identity
sources[1].path="/missing"; sources[1].name="wrong"; sources[1].identity="wrong"
local function check(...)
 assert(select("#",...)==4 and select(1,...)=="plugins" and select(3,...)=="last")
end
check(sources[1].load("ignored"))
assert(wraith.modules.discover("plugins")[1].identity==identity)
package.loaded.plugins={stale=true}
check(sources[1].load())
assert(package.loaded.plugins.stale)
package.preload["plugins.zeta"]=function(name) return name,"synthetic",nil end
local overridden=wraith.modules.discover("plugins")[4]
assert(overridden.kind=="preload" and overridden.path==nil)
package.preload["plugins.zeta"]=function() error("replacement escaped binding") end
assert(select("#",overridden.load())==3)
assert(overridden.load()=="plugins.zeta")
assert(wraith.modules.discover("plugins.zeta")[1].identity~=overridden.identity)
`)
	if err != nil {
		t.Fatal(err)
	}
}

func TestModuleDiscoveryKeepsSelectedNamespacesAndPhysicalDepth(t *testing.T) {
	_, selected := moduleFixture(t, map[string]string{
		"lua/selected/init.lua":        `return "selected"`,
		"lua/selected/flat.name.lua":   `return "flat"`,
		"lua/selected/nested/init.lua": `return "nested"`,
		"lua/selected/nested/deep.lua": `error("too deep")`,
	})
	e, _ := moduleFixture(t, map[string]string{
		"lua/selected.lua":         `error("shadowed root")`,
		"lua/selected/missing.lua": `error("escaped namespace")`,
	}, PackageRoot{Directory: selected, Namespaces: []string{"selected"}})
	if err := e.L.DoString(`
local sources=wraith.modules.discover("selected")
assert(#sources==3 and sources[1].load()=="selected")
assert(sources[2].name=="selected.flat.name" and sources[2].load()=="flat")
assert(sources[3].name=="selected.nested" and sources[3].load()=="nested")
assert(wraith.modules.discover("selected.flat.name")[1].load()=="flat")
assert(not pcall(wraith.modules.discover,"selected.missing"))
package.preload["selected.nested"]=function(name) return name end
assert(wraith.modules.discover("selected")[3].kind=="preload")
package.preload["selected.synthetic"]=function() return "explicit" end
assert(wraith.modules.discover("selected.synthetic")[1].load()=="explicit")
`); err != nil {
		t.Fatal(err)
	}
}

func TestModuleDiscoveryUsesFirstBaseAndRejectsAmbiguity(t *testing.T) {
	e, root := moduleFixture(t, map[string]string{
		"lua/plugins/ignored.txt": "empty first base",
		"plugins.lua":             `error("legacy fallback")`,
		"lua/ambiguous.lua":       `error("ambiguous file executed")`,
		"lua/ambiguous/init.lua":  `error("ambiguous init executed")`,
	})
	_, later := moduleFixture(t, map[string]string{"lua/plugins.lua": `error("later root")`})
	e.runtimeRoots = append(e.runtimeRoots, later)
	if sources, err := e.discoverModules(t.Context(), "plugins"); err != nil || len(sources) != 0 {
		t.Fatalf("first directory did not shadow later sources: %v, %v", sources, err)
	}
	if _, err := e.discoverModules(t.Context(), "ambiguous"); err == nil || !strings.Contains(err.Error(), "ambiguous") {
		t.Fatalf("accepted file/init ambiguity: %v", err)
	}
	alias := filepath.Join(t.TempDir(), "linked-root")
	if err := os.Symlink(root, alias); err != nil {
		t.Fatal(err)
	}
	e.runtimeRoots = []string{alias}
	if _, err := e.discoverModules(t.Context(), "plugins"); err != nil {
		t.Fatalf("explicit symlink root rejected: %v", err)
	}
	if err := os.Symlink(later, filepath.Join(root, "lua", "child")); err != nil {
		t.Fatal(err)
	}
	if _, err := e.discoverModules(t.Context(), "child.plugins"); err == nil || !strings.Contains(err.Error(), "symlink") {
		t.Fatalf("child symlink traversal accepted: %v", err)
	}
}

func TestModuleDiscoveryBoundsWorkAndHonorsCancellation(t *testing.T) {
	e, root := moduleFixture(t, map[string]string{"lua/plugins/init.lua": `return {}`})
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	if _, err := e.discoverModules(ctx, "plugins"); !errors.Is(err, context.Canceled) {
		t.Fatalf("ignored cancellation: %v", err)
	}
	for _, name := range []string{"", "a..b", "a/../b", "a.", strings.Repeat("a", 129)} {
		if _, err := e.discoverModules(t.Context(), name); err == nil {
			t.Errorf("accepted module name %q", name)
		}
	}
	for i := range 256 {
		if err := os.WriteFile(filepath.Join(root, "lua/plugins", fmt.Sprintf("child%d.lua", i)), nil, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := e.discoverModules(t.Context(), "plugins"); err == nil || !strings.Contains(err.Error(), "256 sources") {
		t.Fatalf("source limit ignored: %v", err)
	}
	for i := range 768 {
		if err := os.WriteFile(filepath.Join(root, "lua/plugins", fmt.Sprintf("ignored%d.txt", i)), nil, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := e.discoverModules(t.Context(), "plugins"); err == nil || !strings.Contains(err.Error(), "1024 entries") {
		t.Fatalf("entry limit ignored: %v", err)
	}
	e.runtimeRoots = make([]string, 129)
	if _, err := e.discoverModules(t.Context(), "plugins"); err == nil || !strings.Contains(err.Error(), "128 runtime roots") {
		t.Fatalf("root limit ignored: %v", err)
	}
}

func TestModuleDiscoveryLoadRejectsChangedSourcesAndPureHooks(t *testing.T) {
	e, root := moduleFixture(t, map[string]string{"lua/plugin.lua": `executed=true; return ...`})
	if err := e.L.DoString(`captured=wraith.modules.discover("plugin")[1].load`); err != nil {
		t.Fatal(err)
	}
	e.pureHook = true
	if err := e.L.DoString(`assert(not pcall(captured)); assert(executed==nil)`); err != nil || e.hookViolation == nil {
		t.Fatalf("captured loader escaped hook guard: %v, %v", err, e.hookViolation)
	}
	e.pureHook = false
	path := filepath.Join(root, "lua/plugin.lua")
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	if err := unix.Mkfifo(path, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := e.L.DoString(`assert(not pcall(captured))`); err != nil {
		t.Fatal(err)
	}
}

func TestModuleDiscoveryIsAvailableDuringConfiguration(t *testing.T) {
	_, external := moduleFixture(t, map[string]string{
		"lua/plugins.lua": `return "external"`,
		"lua/extra.lua":   `return "extra"`,
	})
	_, root := moduleFixture(t, map[string]string{
		"init.lua": fmt.Sprintf(`
assert(wraith.capabilities.modules)
assert(wraith.modules.discover("plugins")[1].load()=="plugins")
package.path=package.path..";"..%q
assert(#wraith.modules.discover("extra")==0)
wraith.runtimepath.append(%q)
assert(wraith.modules.discover("extra")[1].load()=="extra")
assert(wraith.modules.discover("plugins")[1].load()=="plugins")
wraith.runtimepath.prepend(%q)
assert(wraith.modules.discover("plugins")[1].load()=="external")
wraith.runtimepath.remove(%q)
assert(wraith.modules.discover("plugins")[1].load()=="plugins")
`, filepath.Join(external, "lua/?.lua"), external, external, external),
		"lua/plugins.lua": `return ...`,
	})
	runtime, err := Open(t.Context(), root, config.Default(), nil)
	if err != nil {
		t.Fatal(err)
	}
	runtime.Close()
}

func TestModuleDiscoveryBindsSelectedChildrenFromOutsideNamespace(t *testing.T) {
	_, selected := moduleFixture(t, map[string]string{
		"lua/plugins/tool/init.lua": `return "selected"`,
	})
	e, _ := moduleFixture(t, map[string]string{
		"lua/plugins/tool/init.lua": `error("parent import bypassed selected namespace")`,
	}, PackageRoot{Directory: selected, Namespaces: []string{"plugins.tool"}})
	if err := e.L.DoString(`
local source=wraith.modules.discover("plugins")[1]
assert(source.name=="plugins.tool" and source.load()=="selected")
package.preload["plugins.tool"]=function(name) return "override:"..name end
assert(wraith.modules.discover("plugins")[1].load()=="override:plugins.tool")
`); err != nil {
		t.Fatal(err)
	}
}

func TestModuleDiscoveryRejectsStaleSymlinksOversizeAndMissingSources(t *testing.T) {
	e, root := moduleFixture(t, map[string]string{"lua/plugin.lua": `error("unused")`})
	if err := e.L.DoString(`captured=wraith.modules.discover("plugin")[1].load`); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(root, "lua/plugin.lua")
	if err := os.WriteFile(path, []byte(strings.Repeat(" ", (1<<20)+1)), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := e.L.DoString(`local ok,err=pcall(captured); assert(not ok and tostring(err):find("1 mib",1,true))`); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	if err := e.L.DoString(`assert(not pcall(captured))`); err != nil {
		t.Fatal(err)
	}
	outside := filepath.Join(t.TempDir(), "outside.lua")
	if err := os.WriteFile(outside, []byte(`error("escaped symlink boundary")`), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, path); err != nil {
		t.Fatal(err)
	}
	if err := e.L.DoString(`local ok,err=pcall(captured); assert(not ok and tostring(err):find("symlink",1,true))`); err != nil {
		t.Fatal(err)
	}
}
