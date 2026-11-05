package ui

import (
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/ansi"
	"strings"
	"testing"
)

func TestReaderSupportsSearchVisualYankAndReadOnlyMotions(t *testing.T) {
	r := newReader("   first\n\nneedle evidence\nlast")
	r.key(key("}"), 8)
	r.key(key("j^"), 8)
	r.key(key("v5l"), 8)
	copied, exit := r.key(key("y"), 8)
	if copied != "needle" || exit {
		t.Fatalf("yank %q", copied)
	}
	r.key(key("/"), 8)
	r.key(key("first"), 8)
	r.key(tea.KeyMsg{Type: tea.KeyEnter}, 8)
	if r.Buffer.Pos != 3 {
		t.Fatalf("search position %d", r.Buffer.Pos)
	}
	original := r.Buffer.String()
	r.key(key("dd"), 8)
	if r.Buffer.String() != original {
		t.Fatal("reader mutated text")
	}
}

func TestSearchHighlightPreservesTextAndThemeColors(t *testing.T) {
	a := fixture(t)
	p := a.Config.Palette()
	source := "界 needle NEEDLE"
	result := highlightMatches(source, "needle", p)
	if ansi.Strip(result) != source || strings.Count(result, color(p.Warn, true)) < 2 {
		t.Fatalf("highlight damaged text: %q", result)
	}
}
