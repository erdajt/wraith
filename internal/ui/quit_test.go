package ui

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/erdajt/wraith/internal/session"
)

func TestExQuitAliasesPreserveStreamingRepliesInEitherFocus(t *testing.T) {
	for _, focus := range []string{"prompt", "chat"} {
		for _, alias := range []string{"q", "qa", "quit"} {
			t.Run(focus+"/"+alias, func(t *testing.T) {
				a := fixture(t)
				a.focus, a.commandMode, a.command = focus, true, alias
				a.busy = true
				a.partial = session.Message{Role: "assistant", Content: "partial reply"}
				cmd := a.key(tea.KeyMsg{Type: tea.KeyEnter})
				if cmd == nil {
					t.Fatalf("quit rejected: %s", a.notice)
				}
				if _, ok := cmd().(tea.QuitMsg); !ok {
					t.Fatal("alias did not quit")
				}
				saved, err := a.Store.Load(a.Session.ID)
				if err != nil || len(saved.Messages) != 1 || saved.Messages[0].Content != "partial reply" || !saved.Messages[0].Interrupted {
					t.Fatalf("streamed reply lost: %+v, %v", saved, err)
				}
			})
		}
	}
}

func TestExQuitFailureRetainsSessionAndPartialReply(t *testing.T) {
	for _, alias := range []string{"q", "qa", "quit"} {
		t.Run(alias, func(t *testing.T) {
			a := fixture(t)
			blocked := filepath.Join(t.TempDir(), "file")
			if err := os.WriteFile(blocked, []byte("block"), 0600); err != nil {
				t.Fatal(err)
			}
			a.Store.Root = blocked
			a.busy = true
			a.partial = session.Message{Role: "assistant", Content: "retain this"}
			if cmd := a.runCommand(alias); cmd != nil || !strings.Contains(a.notice, "session remains open") {
				t.Fatal("quit ignored persistence failure")
			}
			select {
			case <-a.done:
				t.Fatal("failed save closed the application")
			default:
			}
			if len(a.Session.Messages) != 0 || a.partial.Content != "retain this" || a.partial.Interrupted {
				t.Fatal("failed quit mutated the live reply")
			}
		})
	}
}
