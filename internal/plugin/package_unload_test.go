package plugin

import (
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

// TestGroupedUnloadValidatesTheWholeCohort keeps caught errors from unloading an earlier target.
func TestGroupedUnloadValidatesTheWholeCohort(t *testing.T) {
	openTestRuntime(t, `
 assert(wraith.capabilities.plugins_unload_many)
 package.preload.first=function() return {setup=function()
  wraith.commands.create("first",function() end)
 end} end
 package.preload.second=function() return {setup=function() wraith.plugins.depend("first") end} end
 package.preload.third=function() return {setup=function() end,attempt=function()
  assert(not pcall(wraith.plugins.unload_many,{"first","second","third"}))
 end} end
 wraith.plugins.load("first")
 wraith.plugins.load("second")
 wraith.plugins.load("third")
 local before=table.concat(wraith.plugins.list(),",")
 for _,names in ipairs({{"third","missing"},{"third","first"},{"third","third"},
   {[2]="first"},{first="first"},{"third",false},{"third",1},{[1.5]="third"}}) do
  assert(not pcall(wraith.plugins.unload_many,names))
  assert(table.concat(wraith.plugins.list(),",")==before)
 end
 local many={}
 for index=1,129 do many[index]="third" end
 assert(not pcall(wraith.plugins.unload_many,many))
 wraith.plugins.call("third","attempt")
 assert(table.concat(wraith.plugins.list(),",")==before)
 package.preload.loading=function() return {setup=function()
  assert(not pcall(wraith.plugins.unload_many,{"third","loading"}))
 end} end
 wraith.plugins.load("loading")
 wraith.plugins.unload_many({"first","loading","third","second"})
 assert(#wraith.plugins.list()==0)
 package.preload[""]=function() return {setup=function() end} end
 wraith.plugins.load("")
 wraith.plugins.unload_many({""})
 assert(#wraith.plugins.list()==0)
 `)
}

// TestGroupedUnloadPreservesStartupCleanup stages a hookless dependency chain before disposal.
func TestGroupedUnloadPreservesStartupCleanup(t *testing.T) {
	marker := filepath.Join(t.TempDir(), "cleanup")
	r := openTestRuntime(t, fmt.Sprintf(`
 local function record(value)
  local file=assert(io.open(%q,"a")); file:write(value); file:close()
 end
 wraith.commands.create("shared",function() end)
 package.preload.last=function() return {
  setup=function() wraith.commands.create("shared",function() end,"",{replace=true}) end,
  teardown=function()
   assert(not pcall(wraith.plugins.unload_many,{}))
   record("last\n")
  end,
 } end
 package.preload.middle=function() return {setup=function() wraith.plugins.depend("last") end} end
 package.preload.first=function() return {
  setup=function() wraith.plugins.depend("middle") end,
  teardown=function() record("first\n") end,
 } end
 wraith.plugins.load("last")
 wraith.plugins.load("middle")
 wraith.plugins.load("first")
 wraith.plugins.unload_many({"last","first","middle"})
 assert(#wraith.plugins.list()==0 and io.open(%q,"r")==nil)
 `, marker, marker))
	data, err := os.ReadFile(marker)
	if err != nil || string(data) != "first\nlast\n" {
		t.Fatalf("startup cleanup order: %q, %v", data, err)
	}
	if commands := r.Manifest().Commands; len(commands) != 1 || commands[0].Name != "shared" {
		t.Fatalf("covered command was not restored: %+v", commands)
	}
}

// TestGroupedUnloadPreflightDoesNotMutate checks every private guard before any staging begins.
func TestGroupedUnloadPreflightDoesNotMutate(t *testing.T) {
	for _, kind := range []string{"cycle", "current", "called", "loading", "owner"} {
		t.Run(kind, func(t *testing.T) {
			e := &engine{owner: "config", packages: map[string]packageRecord{
				"first": {owner: "one", dependencies: []string{"two"}},
				"last":  {owner: "two"},
			}}
			switch kind {
			case "cycle":
				e.packages["last"] = packageRecord{owner: "two", dependencies: []string{"one"}}
			case "current":
				e.owner = "two"
			case "called":
				e.packageCalls = []string{"two"}
			case "loading":
				e.packages["last"] = packageRecord{owner: "two", loading: true}
			case "owner":
				e.packages["last"] = packageRecord{owner: "one"}
			}
			before := e.clonePackages()
			if err := e.unloadPlugins([]string{"last", "first"}); err == nil {
				t.Fatal("invalid cohort accepted")
			}
			if !reflect.DeepEqual(e.packages, before) || e.dirty || len(e.disposing) != 0 {
				t.Fatal("failed preflight changed lifecycle state")
			}
		})
	}
	for _, dirty := range []bool{false, true} {
		e := &engine{dirty: dirty}
		if err := e.unloadPlugins(nil); err != nil || e.dirty != dirty {
			t.Fatal("empty group changed publication state")
		}
		e.closing = true
		if err := e.unloadPlugins(nil); err == nil || !strings.Contains(err.Error(), "teardown") || e.dirty != dirty {
			t.Fatal("empty group bypassed teardown guard")
		}
	}
}

// TestGroupedUnloadCapturedGuards also blocks empty operations inside restricted phases.
func TestGroupedUnloadCapturedGuards(t *testing.T) {
	e := moduleResolutionFixture(t)
	if err := e.L.DoString(`captured=wraith.plugins.unload_many`); err != nil {
		t.Fatal(err)
	}
	e.dirty = false
	if err := e.L.DoString(`captured({})`); err != nil || e.dirty {
		t.Fatal("public empty group changed publication state", err)
	}
	e.pureHook = true
	if err := e.L.DoString(`assert(not pcall(captured,{}))`); err != nil || e.hookViolation == nil || e.dirty {
		t.Fatal("captured empty group bypassed pure hook guard", err)
	}
	e.pureHook, e.hookViolation = false, nil
	if err := e.L.DoString(`wraith.modules.provider(function() pcall(captured,{}); return nil end)`); err != nil {
		t.Fatal(err)
	}
	e.dirty = false
	if _, err := e.resolveModuleProvider(e.L, "probe"); err == nil || e.dirty {
		t.Fatal("caught empty group bypassed metadata guard", err)
	}
}

// TestGroupedUnloadAcceptsFullLifecycleCapacity exercises the inclusive cohort bound.
func TestGroupedUnloadAcceptsFullLifecycleCapacity(t *testing.T) {
	openTestRuntime(t, `
 local names={}
 for index=1,128 do
  local name="capacity."..index
  package.preload[name]=function() return {setup=function() end} end
  wraith.plugins.load(name)
  names[#names+1]=name
 end
 wraith.plugins.unload_many(names)
 assert(#wraith.plugins.list()==0)
 `)
}
