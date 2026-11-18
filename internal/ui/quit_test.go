package ui

import (
	"fmt"
	"net/http"
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

func TestQuitClosesOneViewUnlessAllWasRequested(t *testing.T) {
	for _, command := range []string{"q", "qa"} {
		t.Run(command, func(t *testing.T) {
			a := fixture(t)
			a.width, a.height = 120, 40
			a.windowAction("v")
			cmd := a.runCommand(command)
			if command == "q" {
				if cmd != nil || len(a.windows) != 1 {
					t.Fatal("q did not close just the selected split")
				}
				cmd = a.runCommand("q")
			}
			if cmd == nil {
				t.Fatal("last view or qa did not exit")
			}
			if _, ok := cmd().(tea.QuitMsg); !ok {
				t.Fatal("missing quit message")
			}
		})
	}
}

func TestUnsentDraftRequiresSaveOrExplicitDiscard(t *testing.T) {
	for _, command := range []string{"q", "qa", "q!", "qa!", "wq"} {
		t.Run(command, func(t *testing.T) {
			a := fixture(t)
			a.width, a.height = 120, 40
			a.windowAction("v")
			a.Prompt.Set("  retain exact draft\n")
			a.Session.Draft = "older saved draft"
			cmd := a.runCommand(command)
			if command == "q" || command == "qa" {
				if cmd != nil || len(a.windows) != 2 || !strings.Contains(a.notice, "unsent draft") || a.Prompt.String() != "  retain exact draft\n" {
					t.Fatal("unguarded quit discarded a draft")
				}
				return
			}
			if (cmd == nil) != (command == "q!") {
				t.Fatal("wrong close scope")
			}
			saved, err := a.Store.Load(a.Session.ID)
			if err != nil {
				t.Fatal(err)
			}
			want := ""
			if command == "wq" {
				want = "  retain exact draft\n"
			}
			if saved.Draft != want || len(saved.Messages) != 0 {
				t.Fatalf("draft persisted as conversation evidence: %+v", saved)
			}
			if command == "wq" {
				resumed, err := New(a.Config, a.Paths, saved, false)
				if err != nil {
					t.Fatal(err)
				}
				defer resumed.Close()
				if resumed.Prompt.String() != want {
					t.Fatal("startup did not restore saved draft")
				}
				resumed.Prompt.Set("different")
				resumed.resume(saved.ID)
				if resumed.Prompt.String() != want {
					t.Fatal("in-app resume did not restore saved draft")
				}
			}
		})
	}
}

func TestDraftSaveFailurePreservesLiveInputAndViews(t *testing.T) {
	for _, command := range []string{"q!", "qa!", "wq"} {
		t.Run(command, func(t *testing.T) {
			a := fixture(t)
			a.width, a.height = 120, 40
			a.windowAction("v")
			a.Prompt.Set("keep me")
			blocked := filepath.Join(t.TempDir(), "blocked")
			if err := os.WriteFile(blocked, nil, 0600); err != nil {
				t.Fatal(err)
			}
			a.Store.Root = blocked
			if cmd := a.runCommand(command); cmd != nil || len(a.windows) != 2 || a.Prompt.String() != "keep me" {
				t.Fatal("save failure discarded draft or closed a view")
			}
		})
	}
}

func TestResumePickerConsumesStartupDraftOnlyOnce(t *testing.T) {
	for _, cancel := range []bool{false, true} {
		t.Run(map[bool]string{false: "select", true: "cancel"}[cancel], func(t *testing.T) {
			a := fixture(t)
			a.Session.Draft = "saved draft"
			if err := a.Store.Save(a.Session); err != nil {
				t.Fatal(err)
			}
			a.OpenSessions("startup prompt")
			if cancel {
				a.menuKey(tea.KeyMsg{Type: tea.KeyEsc})
				a.OpenSessions()
			}
			a.menuKey(tea.KeyMsg{Type: tea.KeyEnter})
			want := "startup prompt"
			if cancel {
				want = "saved draft"
			}
			if a.Prompt.String() != want || a.resumePrompt != nil {
				t.Fatalf("picker restored %q, want %q", a.Prompt.String(), want)
			}
			a.resume(a.Session.ID)
			if a.Prompt.String() != "saved draft" {
				t.Fatal("startup override leaked into later resumes")
			}
		})
	}
}

func TestCompactionCannotRestoreDiscardedDraft(t *testing.T) {
	a := fixture(t)
	a.width, a.height = 120, 40
	a.windowAction("v")
	a.Session.Draft = "previously saved"
	a.Prompt.Set(a.Session.Draft)
	candidate := a.Session.Clone()
	a.busy, a.compacting = true, true
	a.runCommand("q!")
	a.finishCompact(compactMsg{Session: candidate, Archive: "original"})
	saved, err := a.Store.Load(a.Session.ID)
	if err != nil || saved.Draft != "" || a.Prompt.String() != "" {
		t.Fatalf("compaction restored discarded draft: %q, %v", saved.Draft, err)
	}
}

func TestCloseByIDRetainsFocusWhenDraftBlocksClose(t *testing.T) {
	a := fixture(t)
	a.width, a.height = 120, 40
	original := a.windowID
	a.windowAction("v")
	focused := a.windowID
	a.Prompt.Set("keep me")
	a.runAction("window.close_id", fmt.Sprint(original))
	if a.windowID != focused || len(a.windows) != 2 || !strings.Contains(a.notice, "unsent draft") {
		t.Fatal("lua close bypassed guard or changed focus")
	}
}

func TestWriteAliasesCommitDraftOnlyWithMessage(t *testing.T) {
	for _, command := range []string{"w", "w!"} {
		t.Run(command, func(t *testing.T) {
			a := fixture(t)
			a.Session.Draft = "  saved draft\n"
			a.Prompt.Set(a.Session.Draft)
			a.Client.HTTP.Transport = transport(func(*http.Request) (*http.Response, error) {
				return response(`{"done":true,"message":{"content":"received"}}`), nil
			})
			a.runCommand(command)
			drain(t, a)
			saved, err := a.Store.Load(a.Session.ID)
			if err != nil || saved.Draft != "" || len(saved.Messages) != 2 || saved.Messages[0].Content != "saved draft" {
				t.Fatalf("send did not replace saved draft: %+v, %v", saved, err)
			}
			if a.Prompt.String() != "" {
				t.Fatal("sent draft remained in prompt")
			}
		})
	}
}

func TestWriteFailureRetainsExactSavedAndLiveDraft(t *testing.T) {
	a := fixture(t)
	a.Session.Draft = "saved draft"
	a.Prompt.Set("  edited draft\n")
	blocked := filepath.Join(t.TempDir(), "blocked")
	if err := os.WriteFile(blocked, nil, 0600); err != nil {
		t.Fatal(err)
	}
	a.Store.Root = blocked
	a.runCommand("w!")
	if a.Session.Draft != "saved draft" || a.Prompt.String() != "  edited draft\n" || len(a.Session.Messages) != 0 || a.busy {
		t.Fatal("failed send altered saved draft or current input")
	}
}

func TestSessionTransitionsIsolateUndoAndRecalledDrafts(t *testing.T) {
	for _, transition := range []string{"resume", "fork", "clear"} {
		t.Run(transition, func(t *testing.T) {
			a := fixture(t)
			a.Prompt.Set("previous session draft")
			a.inputHistory = []string{"submitted prompt"}
			a.historyKey(tea.KeyMsg{Type: tea.KeyUp})
			want := ""
			switch transition {
			case "resume":
				s := session.New(a.Config.Model)
				s.Directory, s.Provider, s.Draft = a.Directory, a.Config.Provider, "resumed draft"
				if err := a.Store.Save(s); err != nil {
					t.Fatal(err)
				}
				a.resume(s.ID)
				want = s.Draft
			case "fork":
				a.Session.Messages = []session.Message{{Role: "user", Content: "forked draft"}}
				a.forkIndex = 0
				a.forkPrompt()
				want = "forked draft"
			case "clear":
				a.sessionCommand("clear", "")
			}

			a.Prompt.Mode = "insert"
			a.historyKey(tea.KeyMsg{Type: tea.KeyDown})
			a.Prompt.Key("esc", "")
			a.Prompt.Key("u", "u")
			if a.Prompt.String() != want || a.inputIndex != -1 || a.inputDraft != "" {
				t.Fatalf("transition retained another draft: %q", a.Prompt.String())
			}
			a.Prompt.Key("A", "A")
			a.Prompt.Key("runes", " edited")
			a.Prompt.Key("esc", "")
			a.Prompt.Key("u", "u")
			if a.Prompt.String() != want {
				t.Fatal("local edit could not be undone")
			}
		})
	}
}

func TestFocusChangesCancelOutgoingPromptOperators(t *testing.T) {
	for _, action := range []string{"focus.next", "focus.up", "window.split_vertical"} {
		t.Run(action, func(t *testing.T) {
			a := fixture(t)
			a.Prompt.Mode = "normal"
			a.feedInput("2d3")
			a.runAction(action, "")
			if a.focus != "chat" || a.Prompt.Pending != "" || a.Prompt.Count != "" || a.Prompt.OperatorCount != "" {
				t.Fatal("outgoing prompt operator survived focus change")
			}
			cmd := a.key(key("q"))
			if action == "window.split_vertical" {
				if len(a.windows) != 1 {
					t.Fatal("inactive prompt prevented closing split")
				}
			} else if cmd == nil {
				t.Fatal("inactive prompt prevented quit")
			}
		})
	}
}
