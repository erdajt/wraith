package ui

import (
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"
	"github.com/erdajt/wraith/internal/terminal"
)

func TestKeyHintsUseActionColorsWithoutChangingText(t *testing.T) {
	p := fixture(t).Config.Palette()
	for _, tc := range []struct{ text, color string }{
		{"q quit", p.HintQuit}, {"ctrl+q quit", p.HintQuit}, {"esc close", p.HintQuit},
		{"r refresh", p.HintRefresh}, {"ctrl+x cancel", p.HintCancel}, {"esc rejects", p.HintCancel},
		{"F8 cancel", p.HintCancel}, {"enter select", p.Accent}, {"/ search", p.Accent},
	} {
		t.Run(tc.text, func(t *testing.T) {
			got := keyHints(tc.text, p)
			if ansi.Strip(got) != tc.text || !strings.Contains(got, color(tc.color, false)) {
				t.Fatalf("incorrect hint: %q", got)
			}
		})
	}
	text := "q close · r refresh · ctrl+x cancel · j/k move"
	got := keyHints(text, p)
	if ansi.Strip(got) != text || ansi.StringWidth(got) != ansi.StringWidth(text) {
		t.Fatalf("hint separators changed: %q", got)
	}
}

func TestActionHintColorsReachRenderedViews(t *testing.T) {
	for _, name := range []string{"footer", "busy", "reader", "approval", "completion", "terminal", "small"} {
		t.Run(name, func(t *testing.T) {
			a := fixture(t)
			a.width, a.height = 120, 40
			a.Config.Keys["quit"], a.Config.Keys["cancel"] = "ctrl+x", "f8"
			p := a.Config.Palette()
			var view string
			want := map[string]string{}
			switch name {
			case "footer":
				view = a.View()
				want["ctrl+x quit"] = p.HintQuit
			case "busy":
				a.busy = true
				view = a.View()
				want["f8 cancels"] = p.HintCancel
			case "reader":
				a.popup = &menu{Reader: newReader("saved output")}
				view = strings.Join(a.menuFrame(), "\n")
				want["q close"], want["r refresh"] = p.HintQuit, p.HintRefresh
			case "approval":
				a.approve(proposal{Command: "printf test"})
				view = strings.Join(a.menuFrame(), "\n")
				want["esc rejects"] = p.HintCancel
			case "completion":
				a.Prompt.Set("/sett")
				view = strings.Join(a.completionFrame(), "\n")
				want["esc close"] = p.HintQuit
			case "terminal":
				a.panes[0] = &pane{Screen: terminal.NewScreen(118, 20, 0), Reader: newReader("shell output")}
				view = strings.Join(a.terminalFrame(0, 120, 24), "\n")
				want["r refresh"] = p.HintRefresh
			case "small":
				a.width = 40
				view = a.View()
				want["ctrl+x quit"] = p.HintQuit
			}
			assertHintColors(t, view, want)
		})
	}
}

func assertHintColors(t *testing.T, view string, want map[string]string) {
	t.Helper()
	rows := strings.Split(view, "\n")
	for text, c := range want {
		found := false
		for _, row := range rows {
			plain := ansi.Strip(row)
			index := strings.Index(plain, text)
			if index < 0 {
				continue
			}
			found = true
			screen := terminal.NewScreen(ansi.StringWidth(row)+1, 1, 0)
			screen.Feed([]byte(row))
			start := ansi.StringWidth(plain[:index])
			for _, cell := range screen.Cells[0][start : start+ansi.StringWidth(text)] {
				if cell.Style.FG != color(c, false) {
					t.Fatalf("%q: cell %q has color %s, want %s", text, cell.Text, cell.Style.FG, c)
				}
			}
		}
		if !found {
			t.Fatalf("hint %q missing from view", text)
		}
	}
}
