package ui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/ansi"
)

func TestEmptyModificationPreparesNextPrompt(t *testing.T) {
	a := fixture(t)
	a.approve(proposal{Command: "printf original", Text: "printf original", Reason: "inspect output", Continue: true})
	a.approvalAction("modify")
	a.menuKey(tea.KeyMsg{Type: tea.KeyEnter})
	if a.busy || a.popup != nil || a.proposal != nil || a.focus != "prompt" {
		t.Fatal("empty modification sent or retained approval")
	}
	if !strings.HasSuffix(a.Prompt.String(), "modify: ") || !strings.Contains(a.Prompt.String(), "printf original") || !strings.Contains(a.Prompt.String(), "reason: inspect output") {
		t.Fatal("modification draft lost command or reason")
	}
}

func TestInlineModificationEntersPromptWithoutRunningCommand(t *testing.T) {
	a := fixture(t)
	a.loading = true
	a.Prompt.Set("retain existing draft")
	a.approve(proposal{Command: "printf original", Reason: "inspect output", Continue: true})
	a.approvalAction("modify")
	a.menuKey(key("use json output"))
	a.menuKey(tea.KeyMsg{Type: tea.KeyEnter})
	if a.executing || !strings.Contains(a.Prompt.String(), "modify: use json output") || !strings.Contains(a.Prompt.String(), "retain existing draft") {
		t.Fatal("modification did not preserve draft")
	}
	if len(a.Session.Messages) != 0 {
		t.Fatal("loading modification was sent early")
	}
}

func TestModificationCursorTracksInsertionPoint(t *testing.T) {
	for name, text := range map[string]string{"empty": "", "ascii": "dont we fir", "wide": "change 界", "scrolled": strings.Repeat("x", 120)} {
		t.Run(name, func(t *testing.T) {
			a := fixture(t)
			a.width, a.height = 80, 30
			a.approve(proposal{Command: "printf original", Reason: "inspect output"})
			a.approvalAction("modify")
			a.menuKey(key(text))
			p := a.Config.Palette()
			cursor := "\x1b[" + color(p.BG, false) + ";" + color(p.Accent, true) + "m"
			for _, mode := range []string{"insert", "normal"} {
				if mode == "normal" {
					a.menuKey(tea.KeyMsg{Type: tea.KeyEsc})
				}
				found := false
				for _, line := range a.menuFrame() {
					before, after, ok := strings.Cut(line, cursor)
					if !ok {
						continue
					}
					found = true
					column := ansi.StringWidth(text)
					cell := " "
					if mode == "normal" && text != "" {
						runes := []rune(text)
						cell = string(runes[len(runes)-1])
						column -= ansi.StringWidth(cell)
					}
					if got := ansi.StringWidth(before) - 1; got != min(column, 71) {
						t.Fatalf("%s cursor column %d, want %d", mode, got, min(column, 71))
					}
					if !strings.HasPrefix(after, cell+"\x1b[") {
						t.Fatalf("%s cursor covers %q, want %q", mode, ansi.Strip(after), cell)
					}
				}
				if !found {
					t.Fatalf("%s cursor missing", mode)
				}
			}
		})
	}
}
