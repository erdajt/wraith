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

func TestThemedRowsRetainBackgroundAcrossNestedStyles(t *testing.T) {
	a := fixture(t)
	a.Prompt.Set("draft")
	a.popup = &menu{Title: "installed models", Items: []item{{Label: "local-model", Detail: "7 gb"}}}
	for theme, p := range palettes {
		a.Config.Theme = theme
		screen := terminal.NewScreen(a.width, a.height, 0)
		screen.Feed([]byte("\x1b[3m" + strings.ReplaceAll(a.View(), "\n", "\r\n")))
		for y, row := range screen.Cells {
			for x, cell := range row {
				if cell.Style.BG != color(p.BG, true) || cell.Style.Italic {
					t.Fatalf("%s cell %d,%d leaked host style: %+v", theme, x, y, cell.Style)
				}
			}
		}
		y := 3 + a.bodyHeight()
		if screen.Cells[y][a.width-1].Text != "╮" || screen.Cells[y+1][a.width-1].Text != "│" {
			t.Fatal("prompt borders do not align")
		}
	}
}

func TestInheritedPaneKeepsDefaultBackground(t *testing.T) {
	a := fixture(t)
	a.panes[0] = &pane{Visible: true}
	w, h := a.paneSize(0)
	a.panes[0].Screen = terminal.NewScreen(w-2, h-2, 10)
	screen := terminal.NewScreen(a.width, a.height, 0)
	screen.Feed([]byte(strings.ReplaceAll(a.View(), "\n", "\r\n")))
	x, y := (a.width-w)/2, (a.height-h)/2
	if screen.Cells[y+1][x+1].Style.BG != "" {
		t.Fatal("inherited shell lost its host background")
	}
	if screen.Cells[0][0].Style.BG == "" {
		t.Fatal("app background became transparent with the shell")
	}
}
