package ui

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func TestDocumentedSurfaceExampleOpensEditableMappedView(t *testing.T) {
	document, err := os.ReadFile(filepath.Join("..", "..", "docs", "lua", "surfaces.md"))
	if err != nil {
		t.Fatal(err)
	}
	_, example, found := strings.Cut(string(document), "```lua\n")
	if !found {
		t.Fatal("surface example is missing")
	}
	example, _, found = strings.Cut(example, "\n```")
	if !found {
		t.Fatal("surface example is incomplete")
	}
	a := pluginFixture(t, example)
	a.width, a.height = 140, 40
	_, cmd := a.Update(a.runCommand("notes")())
	if cmd != nil {
		applyPluginEvents(t, a, cmd)
	}
	view := a.focusedSurfaceView()
	if view == nil || a.surfaceBuffers[view.window.BufferID].Name != "review notes" {
		t.Fatalf("example did not open its view: %s", a.notice)
	}
	a.key(tea.KeyMsg{Type: tea.KeySpace, Runes: []rune(" ")})
	a.key(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("f")})
	if view.reader.Buffer.Pos != len([]rune("scope\n\n")) {
		t.Fatal("documented buffer-local mapping did not move to findings")
	}
	a.feedInput("Ichecked: <Esc>")
	stored, err := a.plugins.Surfaces().Buffer(view.window.BufferID)
	if err != nil || stored.Lines[2] != "checked: findings" {
		t.Fatalf("example did not accept native editing: %#v, %v", stored.Lines, err)
	}
}
