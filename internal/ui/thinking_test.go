package ui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/erdajt/wraith/internal/session"
)

func TestThinkingExpandsWithVimKeyAndMouse(t *testing.T) {
	a := fixture(t)
	a.Session.Messages = []session.Message{{Role: "assistant", Content: "answer", Thinking: "visible reasoning"}}
	a.focus = "chat"
	a.selected = 0
	a.follow = false
	if strings.Contains(a.View(), "visible reasoning") {
		t.Fatal("thinking should start collapsed")
	}
	a.key(key("za"))
	if !strings.Contains(a.View(), "visible reasoning") {
		t.Fatal("vim toggle did not expand")
	}
	lines := a.transcript(a.mainWidth() - 5)
	for i, line := range lines {
		if line.Thinking {
			a.Update(tea.MouseMsg{X: 4, Y: 3 + i, Action: tea.MouseActionPress, Button: tea.MouseButtonLeft})
			break
		}
	}
	if strings.Contains(a.View(), "visible reasoning") {
		t.Fatal("mouse did not collapse")
	}
	a.Config.Appearance.Mouse = false
	a.Update(tea.MouseMsg{X: 4, Y: 4, Action: tea.MouseActionPress, Button: tea.MouseButtonLeft})
	if a.thinkingOpen(0) {
		t.Fatal("disabled mouse still toggles")
	}
}
