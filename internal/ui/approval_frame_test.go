package ui

import (
	"fmt"
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"
	"github.com/erdajt/wraith/internal/session"
	"github.com/erdajt/wraith/internal/terminal"
)

func TestApprovalClearsWorkspaceAndKeepsAllActionsReachable(t *testing.T) {
	for _, size := range [][2]int{{50, 18}, {90, 30}, {140, 45}} {
		for _, background := range []string{"theme", "inherit"} {
			t.Run(fmt.Sprintf("%dx%d/%s", size[0], size[1], background), func(t *testing.T) {
				a := fixture(t)
				a.width, a.height = size[0], size[1]
				a.Config.Appearance.Background = background
				a.Session.Messages = []session.Message{{Role: "assistant", Content: strings.Repeat("chat-underlay ", 50)}}
				a.Prompt.Set("draft-underlay")

				for i := range a.panes {
					a.panes[i] = &pane{Visible: true}
					w, h := a.paneSize(i)
					a.panes[i].Screen = terminal.NewScreen(w-2, h-2, 10)
					a.panes[i].Screen.Feed([]byte("shell-underlay"))
				}

				command := strings.Repeat("printf 'a long command argument'\n", 12)
				a.approve(proposal{Command: command, Reason: strings.Repeat("inspect local output ", 15), Background: true})

				for i, action := range a.popup.Items {
					a.popup.Index = i
					view := a.View()
					plain := ansi.Strip(view)
					if strings.Contains(plain, "underlay") {
						t.Fatal("workspace bled through approval")
					}

					if !strings.Contains(plain, "APPROVAL REQUIRED") || !strings.Contains(plain, "not executed · background") || !strings.Contains(plain, "reason:") || !strings.Contains(plain, "› "+action.Label) {
						t.Fatalf("approval lost heading, reason or selected action: %s", plain)
					}

					rows := strings.Split(view, "\n")
					if len(rows) != a.height {
						t.Fatalf("approval height %d, want %d", len(rows), a.height)
					}
					for _, row := range rows {
						if ansi.StringWidth(row) != a.width {
							t.Fatalf("approval row width %d", ansi.StringWidth(row))
						}
					}

					if !strings.Contains(view, "\x1b[1;7;") {
						t.Fatal("selected action has no contrast fill")
					}
				}

				if a.proposal.Resolved != command {
					t.Fatal("layout changed approved command")
				}

				if background == "inherit" && strings.Contains(a.View(), "48;2;") {
					t.Fatal("approval overrode inherited background")
				}
			})
		}
	}
}
