package ui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/erdajt/wraith/internal/session"
)

func TestSearchMovesToEachMatchOnTheSameLine(t *testing.T) {
	a := fixture(t)
	a.focus, a.follow = "chat", false
	a.chatCursor = 0
	a.Session.Messages = []session.Message{{Role: "assistant", Content: "界 needle NEEDLE"}}
	a.key(key("/needle"))
	a.key(tea.KeyMsg{Type: tea.KeyEnter})
	if a.chatCursor != 1 || a.chatColumn != 3 {
		t.Fatalf("first match %d:%d", a.chatCursor, a.chatColumn)
	}
	a.key(key("n"))
	if a.chatColumn != 10 {
		t.Fatalf("next match column %d", a.chatColumn)
	}
	a.key(key("N"))
	if a.chatColumn != 3 {
		t.Fatal("reverse search skipped match")
	}
}

func TestPromptSearchHighlightsAndPlacesCursor(t *testing.T) {
	a := fixture(t)
	a.Prompt.Set("界 needle NEEDLE")
	a.key(tea.KeyMsg{Type: tea.KeyEsc})
	a.key(key("/needle"))
	a.key(tea.KeyMsg{Type: tea.KeyEnter})
	if a.Prompt.Pos != 2 {
		t.Fatalf("prompt match at %d", a.Prompt.Pos)
	}
	if !strings.Contains(strings.Join(a.promptLines(40), "\n"), color(a.Config.Palette().Warn, true)) {
		t.Fatal("prompt matches not highlighted")
	}
	a.key(key("n"))
	if a.Prompt.Pos != 9 {
		t.Fatalf("next prompt match at %d", a.Prompt.Pos)
	}
}
