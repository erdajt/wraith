package ui

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/erdajt/wraith/internal/plugin"
)

func TestSharedBufferEditingUndoAndReadonlyMotions(t *testing.T) {
	a := pluginFixture(t, "")
	buffer := surfaceFixture(t, a, true)
	first := surfaceWindowFixture(t, a, buffer.ID, true)
	second := surfaceWindowFixture(t, a, buffer.ID, false)
	one, two := a.windows[first].surface, a.windows[second].surface
	a.feedInput("ix<Esc>")
	if one.reader.Buffer.String() != "xfirst\n界needle\nlast" || two.reader.Buffer.String() != one.reader.Buffer.String() {
		t.Fatal("editing one pane did not update its shared document")
	}
	a.focusWindow(second)
	a.feedInput("u")
	if one.reader.Buffer.String() != "first\n界needle\nlast" || two.reader.Buffer.String() != one.reader.Buffer.String() {
		t.Fatal("undo from another pane did not use shared document history")
	}
	a.feedInput("/needle<Enter>")
	if two.reader.Buffer.Pos != 7 || one.reader.Buffer.Search != "" {
		t.Fatal("search cursor or independent search state is incorrect")
	}
	stored, err := a.plugins.Surfaces().Window(two.window.ID)
	if err != nil || stored.Cursor != (plugin.SurfacePosition{Line: 1, Column: 1}) {
		t.Fatalf("stored cursor = %#v, error = %v", stored.Cursor, err)
	}

	readonly := surfaceFixture(t, a, false)
	surfaceWindowFixture(t, a, readonly.ID, true)
	before := a.focusedSurfaceView().reader.Buffer.String()
	a.feedInput("dd")
	if got := a.focusedSurfaceView().reader.Buffer.String(); got != before {
		t.Fatal("read-only input changed buffer text")
	}
	a.feedInput("G0Y")
	if got := string(a.focusedSurfaceView().reader.Buffer.Register); got != "last" {
		t.Fatalf("read-only line yank = %q", got)
	}
}

func TestExternalBufferDeletionRebasesCursorBeforeStoreSync(t *testing.T) {
	a := pluginFixture(t, "")
	buffer := surfaceFixture(t, a, true)
	id := surfaceWindowFixture(t, a, buffer.ID, true)
	view := a.windows[id].surface
	a.feedInput("G$")
	err := a.applySurfaceMutation(a.plugins, plugin.SurfaceMutation{Operation: "buffer.set_lines", ID: buffer.ID, Owner: buffer.Owner, Lease: a.plugins.Surfaces().OwnerLease(buffer.Owner), Expected: buffer.Revision, Start: 0, End: 3, Lines: []string{"x"}})
	if err != nil {
		t.Fatal(err)
	}
	stored, err := a.plugins.Surfaces().Window(view.window.ID)
	if err != nil || stored.Revision != view.window.Revision || stored.Cursor.Line != 0 || view.reader.Buffer.Pos != 0 {
		t.Fatalf("cursor did not reconcile: window=%#v position=%d error=%v", stored, view.reader.Buffer.Pos, err)
	}
}

func TestPluginPickerAcceptsNativeInputMappings(t *testing.T) {
	a := pluginFixture(t, `wraith.keymap.set("picker", "x", {keys="j"})
wraith.commands.create("pick",function()
 wraith.ui.select({"first", "second"}, {}, function() end)
end)`)
	result := a.runCommand("pick")().(pluginResultMsg)
	a.openPluginDialog(a.plugins, result.Result.Effects[0].Data.(plugin.Dialog))
	a.key(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("x")})
	if a.pluginDialog.selected != 1 || a.inputSnapshot()["available"] != true {
		t.Fatal("picker native input mapping did not reach the dialog")
	}
	a.setInputMode("insert")
	if a.pluginDialog == nil || a.pluginDialog.spec.Kind != "select" {
		t.Fatal("mode change altered picker ownership")
	}
}
