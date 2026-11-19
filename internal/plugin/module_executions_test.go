package plugin

import (
	"context"
	"fmt"
	"strings"
	"testing"

	lua "github.com/yuin/gopher-lua"
)

// moduleExecutionFixture gives receipts a real lifecycle owner and native cache.
func moduleExecutionFixture(t *testing.T) (*engine, *lua.LTable, string) {
	t.Helper()
	e := moduleProviderFixture(t)
	e.L.SetContext(t.Context())
	if err := e.L.DoString(`
package.preload.provider=function() return {setup=function() end} end
wraith.plugins.load("provider")
`); err != nil {
		t.Fatal(err)
	}
	cache := e.L.GetField(e.L.Get(lua.RegistryIndex), "_LOADED").(*lua.LTable)
	return e, cache, e.packages["provider"].owner
}

// TestModuleExecutionReceiptsBoundRepeatedFalsyLoads keeps current results rather than an unbounded journal.
func TestModuleExecutionReceiptsBoundRepeatedFalsyLoads(t *testing.T) {
	e, cache, owner := moduleExecutionFixture(t)
	cache.RawSetString("falsy", lua.LFalse)
	for range 2048 {
		if err := e.recordModuleExecution("falsy", owner, cache); err != nil {
			t.Fatal(err)
		}
	}
	if len(e.moduleExecutions) != 1 {
		t.Fatal("repeated loads grew the execution ledger")
	}
	for index := range 1023 {
		if err := e.recordModuleExecution(fmt.Sprintf("module%d", index), owner, cache); err != nil {
			t.Fatal(err)
		}
	}
	if err := e.recordModuleExecution("overflow", owner, cache); err == nil {
		t.Fatal("accepted an execution beyond the receipt limit")
	}
	serial := e.serial
	if err := e.recordModuleExecution("falsy", owner, cache); err != nil || e.serial <= serial {
		t.Fatalf("existing receipt could not advance at capacity: %v", err)
	}
	if err := e.recordModuleExecution("falsy", "retired", cache); err == nil {
		t.Fatal("retired owner replaced a live receipt")
	}
}

// TestModuleExecutionRollbackClearsAfterCleanup preserves exports while discarded cleanup runs.
func TestModuleExecutionRollbackClearsAfterCleanup(t *testing.T) {
	e, cache, owner := moduleExecutionFixture(t)
	cache.RawSetString("committed", lua.LString("kept"))
	if err := e.recordModuleExecution("committed", owner, cache); err != nil {
		t.Fatal(err)
	}
	before := e.captureRegistry()
	e.owner = owner
	if err := e.L.DoString(`
package.preload.fresh=function()
 wraith.commands.create("from-source",function() end)
 return {ready=true}
end
assert(require("fresh").ready)
function cleanup_source() assert(require("fresh").ready); cleanup_read=true end
`); err != nil {
		t.Fatal(err)
	}
	e.addCleanup(e.L.GetGlobal("cleanup_source").(*lua.LFunction))
	if err := e.recordModuleExecution("fresh", owner, cache); err != nil {
		t.Fatal(err)
	}
	disposal := e.restoreRegistry(before)
	if cache.RawGetString("fresh") == lua.LNil || len(disposal.modules) != 1 || len(e.manifest.Commands) != 0 {
		t.Fatal("rollback cleared exports early or retained source registrations")
	}
	if err := e.disposeRegistry(disposal); err != nil {
		t.Fatal(err)
	}
	if e.L.GetGlobal("cleanup_read") != lua.LTrue || cache.RawGetString("fresh") != lua.LNil || cache.RawGetString("committed") != lua.LString("kept") {
		t.Fatal("cleanup or invalidation crossed the rollback boundary")
	}
}

// TestModuleExecutionNestedRollbackKeepsForeignValues never restores overwritten exports or clears foreign tables.
func TestModuleExecutionNestedRollbackKeepsForeignValues(t *testing.T) {
	e, cache, owner := moduleExecutionFixture(t)
	outer := e.captureRegistry()
	for index := range 2 {
		cache.RawSetString("nested", lua.LNumber(index))
		if err := e.recordModuleExecution("nested", owner, cache); err != nil {
			t.Fatal(err)
		}
		inner := e.captureRegistry()
		cache.RawSetString("nested", lua.LFalse)
		if err := e.recordModuleExecution("nested", owner, cache); err != nil {
			t.Fatal(err)
		}
		disposal := e.restoreRegistry(inner)
		foreign := e.L.NewTable()
		if index == 1 {
			cache.RawSetString("nested", foreign)
		}
		if err := e.disposeRegistry(disposal); err != nil {
			t.Fatal(err)
		}
		want := lua.LValue(lua.LNil)
		if index == 1 {
			want = foreign
		}
		if cache.RawGetString("nested") != want || len(e.moduleExecutions) != 0 {
			t.Fatal("rollback restored an old receipt or erased a foreign value")
		}
	}
	if err := e.disposeRegistry(e.restoreRegistry(outer)); err != nil {
		t.Fatal(err)
	}
}

// TestModuleExecutionRetirementWaitsForAcceptedUnload distinguishes staged removal from a retired generation.
func TestModuleExecutionRetirementWaitsForAcceptedUnload(t *testing.T) {
	e, cache, owner := moduleExecutionFixture(t)
	value := e.L.NewTable()
	cache.RawSetString("owned", value)
	if err := e.recordModuleExecution("owned", owner, cache); err != nil {
		t.Fatal(err)
	}
	before := e.captureRegistry()
	if err := e.unloadPlugin("provider"); err != nil {
		t.Fatal(err)
	}
	if err := e.disposeRegistry(e.restoreRegistry(before)); err != nil {
		t.Fatal(err)
	}
	if cache.RawGetString("owned") != value || len(e.retiredModuleExecutions()) != 0 {
		t.Fatal("rejected unload invalidated a live export")
	}
	if err := e.unloadPlugin("provider"); err != nil {
		t.Fatal(err)
	}
	if cache.RawGetString("owned") != value {
		t.Fatal("staging unload cleared a live export")
	}
	if err := e.disposeRegistry(registryDisposal{modules: e.retiredModuleExecutions()}); err != nil {
		t.Fatal(err)
	}
	if cache.RawGetString("owned") != lua.LNil || len(e.moduleExecutions) != 0 {
		t.Fatal("accepted unload retained retired exports")
	}
}

// TestModuleExecutionDisposalSurvivesCleanupFailure clears repeated false values despite cleanup errors or deadlines.
func TestModuleExecutionDisposalSurvivesCleanupFailure(t *testing.T) {
	for _, tc := range []struct{ name, source, failure string }{
		{"cancelled callback", `cleanup_ran=true`, ""},
		{"cleanup error", `error("cleanup failed")`, "cleanup failed"},
		{"cleanup deadline", `while true do end`, "context deadline exceeded"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			e, cache, owner := moduleExecutionFixture(t)
			cache.RawSetString("falsy", lua.LFalse)
			if err := e.recordModuleExecution("falsy", owner, cache); err != nil {
				t.Fatal(err)
			}
			before := e.captureRegistry()
			if err := e.recordModuleExecution("falsy", owner, cache); err != nil {
				t.Fatal(err)
			}
			cleanup, err := e.L.LoadString(tc.source)
			if err != nil {
				t.Fatal(err)
			}
			e.owner = owner
			e.addCleanup(cleanup)
			ctx, cancel := context.WithCancel(t.Context())
			cancel()
			e.L.SetContext(ctx)

			err = e.disposeRegistry(e.restoreRegistry(before))
			if tc.failure == "" && (err != nil || e.L.GetGlobal("cleanup_ran") != lua.LTrue) ||
				tc.failure != "" && (err == nil || !strings.Contains(err.Error(), tc.failure)) {
				t.Fatalf("cleanup outcome: %v", err)
			}
			if cache.RawGetString("falsy") != lua.LNil || len(e.moduleExecutions) != 0 || e.L.Context() != ctx {
				t.Fatal("cleanup skipped invalidation or retained its temporary context")
			}
		})
	}
}
