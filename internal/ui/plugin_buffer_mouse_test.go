package ui

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/erdajt/wraith/internal/plugin"
)

func TestSurfaceMouseUsesTopmostViewAndGraphemeCells(t *testing.T) {
	a := pluginFixture(t, "")
	a.width, a.height, a.Config.Appearance.Mouse = 100, 30, true
	buffer := surfaceFixture(t, a, false)
	if err := a.applySurfaceMutation(a.plugins, plugin.SurfaceMutation{Operation: "buffer.set_lines", ID: buffer.ID, Owner: buffer.Owner, Lease: a.plugins.Surfaces().OwnerLease(buffer.Owner), Expected: buffer.Revision, Start: 0, End: 3, Lines: []string{"界\téx"}}); err != nil {
		t.Fatal(err)
	}
	id := surfaceWindowFixture(t, a, buffer.ID, true)
	window := a.windows[id].surface.window
	window.Kind, window.Width, window.Height, window.Column, window.Row = "float", 30, 8, 4, 2
	if err := a.applySurfaceMutation(a.plugins, plugin.SurfaceMutation{Operation: "window.configure", ID: window.ID, Owner: window.Owner, Lease: a.plugins.Surfaces().OwnerLease(window.Owner), Expected: window.Revision, Window: &window}); err != nil {
		t.Fatal(err)
	}
	a.focusWindow(a.windowTree.id)
	click := func(x int) {
		t.Helper()
		a.mouse(tea.MouseMsg{X: x, Y: 6, Button: tea.MouseButtonLeft, Action: tea.MouseActionPress})
		if a.windowID != id {
			t.Fatal("click reached a tiled view behind the floating pane")
		}
	}
	click(6)
	if got := a.windows[id].surface.reader.Buffer.Pos; got != 0 {
		t.Fatalf("wide grapheme click = rune %d", got)
	}
	click(8)
	if got := a.windows[id].surface.reader.Buffer.Pos; got != 1 {
		t.Fatalf("tab click = rune %d", got)
	}
	click(9)
	if got := a.windows[id].surface.reader.Buffer.Pos; got != 2 {
		t.Fatalf("combining grapheme click = rune %d", got)
	}
	stored, err := a.plugins.Surfaces().Window(window.ID)
	if err != nil || stored.Cursor.Column != 2 {
		t.Fatal("pointer cursor was not visible through the window handle")
	}
}
