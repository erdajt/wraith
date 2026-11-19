package plugin

import (
	"testing"

	"github.com/erdajt/wraith/internal/config"
	lua "github.com/yuin/gopher-lua"
)

func TestSurfaceLeaseRollbackAndUnloadedOwner(t *testing.T) {
	e, err := load(t.Context(), t.TempDir(), config.Default(), nil)
	if err != nil {
		t.Fatal(err)
	}
	defer e.L.Close()
	defer e.services.close()
	defer e.surfaces.close()
	e.packages["fixture"] = packageRecord{owner: "package:fixture:1"}
	e.owner = "package:fixture:1"
	before := e.captureRegistry()
	lease := e.surfaceLease()
	id, err := e.surfaces.Reserve(e.owner, "buffer")
	if err != nil {
		t.Fatal(err)
	}
	e.surfaceReserved = append(e.surfaceReserved, id)
	e.restoreRegistry(before)
	if e.surfaces.OwnerValid(e.owner, lease) || len(e.surfaces.reserved) != 0 {
		t.Fatal("rollback retained its receipt or reservation")
	}
	replacement := e.surfaceLease()
	if replacement == lease || !e.surfaces.OwnerValid(e.owner, replacement) {
		t.Fatal("live package could not retry with a fresh receipt")
	}
	delete(e.packages, "fixture")
	err = e.L.CallByParam(lua.P{Fn: e.L.NewFunction(func(*lua.LState) int { e.surfaceLease(); return 0 }), NRet: 0, Protect: true})
	if err == nil {
		t.Fatal("unloaded owner recreated a valid lease")
	}
}
