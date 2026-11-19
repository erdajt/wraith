package plugin

import (
	"fmt"
	"strings"
	"testing"
	"unicode/utf8"

	lua "github.com/yuin/gopher-lua"
)

// TestModuleClaimsReserveLifetimeCapacity keeps nested requests within the remaining source budget.
func TestModuleClaimsReserveLifetimeCapacity(t *testing.T) {
	e := moduleProviderFixture(t)
	for _, name := range []string{"", "../bad", strings.Repeat("m", 129)} {
		if _, err := e.claimModule(name); err == nil || len(e.moduleClaims) != 0 {
			t.Fatal("invalid name consumed module capacity")
		}
	}
	for range 2048 {
		release, err := e.claimModule("reused")
		if err != nil {
			t.Fatal(err)
		}
		release()
		release()
	}
	for index := range 1022 {
		release, err := e.claimModule(fmt.Sprintf("module%d", index))
		if err != nil {
			t.Fatal(err)
		}
		release()
	}
	release, err := e.claimModule("outer")
	if err != nil {
		t.Fatal(err)
	}
	defer release()
	if _, err := e.claimModule("inner"); err == nil || len(e.moduleClaims) != 1024 {
		t.Fatal("nested activation consumed its caller's reserved slot")
	}
	nested, err := e.claimModule("outer")
	if err != nil {
		t.Fatal(err)
	}
	nested()
	if e.moduleClaims["outer"].pending != 1 || e.moduleClaims["reused"].pending != 0 {
		t.Fatal("nested release changed another reservation")
	}
}

// TestModuleClaimPoisonSurvivesRollback retains the first proof failure after pending scopes unwind.
func TestModuleClaimPoisonSurvivesRollback(t *testing.T) {
	e := moduleProviderFixture(t)
	before := e.captureRegistry()
	var releases []func()
	for range 64 {
		release, err := e.claimModule("owned")
		if err != nil {
			t.Fatal(err)
		}
		releases = append(releases, release)
	}
	if _, err := e.claimModule("owned"); err == nil || e.moduleClaims["owned"].pending != 64 {
		t.Fatal("unbounded pending references")
	}
	if err := e.poisonModule("missing", "failure"); err == nil {
		t.Fatal("poison created an unreserved claim")
	}
	if err := e.poisonModule("owned", strings.Repeat("界", 500)); err != nil {
		t.Fatal(err)
	}
	first := e.moduleClaims["owned"].poison
	if len(first) > 1024 || !utf8.ValidString(first) {
		t.Fatal("poison diagnostic exceeded its bound or split utf-8")
	}
	if err := e.poisonModule("owned", "later"); err != nil {
		t.Fatal(err)
	}
	for _, release := range releases {
		release()
		release()
	}
	if err := e.disposeRegistry(e.restoreRegistry(before)); err != nil {
		t.Fatal(err)
	}
	if e.moduleClaims["owned"].pending != 0 || e.moduleClaims["owned"].poison != first {
		t.Fatal("rollback lost poison or retained pending references")
	}
	if _, err := e.claimModule("owned"); err == nil || !strings.Contains(err.Error(), "use /reload") {
		t.Fatal("poisoned module was reusable")
	}
}

// TestModuleCleanupReadsDetachedReceipts retains exports for teardown after their provider is removed.
func TestModuleCleanupReadsDetachedReceipts(t *testing.T) {
	e, cache, owner := moduleExecutionFixture(t)
	pkg := e.L.GetGlobal("package").(*lua.LTable)
	preload := pkg.RawGetString("preload").(*lua.LTable)
	release, err := e.claimModule("owned")
	if err != nil {
		t.Fatal(err)
	}
	release()
	value := e.L.NewTable()
	cache.RawSetString("owned", value)
	if err := e.recordModuleExecution("owned", owner, cache); err != nil {
		t.Fatal(err)
	}
	e.owner = owner
	read := false
	e.addCleanup(e.L.NewFunction(func(L *lua.LState) int {
		got, err := e.closingModuleValue(L, "owned", pkg, cache, preload)
		if err != nil || got != value || len(e.moduleExecutions) != 0 {
			t.Fatalf("detached cleanup read: %v", err)
		}
		read = true
		L.RaiseError("cleanup failure")
		return 0
	}))
	e.owner = "config"
	if err := e.unloadPlugin("provider"); err != nil {
		t.Fatal(err)
	}
	disposal := registryDisposal{hooks: e.disposing, modules: e.retiredModuleExecutions()}
	e.disposing = nil
	if err := e.disposeRegistry(disposal); err == nil || !strings.Contains(err.Error(), "cleanup failure") {
		t.Fatalf("cleanup outcome: %v", err)
	}
	if !read || len(e.moduleDisposals) != 0 || cache.RawGetString("owned") != lua.LNil {
		t.Fatal("cleanup retained its receipt frame or cache")
	}
	e.closing = true
	cache.RawSetString("owned", value)
	if _, err := e.closingModuleValue(e.L, "owned", pkg, cache, preload); err == nil {
		t.Fatal("retired claimed name became an unowned cache hit")
	}
}

// TestModuleCleanupReadsRequireLatestProof rejects false, foreign and poisoned exports without invoking loaders.
func TestModuleCleanupReadsRequireLatestProof(t *testing.T) {
	e, cache, owner := moduleExecutionFixture(t)
	pkg := e.L.GetGlobal("package").(*lua.LTable)
	preload := pkg.RawGetString("preload").(*lua.LTable)
	e.closing = true
	for _, value := range []lua.LValue{lua.LNil, lua.LFalse, lua.LTrue, lua.LString("cached")} {
		cache.RawSetString("plain", value)
		got, err := e.closingModuleValue(e.L, "plain", pkg, cache, preload)
		if lua.LVAsBool(value) && (err != nil || got != value) || !lua.LVAsBool(value) && err == nil {
			t.Fatalf("native cache classification: %v, %v", value, err)
		}
	}
	e.closing = false
	release, err := e.claimModule("owned")
	if err != nil {
		t.Fatal(err)
	}
	release()
	cache.RawSetString("owned", lua.LString("latest"))
	if err := e.recordModuleExecution("owned", owner, cache); err != nil {
		t.Fatal(err)
	}
	older := e.moduleExecutions["owned"]
	older.serial--
	older.value = lua.LString("earlier")
	e.moduleDisposals = [][]moduleExecution{{older}}
	e.closing = true
	cache.RawSetString("owned", older.value)
	if _, err := e.closingModuleValue(e.L, "owned", pkg, cache, preload); err == nil {
		t.Fatal("older disposal receipt bypassed newer proof")
	}
	cache.RawSetString("owned", lua.LString("latest"))
	if got, err := e.closingModuleValue(e.L, "owned", pkg, cache, preload); err != nil || got != lua.LString("latest") {
		t.Fatalf("current receipt not readable: %v", err)
	}
	pkg.RawSetString("loaded", e.L.NewTable())
	if _, err := e.closingModuleValue(e.L, "owned", pkg, cache, preload); err == nil {
		t.Fatal("replacement cache accepted")
	}
	pkg.RawSetString("loaded", cache)
	registry := e.L.Get(lua.RegistryIndex).(*lua.LTable)
	foreign := e.L.NewTable()
	foreign.RawSetString("owned", lua.LString("foreign"))
	registry.RawSetString("_LOADED", foreign)
	if _, err := e.closingModuleValue(e.L, "owned", pkg, cache, preload); err == nil ||
		registry.RawGetString("_LOADED") != foreign || foreign.RawGetString("owned") != lua.LString("foreign") {
		t.Fatal("replacement native cache accepted or modified")
	}
	registry.RawSetString("_LOADED", cache)
	if err := e.poisonModule("owned", "failed proof"); err != nil {
		t.Fatal(err)
	}
	if _, err := e.closingModuleValue(e.L, "owned", pkg, cache, preload); err == nil {
		t.Fatal("poisoned export was readable during cleanup")
	}
}
