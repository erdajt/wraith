package ui

import (
	"strings"
	"testing"

	"github.com/erdajt/wraith/internal/session"
	"github.com/erdajt/wraith/internal/terminal"
)

func TestThinkingKeepsMutedColorOnEveryWrappedLine(t *testing.T) {
	a := fixture(t)
	a.showThinking = true
	a.Session.Messages = []session.Message{{Role: "assistant", Thinking: strings.Repeat("reasoning ", 20) + "\nsecond paragraph"}}
	p := a.Config.Palette()
	for _, row := range a.transcript(25)[2:] {
		if strings.TrimSpace(row.Text) == "" {
			continue
		}
		screen := terminal.NewScreen(25, 1, 0)
		screen.Feed([]byte(base(p, row.Text, 25)))
		for _, cell := range screen.Cells[0] {
			if strings.TrimSpace(cell.Text) != "" && cell.Style.FG != color(p.Muted, false) {
				t.Fatalf("thinking changed color: %#v", cell)
			}
		}
	}
}

func TestConversationSearchHighlightsBothRoles(t *testing.T) {
	a := fixture(t)
	a.follow = false
	a.Session.Messages = []session.Message{{Role: "user", Content: "needle"}, {Role: "assistant", Content: "NEEDLE"}}
	a.search = "needle"
	screen := terminal.NewScreen(a.width, a.height, 0)
	screen.Feed([]byte(strings.ReplaceAll(a.View(), "\n", "\r\n")))
	matched := 0
	for _, row := range screen.Cells {
		for _, c := range row {
			if c.Style.BG == color(a.Config.Palette().Warn, true) {
				matched++
			}
		}
	}
	if matched != 12 {
		t.Fatalf("highlighted %d cells", matched)
	}
}
