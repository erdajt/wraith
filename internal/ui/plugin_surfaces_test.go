package ui

import (
	"testing"

	"github.com/erdajt/wraith/internal/plugin"
)

// surfaceFixture creates an owned buffer through the same checked host bridge as Lua effects.
func surfaceFixture(t *testing.T, a *App, editable bool) plugin.SurfaceBuffer {
	t.Helper()
	store := a.plugins.Surfaces()
	id, err := store.Reserve("config", "buffer")
	if err != nil {
		t.Fatal(err)
	}
	err = a.applySurfaceMutation(a.plugins, plugin.SurfaceMutation{Operation: "buffer.create", ID: id, Owner: "config", Lease: store.OwnerLease("config"), Buffer: &plugin.SurfaceBuffer{Editable: editable, Lines: []string{"first", "界needle", "last"}}})
	if err != nil {
		t.Fatal(err)
	}
	return a.surfaceBuffers[id]
}

// surfaceWindowFixture opens one view without depending on startup command dispatch.
func surfaceWindowFixture(t *testing.T, a *App, buffer string, focus bool) uint64 {
	t.Helper()
	store := a.plugins.Surfaces()
	id, err := store.Reserve("config", "window")
	if err != nil {
		t.Fatal(err)
	}
	err = a.applySurfaceMutation(a.plugins, plugin.SurfaceMutation{Operation: "window.create", ID: id, Owner: "config", Lease: store.OwnerLease("config"), Window: &plugin.SurfaceWindow{BufferID: buffer, Kind: "split", Position: "right", Focus: focus}})
	if err != nil {
		t.Fatal(err)
	}
	return a.surfaceWindows[id]
}

func TestSurfaceViewsShareTextWithIndependentNavigation(t *testing.T) {
	a := pluginFixture(t, "")
	buffer := surfaceFixture(t, a, false)
	first := surfaceWindowFixture(t, a, buffer.ID, true)
	second := surfaceWindowFixture(t, a, buffer.ID, false)
	one, two := a.windows[first].surface.reader, a.windows[second].surface.reader
	one.Buffer.Pos, one.Buffer.Search, one.Offset = 7, "needle", 1
	if two.Buffer.Pos != 0 || two.Buffer.Search != "" || two.Offset != 0 || a.windowID != first {
		t.Fatal("creating a view stole focus or shared navigation state")
	}
	a.Prompt.Set("unsent draft")
	err := a.applySurfaceMutation(a.plugins, plugin.SurfaceMutation{Operation: "buffer.set_lines", ID: buffer.ID, Owner: "config", Lease: a.plugins.Surfaces().OwnerLease("config"), Expected: buffer.Revision, Start: 0, End: 1, Lines: []string{"prefix", "first"}})
	if err != nil {
		t.Fatal(err)
	}
	if one.Buffer.Pos != 14 || two.Buffer.Pos != 0 || one.Buffer.String() != two.Buffer.String() || a.Prompt.String() != "unsent draft" {
		t.Fatal("background edit lost cursor anchors or changed the draft")
	}
	if err := a.applySurfaceMutation(a.plugins, plugin.SurfaceMutation{Operation: "buffer.set_lines", ID: buffer.ID, Owner: "config", Lease: a.plugins.Surfaces().OwnerLease("config"), Expected: buffer.Revision, Start: 0, End: 1, Lines: []string{"stale"}}); err == nil {
		t.Fatal("stale text replacement was accepted")
	}
}

func TestSurfaceOwnerRevocationRestoresNativeFocus(t *testing.T) {
	a := pluginFixture(t, "")
	buffer := surfaceFixture(t, a, false)
	surfaceWindowFixture(t, a, buffer.ID, true)
	a.plugins.Surfaces().RevokeOwner("config")
	a.reconcileSurfaces()
	if len(a.surfaceWindows) != 0 || len(a.surfaceBuffers) != 0 || a.conversationView.surface != nil || a.windows[a.windowID] == nil {
		t.Fatal("owner revocation left a stale focused surface")
	}
}
