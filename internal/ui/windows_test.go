package ui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/ansi"
	"github.com/erdajt/wraith/internal/session"
)

func TestWindowChordsCreateIndependentViewsOfSharedConversation(t *testing.T) {
	a := fixture(t)
	a.width, a.height, a.focus = 120, 45, "chat"
	a.Session.Messages = []session.Message{{Role: "assistant", Content: strings.Repeat("one\ntwo\nthree\n", 20)}}
	a.ensureChatCursor()
	a.search = "one"
	first := a.conversationView
	a.key(tea.KeyMsg{Type: tea.KeyCtrlW})
	a.key(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("v")})
	if len(a.windows) != 2 || a.conversationView == first || a.search != "one" {
		t.Fatal("split did not create an independent view at the same location")
	}
	a.search = "two"
	a.chatKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("G")})
	secondCursor := a.chatCursor
	a.windowAction("h")
	if a.conversationView != first || a.search != "one" || a.chatCursor == secondCursor {
		t.Fatal("navigation leaked between windows")
	}
	a.windowAction("s")
	if len(a.windows) != 3 {
		t.Fatal("nested split failed")
	}
	frame := strings.Split(a.View(), "\n")
	if len(frame) != a.height {
		t.Fatal("split changed frame height")
	}
	for _, line := range frame {
		if ansi.StringWidth(line) != a.width {
			t.Fatal("split overflowed frame width")
		}
	}
	a.windowAction("c")
	a.windowAction("o")
	if len(a.windows) != 1 || a.windowTree.id != a.windowID {
		t.Fatal("close/only failed to restore one window")
	}
}

func TestSplitsRespectInputAndMinimumGeometry(t *testing.T) {
	a := fixture(t)
	a.Prompt.Set("one two")
	a.Prompt.Pos = len(a.Prompt.Text)
	a.key(tea.KeyMsg{Type: tea.KeyCtrlW})
	if len(a.keyPending) > 0 || len(a.windows) != 1 {
		t.Fatal("insert-mode ctrl+w became a window command")
	}
	a.focus, a.width, a.height = "chat", 50, 18
	a.windowAction("s")
	if len(a.windows) != 1 || !strings.Contains(a.notice, "not enough space") {
		t.Fatal("horizontal split ignored minimum height")
	}
	a.windowAction("v")
	a.windowAction("v")
	if len(a.windows) != 2 {
		t.Fatal("vertical split ignored minimum width")
	}
	a.search, a.visualMode = "old", "visual"
	a.resetChatNavigation()
	for _, view := range a.windows {
		if view.search != "" || view.visualMode != "" || !view.follow || view.chatCursor != -1 {
			t.Fatal("session reset left stale window state")
		}
	}
}

func TestWindowChordsReachNormalAndPendingConversationModes(t *testing.T) {
	for _, mode := range []string{"prompt", "chat", "operator"} {
		for _, last := range []tea.KeyMsg{{Type: tea.KeyRunes, Runes: []rune{'v'}}, {Type: tea.KeyCtrlV}} {
			t.Run(mode+last.String(), func(t *testing.T) {
				a := fixture(t)
				a.width, a.height = 120, 40
				a.Prompt.Mode = "normal"
				if mode != "prompt" {
					a.focus = "chat"
				}
				if mode == "operator" {
					a.chatMotionPending = "y"
				}
				a.key(tea.KeyMsg{Type: tea.KeyCtrlW})
				a.key(last)
				if len(a.windows) != 2 || a.focus != "chat" {
					t.Fatalf("split chord lost in %s: %s", mode, a.notice)
				}
			})
		}
	}
}

func TestWindowActionsCancelPendingConversationOperators(t *testing.T) {
	for _, action := range []string{">", "=", "o", "+"} {
		t.Run(action, func(t *testing.T) {
			a := fixture(t)
			a.width, a.height, a.focus = 120, 40, "chat"
			a.windowAction("v")
			a.chatMotionPending, a.chatMotionCount, a.chatCount = "y", "2", "3"
			a.key(tea.KeyMsg{Type: tea.KeyCtrlW})
			a.key(key(action))
			if a.chatMotionPending != "" || a.chatMotionCount != "" || a.chatCount != "" {
				t.Fatal("window action retained an unfinished text operator")
			}
		})
	}
}
