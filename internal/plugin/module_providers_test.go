package plugin

import (
	"fmt"
	"slices"
	"testing"

	"github.com/erdajt/wraith/internal/config"
	lua "github.com/yuin/gopher-lua"
)

// moduleProviderFixture loads the actual registry and lifecycle machinery without exposing a resolver API.
func moduleProviderFixture(t *testing.T) *engine {
	t.Helper()
	e, err := load(t.Context(), t.TempDir(), config.Default(), nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(e.L.Close)
	t.Cleanup(e.services.close)
	t.Cleanup(e.surfaces.close)
	t.Cleanup(e.invocations.close)
	return e
}

// TestModuleProvidersKeepBoundedRegistrationOrder preserves identities and precedence across replacement.
func TestModuleProvidersKeepBoundedRegistrationOrder(t *testing.T) {
	e := moduleProviderFixture(t)
	resolver := e.L.NewFunction(func(*lua.LState) int { return 0 })
	if _, err := e.addModuleProvider(nil); err == nil {
		t.Fatal("accepted a missing resolver")
	}
	var ids []string
	for range 32 {
		id, err := e.addModuleProvider(resolver)
		if err != nil || slices.Contains(ids, id) {
			t.Fatalf("registration %q: %v", id, err)
		}
		ids = append(ids, id)
	}
	if _, err := e.addModuleProvider(resolver); err == nil || len(e.moduleProviders) != 32 {
		t.Fatal("provider capacity was not enforced")
	}

	e.owner = "other"
	if err := e.removeModuleProvider(ids[1]); err == nil || e.moduleProviders[1].id != ids[1] {
		t.Fatal("foreign owner removed a provider")
	}
	e.owner = "config"
	if err := e.removeModuleProvider(ids[1]); err != nil {
		t.Fatal(err)
	}
	id, err := e.addModuleProvider(resolver)
	if err != nil || slices.Contains(ids, id) {
		t.Fatalf("replacement reused an identity: %q, %v", id, err)
	}
	want := append(slices.Delete(slices.Clone(ids), 1, 2), id)
	for index, provider := range e.moduleProviders {
		if provider.id != want[index] || provider.owner != "config" || provider.resolve != resolver {
			t.Fatalf("provider order or identity changed: %+v", provider)
		}
	}
	if err := e.removeModuleProvider(ids[1]); err == nil {
		t.Fatal("removed identity was still valid")
	}
}

// TestModuleProviderGuardsPreserveStagedState rejects mutations without changing publication state.
func TestModuleProviderGuardsPreserveStagedState(t *testing.T) {
	e := moduleProviderFixture(t)
	resolver := e.L.NewFunction(func(*lua.LState) int { return 0 })
	id, err := e.addModuleProvider(resolver)
	if err != nil {
		t.Fatal(err)
	}
	before, serial := slices.Clone(e.moduleProviders), e.serial
	for _, dirty := range []bool{false, true} {
		for _, guard := range []string{"teardown", "missing owner"} {
			t.Run(guard+"/dirty="+fmt.Sprint(dirty), func(t *testing.T) {
				e.dirty = dirty
				e.closing, e.owner = guard == "teardown", "config"
				if guard == "missing owner" {
					e.owner = ""
				}
				if _, err := e.addModuleProvider(resolver); err == nil {
					t.Fatal("accepted a guarded registration")
				}
				if err := e.removeModuleProvider(id); err == nil {
					t.Fatal("accepted a guarded removal")
				}
				if e.dirty != dirty || e.serial != serial || !slices.Equal(e.moduleProviders, before) {
					t.Fatal("rejected mutation changed the staged state")
				}
			})
		}
	}
}

// TestModuleProviderRollbackDetachesSnapshotsAndRetiresIDs prevents rollback from reviving abandoned handles.
func TestModuleProviderRollbackDetachesSnapshotsAndRetiresIDs(t *testing.T) {
	e := moduleProviderFixture(t)
	resolver := e.L.NewFunction(func(*lua.LState) int { return 0 })
	original, err := e.addModuleProvider(resolver)
	if err != nil {
		t.Fatal(err)
	}
	before := e.captureRegistry()
	if err := e.removeModuleProvider(original); err != nil {
		t.Fatal(err)
	}
	abandoned, err := e.addModuleProvider(resolver)
	if err != nil {
		t.Fatal(err)
	}
	if before.moduleProviders[0].id != original {
		t.Fatal("staged mutations changed the registry snapshot")
	}
	e.restoreRegistry(before)
	if len(e.moduleProviders) != 1 || e.moduleProviders[0].id != original {
		t.Fatal("rollback did not restore the original provider")
	}
	e.moduleProviders[0].owner = "changed"
	if before.moduleProviders[0].owner != "config" {
		t.Fatal("restored registry shares its snapshot slice")
	}
	e.restoreRegistry(before)
	fresh, err := e.addModuleProvider(resolver)
	if err != nil || fresh == original || fresh == abandoned {
		t.Fatalf("rollback recycled an identity: %q, %v", fresh, err)
	}
}

// TestModuleProvidersFollowExactOwnerUnloadAndRollback restores resolvers when package removal is rejected.
func TestModuleProvidersFollowExactOwnerUnloadAndRollback(t *testing.T) {
	e := moduleProviderFixture(t)
	resolver := e.L.NewFunction(func(*lua.LState) int { return 0 })
	for _, owner := range []string{"plugin:first:1", "plugin:first:10", "plugin:first:1", "config"} {
		e.owner = owner
		if _, err := e.addModuleProvider(resolver); err != nil {
			t.Fatal(err)
		}
	}
	e.packages["first"] = packageRecord{owner: "plugin:first:1"}
	e.loaded["first"] = true
	e.owner = "config"
	before := e.captureRegistry()
	if err := e.unloadPlugin("first"); err != nil {
		t.Fatal(err)
	}
	if len(e.moduleProviders) != 2 || e.moduleProviders[0].owner != "plugin:first:10" || e.moduleProviders[1].owner != "config" {
		t.Fatalf("unload removed another owner or retained its resolver: %+v", e.moduleProviders)
	}
	e.restoreRegistry(before)
	if !e.loaded["first"] || !slices.Equal(e.moduleProviders, before.moduleProviders) {
		t.Fatal("rejected unload did not restore package resolvers")
	}
}
