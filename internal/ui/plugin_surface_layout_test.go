package ui

import (
	"testing"

	"github.com/erdajt/wraith/internal/plugin"
)

func TestSurfaceWindowConfigurationMovesExistingLeaf(t *testing.T) {
	a := pluginFixture(t, "")
	a.width, a.height = 120, 40
	buffer := surfaceFixture(t, a, false)
	id := surfaceWindowFixture(t, a, buffer.ID, true)
	view := a.windows[id].surface
	configure := func(window plugin.SurfaceWindow) error {
		return a.applySurfaceMutation(a.plugins, plugin.SurfaceMutation{Operation: "window.configure", ID: window.ID, Owner: window.Owner, Lease: a.plugins.Surfaces().OwnerLease(window.Owner), Expected: view.window.Revision, Window: &window})
	}
	window := view.window
	window.Kind, window.Width, window.Height = "float", 40, 10
	window.Row, window.Column = 4, 5
	if err := configure(window); err != nil {
		t.Fatal(err)
	}
	if a.windowTree.contains(id) || a.windowID != id {
		t.Fatal("floating migration retained a tiled leaf or changed focus")
	}
	places := a.windowPlaces()
	if got := places[len(places)-1]; got.id != id || got.x != 5 || got.y != 7 || got.width != 40 || got.height != 10 {
		t.Fatalf("floating placement = %#v", got)
	}

	window = view.window
	window.Kind, window.Position = "split", "left"
	if err := configure(window); err != nil {
		t.Fatal(err)
	}
	if !a.windowTree.contains(id) || a.windowPlaces()[0].id != id {
		t.Fatal("floating view did not move into the requested split position")
	}
	before := view.window.Revision
	window = view.window
	window.RelativeTo = "missing"
	if err := configure(window); err == nil {
		t.Fatal("invalid relative target was accepted")
	}
	stored, err := a.plugins.Surfaces().Window(window.ID)
	if err != nil || stored.Revision != before || !a.windowTree.contains(id) {
		t.Fatal("rejected placement changed store or layout")
	}
}

func TestClosingNativeRootKeepsFloatingViewsAndLiveFallback(t *testing.T) {
	a := pluginFixture(t, "")
	buffer := surfaceFixture(t, a, false)
	id := surfaceWindowFixture(t, a, buffer.ID, true)
	view := a.windows[id].surface
	window := view.window
	window.Kind, window.Width, window.Height = "float", 40, 10
	if err := a.applySurfaceMutation(a.plugins, plugin.SurfaceMutation{Operation: "window.configure", ID: window.ID, Owner: window.Owner, Lease: a.plugins.Surfaces().OwnerLease(window.Owner), Expected: window.Revision, Window: &window}); err != nil {
		t.Fatal(err)
	}
	closed := a.windowTree.id
	a.focusWindow(closed)
	a.windowAction("c")
	if a.windowTree.contains(closed) || a.windows[a.windowTree.id] == nil || a.windows[id] == nil {
		t.Fatal("closing the native root left a dead leaf or removed a floating view")
	}
}

func TestSurfaceWindowSizesUseNativeDividers(t *testing.T) {
	a := pluginFixture(t, "")
	a.width, a.height = 140, 40
	buffer := surfaceFixture(t, a, false)
	id := surfaceWindowFixture(t, a, buffer.ID, true)
	view := a.windows[id].surface
	window := view.window
	window.Width = 40
	if err := a.applySurfaceMutation(a.plugins, plugin.SurfaceMutation{Operation: "window.configure", ID: window.ID, Owner: window.Owner, Lease: a.plugins.Surfaces().OwnerLease(window.Owner), Expected: window.Revision, Window: &window}); err != nil {
		t.Fatal(err)
	}
	width, _ := a.chatSize()
	if width != 40 {
		t.Fatalf("requested width = %d, want 40", width)
	}
	a.windowAction(">")
	width, height := a.chatSize()
	stored, err := a.plugins.Surfaces().Window(window.ID)
	if err != nil || width <= 40 || stored.Width != width || stored.Height != height {
		t.Fatalf("user resize not visible to plugins: %#v, measured %dx%d, error %v", stored, width, height, err)
	}
	a.width = 50
	for _, place := range a.windowPlaces() {
		if place.x < 0 || place.x+place.width > a.width || place.width <= 0 {
			t.Fatalf("small terminal placed pane outside bounds: %#v", place)
		}
	}
}

func TestOrderedTextEditThenWindowConfigurationKeepsRevisionFlow(t *testing.T) {
	a := pluginFixture(t, `wraith.commands.create("ordered",function()
 local buffer=wraith.buffers.create({lines={"first","second"},editable=true})
 local window=wraith.windows.open(buffer,{focus=true,cursor={line=1,column=2}})
 wraith.buffers.set_lines(buffer,0,0,{"prefix"})
 wraith.windows.configure(window,{title="final",cursor={line=0,column=1}})
end)`)
	a.Update(a.runCommand("ordered")())
	view := a.focusedSurfaceView()
	if view == nil || view.window.Title != "final" || view.reader.Buffer.Pos != 1 {
		t.Fatalf("ordered configuration was invalidated by host rebase: %s, view=%#v", a.notice, view)
	}
}

func TestSplitFromPromptKeepsPluginViewStateIndependent(t *testing.T) {
	a := pluginFixture(t, "")
	a.width, a.height = 140, 40
	buffer := surfaceFixture(t, a, true)
	first := surfaceWindowFixture(t, a, buffer.ID, true)
	a.focus = "prompt"
	a.windowAction("v")
	second := a.windowID
	if first == second || a.windows[first].surface == a.windows[second].surface || a.windows[first].surface.reader == a.windows[second].surface.reader {
		t.Fatal("split from prompt borrowed the original plugin editor")
	}
	if len(a.plugins.Surfaces().Index().Windows) != 2 {
		t.Fatal("split from prompt was not published as an owned window")
	}
	a.feedInput("j")
	if a.windows[first].surface.reader.Buffer.Pos != 0 {
		t.Fatal("new pane moved the original cursor")
	}
}

func TestSurfaceReconciliationDoesNotWriteIntermediateCursors(t *testing.T) {
	a := pluginFixture(t, "")
	buffer := surfaceFixture(t, a, true)
	surfaceWindowFixture(t, a, buffer.ID, true)
	a.feedInput("jl")
	view := a.focusedSurfaceView()
	if _, err := a.plugins.Surfaces().HostApply(plugin.SurfaceMutation{Operation: "buffer.set_lines", ID: buffer.ID, Owner: buffer.Owner, Expected: buffer.Revision, Start: 0, End: 0, Lines: []string{"prefix"}}); err != nil {
		t.Fatal(err)
	}
	before, _ := a.plugins.Surfaces().Window(view.window.ID)
	a.notice = "unchanged"
	a.reconcileSurfaces()
	after, _ := a.plugins.Surfaces().Window(view.window.ID)
	if a.notice != "unchanged" || before.Revision != after.Revision || view.window.Revision != after.Revision || view.reader.Buffer.Pos != 14 {
		t.Fatalf("reconcile changed canonical state: before=%#v after=%#v cursor=%d notice=%q", before, after, view.reader.Buffer.Pos, a.notice)
	}
}
