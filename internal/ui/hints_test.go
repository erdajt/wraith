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

func TestPickerAndHelpHintsUseActionColors(t *testing.T) {
	for _, kind := range []string{"jobs", "pins", "environment", "help", "leader", "filter"} {
		t.Run(kind, func(t *testing.T) {
			a := fixture(t)
			p := a.Config.Palette()
			want := map[string]string{}
			switch kind {
			case "jobs":
				a.openJobs()
				want["r refresh"], want[a.Config.Keys["job_cancel"]+" cancel"] = p.HintRefresh, p.HintCancel
			case "pins":
				a.memoryCommand("pins", "")
				want[a.Config.Keys["pin_remove"]+" unpin"] = p.HintCancel
			case "environment":
				a.environmentCommand("", false)
				want[a.Config.Keys["env_remove"]+" remove"] = p.HintCancel
			case "help":
				a.memoryCommand("help", "")
				a.popup.Index = len(a.popup.Items) - 1
				want["normal: q · "+a.Config.Keys["quit"]] = p.HintQuit
			case "leader":
				a.Config.Bindings = map[string]string{"x": "quit", "c": "cancel", "r": "terminal-restart 1"}
				a.openLeader("")
				want["x"], want["c"], want["r"] = p.HintQuit, p.HintCancel, p.HintRefresh
				view := strings.Join(a.menuFrame(), "\n")

				for _, it := range a.popup.Items {
					found := false
					for _, row := range strings.Split(view, "\n") {
						if strings.Contains(ansi.Strip(row), "  ·  /"+it.Value) {
							found = true
							assertHintColors(t, row, map[string]string{it.Label: want[it.Label]})
						}
					}

					if !found {
						t.Fatalf("leader binding %q missing", it.Label)
					}
				}

				return
			case "filter":
				a.settingsMenu()
				a.popup.Searching, a.popup.Query = true, "appearance"
				want["esc resets"] = p.HintCancel
			}

			assertHintColors(t, strings.Join(a.menuFrame(), "\n"), want)
		})
	}
}

func TestNoticeHintsFollowRemappedKeysAndPreserveText(t *testing.T) {
	a := fixture(t)
	a.Config.Keys["send"], a.Config.Keys["cancel"] = "f7", "f8"
	a.Config.Keys["terminal_unfocus"] = "f9"
	p := a.Config.Palette()

	for _, tc := range []struct{ text, hint, color string }{
		{"prompt updated · f7 sends", "f7 sends", p.Accent},
		{"wait for the current operation or press f8", "press f8", p.HintCancel},
		{"shell active · f9 returns focus · shift+pgup scrolls", "f9 returns focus", p.Accent},
		{"provider failed · /retry", "/retry", p.HintRefresh},
		{"wait or use /cancel", "/cancel", p.HintCancel},
		{"shell stopped · /terminal-restart 1", "/terminal-restart", p.HintRefresh},
		{"no matching command · esc returns to normal mode", "esc returns to normal mode", p.Accent},
	} {
		t.Run(tc.text, func(t *testing.T) {
			view := a.noticeText(tc.text, p)
			if ansi.Strip(view) != tc.text {
				t.Fatal("hint styling changed notice text")
			}

			assertHintColors(t, view, map[string]string{tc.hint: tc.color})
		})
	}

	a.busy = true
	a.Prompt.Set("keep working")
	a.submit()

	if !strings.Contains(a.notice, "press f8") || strings.Contains(a.notice, "ctrl+c") {
		t.Fatalf("busy notice ignored cancel binding: %q", a.notice)
	}
}
