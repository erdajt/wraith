package plugin

import (
	"testing"

	"github.com/erdajt/wraith/internal/config"
	lua "github.com/yuin/gopher-lua"
)

func TestStagedSurfacesReadWritesWithoutPublishingBeforeHostAcceptance(t *testing.T) {
	e, err := load(t.Context(), t.TempDir(), config.Default(), nil)
	if err != nil {
		t.Fatal(err)
	}
	defer e.L.Close()
	defer e.services.close()
	defer e.surfaces.close()
	before := e.captureRegistry()
	id := e.reserveSurface("buffer")
	e.stageSurface(SurfaceMutation{Operation: "buffer.create", ID: id, Buffer: &SurfaceBuffer{Lines: []string{"first"}}})
	observed, err := e.previewSurfaces().Buffer(id)
	if err != nil || observed.Revision != 1 {
		t.Fatalf("preview create: %+v, %v", observed, err)
	}
	e.stageSurface(SurfaceMutation{Operation: "buffer.set_lines", ID: id, Expected: 1, Start: 0, End: 1, Lines: []string{"second"}})
	if _, err := e.surfaces.Buffer(id); err == nil {
		t.Fatal("surface published before host acceptance")
	}
	for _, effect := range e.effects {
		if _, err := e.surfaces.Apply(effect.Data.(SurfaceMutation)); err != nil {
			t.Fatal(err)
		}
	}
	stored, _ := e.surfaces.Buffer(id)
	if stored.Revision != 2 || stored.Lines[0] != "second" {
		t.Fatalf("ordered host mutations: %+v", stored)
	}
	e.restoreRegistry(before)
}

func TestCaughtSurfaceFailureDiscardsOnlyItsReservation(t *testing.T) {
	e, err := load(t.Context(), t.TempDir(), config.Default(), nil)
	if err != nil {
		t.Fatal(err)
	}
	defer e.L.Close()
	defer e.services.close()
	defer e.surfaces.close()
	err = e.L.CallByParam(lua.P{Fn: e.L.NewFunction(func(*lua.LState) int {
		id := e.reserveSurface("buffer")
		e.stageSurface(SurfaceMutation{Operation: "buffer.create", ID: id, Buffer: &SurfaceBuffer{Lines: []string{"\x1b"}}})
		return 0
	}), Protect: true})
	if err == nil || len(e.surfaces.reserved) != 0 || len(e.effects) != 0 {
		t.Fatalf("invalid surface retained state: err=%v reservations=%d effects=%d", err, len(e.surfaces.reserved), len(e.effects))
	}
}
