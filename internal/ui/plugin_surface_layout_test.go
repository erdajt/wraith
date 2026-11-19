package ui

import (
	"fmt"
	"strings"
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

// TestSurfaceBufferRebindAppliesCanonicalNavigation exercises the public API and the next edit.
func TestSurfaceBufferRebindAppliesCanonicalNavigation(t *testing.T) {
	for _, mode := range []string{"normal", "insert"} {
		for _, opened := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/opened=%t", mode, opened), func(t *testing.T) {
				a := pluginFixture(t, fmt.Sprintf(`
local first, target, window
wraith.commands.create("prepare", function()
 first = wraith.buffers.create({lines={"first", "界original"}, editable=true})
 target = wraith.buffers.create({lines={"a", "界target", ""}, editable=true})
 if %t then wraith.windows.open(target, {title="other", focus=false}) end
 window = wraith.windows.open(first, {title="current", focus=true, mode=%q, cursor={line=1,column=1}})
end)
wraith.commands.create("rebind", function()
 wraith.windows.configure(window, {buffer_id=target})
end)
wraith.commands.create("retitle", function()
 wraith.windows.configure(window, {title="retitled"})
end)`, opened, mode))
				applyPluginEvents(t, a, a.runCommand("prepare"))
				view := a.focusedSurfaceView()
				if view == nil {
					t.Fatal("missing focused view")
				}
				id, first := a.windowID, view.window.BufferID
				for _, other := range a.windows {
					if other.surface == nil || other.surface == view {
						continue
					}
					b := other.surface.reader.Buffer
					b.Mode, b.Pos, b.Anchor = "visual", 4, 2
					b.Search, b.Pending, b.Count = "target", "f", "3"
					other.surface.reader.Offset = 1
				}
				view.reader.Buffer.Pending, view.reader.Buffer.Count = "d", "2"
				view.reader.Buffer.Search, view.reader.Offset = "original", 1

				applyPluginEvents(t, a, a.runCommand("rebind"))
				b := view.reader.Buffer
				if a.windowID != id || view.window.BufferID == first || b.Mode != mode || b.Pos != 3 || b.Search != "" || b.Pending != "" || b.Count != "" || b.Anchor != 0 || view.reader.Offset != 0 {
					t.Fatalf("rebind state: window=%+v reader=%+v", view.window, b)
				}
				stored, err := a.plugins.Surfaces().Window(view.window.ID)
				if err != nil || stored.Cursor != view.window.Cursor || stored.Mode != b.Mode {
					t.Fatalf("canonical cursor disagrees: %+v, %v", stored, err)
				}
				if mode == "normal" {
					a.feedInput("i")
				}
				a.feedInput("新")
				target, err := a.plugins.Surfaces().Buffer(view.window.BufferID)
				if err != nil || strings.Join(target.Lines, "\n") != "a\n界新target\n" || a.surfaceBuffers[first].Lines[1] != "界original" {
					t.Fatalf("edit hit wrong document or rune: %+v, %v", target, err)
				}
				stored, _ = a.plugins.Surfaces().Window(view.window.ID)
				if stored.Cursor != (plugin.SurfacePosition{Line: 1, Column: 2}) || stored.Mode != "insert" {
					t.Fatalf("edit cursor not published: %+v", stored)
				}
				b.Key("esc", "")
				a.feedInput("u")
				if strings.Join(a.surfaceBuffers[target.ID].Lines, "\n") != "a\n界target\n" {
					t.Fatal("rebound view lost document undo")
				}
				b.Mode, b.Anchor, b.Search = "visual", 0, "target"
				applyPluginEvents(t, a, a.runCommand("retitle"))
				if b.Mode != "visual" || b.Search != "target" || b.Anchor != 0 {
					t.Fatal("same-buffer update discarded navigation")
				}
				a.plugins.Surfaces().RevokeOwner("config")
				a.reconcileSurfaces()
				if len(a.surfaceWindows) != 0 || a.windows[a.windowID] == nil {
					t.Fatal("rebound view survived owner disposal")
				}
			})
		}
	}
}

// TestSurfaceRebindKeepsAcceptedBoundaryCursors preserves empty lines and end positions before the next motion.
func TestSurfaceRebindKeepsAcceptedBoundaryCursors(t *testing.T) {
	for _, cursor := range []plugin.SurfacePosition{{Line: 1}, {Line: 2, Column: 1}} {
		t.Run(fmt.Sprint(cursor), func(t *testing.T) {
			a := pluginFixture(t, fmt.Sprintf(`
wraith.commands.create("boundary", function()
 local first = wraith.buffers.create({lines={"long", "original", "last"}, editable=true})
 local target = wraith.buffers.create({lines={"界", "", "終"}, editable=true})
 local window = wraith.windows.open(first, {focus=true, cursor={line=%d,column=%d}})
 wraith.windows.configure(window, {buffer_id=target})
end)`, cursor.Line, cursor.Column))
			applyPluginEvents(t, a, a.runCommand("boundary"))
			view := a.focusedSurfaceView()
			if view == nil {
				t.Fatal("missing rebound view")
			}
			buffer := a.surfaceBuffers[view.window.BufferID]
			if view.window.Cursor != cursor || view.reader.Buffer.Pos != surfaceCursorOffset(buffer.Lines, cursor) || view.reader.Buffer.Mode != "normal" {
				t.Fatalf("accepted boundary cursor lost: %+v", view.window)
			}
			a.feedInput("0")
			if view.window.Cursor != (plugin.SurfacePosition{Line: cursor.Line}) {
				t.Fatalf("boundary motion used stale column: %+v", view.window.Cursor)
			}
		})
	}
}
