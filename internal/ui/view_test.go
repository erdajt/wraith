package ui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/ansi"
	"github.com/erdajt/wraith/internal/session"
	"github.com/erdajt/wraith/internal/terminal"
)

func TestLayoutsStayInsideTerminalAndStripControlPayloads(t *testing.T) {
	a := fixture(t)
	a.Session.Messages = []session.Message{{Role: "assistant", Content: "\x1b]52;c;secret\a\x1b[2Jvisible evidence\n```sh\nprintf test\n```"}}
	a.Prompt.Set("界 draft\nnext line")
	a.notice = "status\nwith a line break"
	for theme := range palettes {
		for _, size := range [][2]int{{50, 18}, {80, 24}, {120, 40}} {
			a.Config.Theme = theme
			a.Update(tea.WindowSizeMsg{Width: size[0], Height: size[1]})
			for _, layout := range []string{"chat", "split", "overlay", "menu"} {
				a.panes = [2]*pane{}
				a.popup = nil
				switch layout {
				case "split", "overlay":
					index := 0
					if layout == "split" {
						index = 1
					}
					a.panes[index] = &pane{Visible: true}
					w, h := a.paneSize(index)
					a.panes[index].Screen = terminal.NewScreen(w-2, h-2, 10)
					a.panes[index].Screen.Feed([]byte("$ ready"))
				case "menu":
					a.popup = &menu{Title: "settings", Items: []item{{Label: "multi\nline"}}}
				}
				view := a.View()
				rows := strings.Split(view, "\n")
				if len(rows) != size[1] {
					t.Fatalf("%s/%s: %d rows, want %d", theme, layout, len(rows), size[1])
				}
				for i, row := range rows {
					if width := ansi.StringWidth(row); width != size[0] {
						t.Fatalf("%s/%s row %d: width %d, want %d", theme, layout, i, width, size[0])
					}
				}
				if strings.Contains(view, "\x1b]52") || strings.Contains(view, "\x1b[2J") {
					t.Fatal("model control sequence reached host terminal")
				}
			}
		}
	}
}

func TestScrollUpStartsAtVisibleBottom(t *testing.T) {
	a := fixture(t)
	a.focus = "chat"
	a.Session.Messages = []session.Message{{Role: "assistant", Content: strings.Repeat("evidence\n", 100)}}
	bottom := len(a.transcript(a.mainWidth()-5)) - a.bodyHeight()
	a.key(key("k"))
	if a.offset != bottom-1 {
		t.Fatalf("scroll jumped away from visible content: %d, want %d", a.offset, bottom-1)
	}
	for range 200 {
		a.key(key("j"))
	}
	a.key(key("k"))
	if a.offset != bottom-1 {
		t.Fatal("scroll overshoot accumulated past the end")
	}
}
