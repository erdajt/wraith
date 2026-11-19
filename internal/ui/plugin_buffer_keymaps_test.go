package ui

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/erdajt/wraith/internal/keymap"
	"github.com/erdajt/wraith/internal/plugin"
)

func TestBufferLocalMappingsStayScopedAndRefresh(t *testing.T) {
	a := pluginFixture(t, `wraith.keymap.set("buffer.normal", "x", {keys="j"})`)
	buffer := surfaceFixture(t, a, false)
	first := surfaceWindowFixture(t, a, buffer.ID, true)
	other := surfaceFixture(t, a, false)
	second := surfaceWindowFixture(t, a, other.ID, false)
	binding := keymap.Binding{Mode: "buffer.normal", Keys: "x", Input: "G"}
	set := func() {
		t.Helper()
		current := a.surfaceBuffers[buffer.ID]
		err := a.applySurfaceMutation(a.plugins, plugin.SurfaceMutation{Operation: "buffer.set_keymap", ID: buffer.ID, Owner: "config", Lease: a.plugins.Surfaces().OwnerLease("config"), Expected: current.Revision, Binding: &binding})
		if err != nil {
			t.Fatal(err)
		}
	}
	set()
	a.key(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("x")})
	if a.windows[first].surface.reader.Buffer.Pos != 14 {
		t.Fatal("local mapping did not override global mapping")
	}
	a.focusWindow(second)
	a.key(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("x")})
	if a.windows[second].surface.reader.Buffer.Pos != 6 {
		t.Fatal("local mapping leaked into another buffer")
	}
	binding.Input = "gg"
	set()
	a.focusWindow(first)
	a.key(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("x")})
	if a.windows[first].surface.reader.Buffer.Pos != 0 {
		t.Fatal("changed local mapping reused stale cache")
	}
}

func TestNativePromptLocalMappingAndDialogOwnership(t *testing.T) {
	a := pluginFixture(t, `wraith.keymap.set("buffer.normal", "x", "prompt.send")
wraith.commands.create("input",function() wraith.ui.input({},function() end) end)`)
	a.Prompt.Set("hidden draft")
	a.Prompt.Mode = "normal"
	a.syncNativeSurfaces()
	var buffer plugin.SurfaceBuffer
	for _, item := range a.surfaceBuffers {
		if item.Projection == "prompt" {
			buffer = item
		}
	}
	binding := keymap.Binding{Mode: "prompt.normal", Keys: "x", Input: "$"}
	if err := a.applySurfaceMutation(a.plugins, plugin.SurfaceMutation{Operation: "buffer.set_keymap", ID: buffer.ID, Owner: "config", Lease: a.plugins.Surfaces().OwnerLease("config"), Expected: buffer.Revision, Binding: &binding}); err != nil {
		t.Fatal(err)
	}
	a.key(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("x")})
	if a.Prompt.Pos != len(a.Prompt.Text)-1 {
		t.Fatal("native prompt ignored its local mapping")
	}
	result := a.runCommand("input")().(pluginResultMsg)
	a.openPluginDialog(a.plugins, result.Result.Effects[0].Data.(plugin.Dialog))
	a.pluginDialog.reader.Buffer.Mode = "normal"
	a.key(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("x")})
	if a.pluginDialog == nil || a.Prompt.String() != "hidden draft" || a.busy {
		t.Fatal("dialog mapping submitted the hidden conversation draft")
	}
}
