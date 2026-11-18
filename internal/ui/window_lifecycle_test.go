package ui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/erdajt/wraith/internal/session"
)

func TestWindowMouseCoordinatesAndSideShellBounds(t *testing.T) {
	a := fixture(t)
	a.width, a.height, a.focus = 140, 45, "chat"
	a.Session.Messages = []session.Message{{Role: "assistant", Content: strings.Repeat("row\n", 40)}}
	a.windowAction("v")
	a.windowAction("s")
	a.panes[1] = &pane{Visible: true}
	places := a.windowPlaces()
	for _, place := range places {
		if place.x+place.width > a.mainWidth() || place.y+place.height > 3+a.bodyHeight() {
			t.Fatal("conversation crossed shared prompt or side shell")
		}
		a.mouse(tea.MouseMsg{X: place.x + 3, Y: place.y + 1, Action: tea.MouseActionPress, Button: tea.MouseButtonLeft})
		if a.windowID != place.id || a.chatColumn != 1 || a.follow {
			t.Fatalf("mouse did not focus leaf %d", place.id)
		}
	}
}

func TestWindowAnchorsFollowRetainedMessagesAcrossCompaction(t *testing.T) {
	a := fixture(t)
	a.width, a.height = 120, 45
	a.Session.Messages = []session.Message{{Role: "user", Content: "old"}, {Role: "assistant", Content: "discarded"}, {Role: "user", Content: "retained"}, {Role: "assistant", Content: "answer"}}
	a.windowAction("v")
	a.selected, a.search = 2, "retained"
	a.jumpSelected()
	a.windowAction("h")
	a.selected = 0
	a.jumpSelected()
	a.visualMode, a.visualRow = "visual", a.chatCursor
	before := a.Session.Messages
	next := before[2:]
	a.remapConversation(before, next)
	a.Session.Messages = next
	a.View()
	if a.visualMode != "" || a.rows[a.chatCursor].Message != 0 {
		t.Fatal("removed selection did not move to nearest retained message")
	}
	a.windowAction("l")
	if a.search != "retained" || a.rows[a.chatCursor].Message != 0 || a.rows[a.chatCursor].Section != "header" {
		t.Fatal("retained message anchor was lost")
	}
}

func TestFrozenWindowDoesNotFollowStreamOrBlockAgent(t *testing.T) {
	a := fixture(t)
	a.width, a.height = 120, 45
	a.Session.Messages = []session.Message{{Role: "assistant", Content: strings.Repeat("old\n", 50)}}
	a.windowAction("v")
	frozen := a.conversationView
	a.ensureChatCursor()
	a.chatCursor, a.offset, a.search = 1, 0, "old"
	a.visualMode, a.visualRow = "visual", 1
	a.windowAction("h")
	a.busy = true
	a.partial = session.Message{Role: "assistant", Content: strings.Repeat("new\n", 50)}
	a.View()
	if frozen.follow || frozen.chatCursor != 1 || frozen.offset != 0 || frozen.search != "old" {
		t.Fatal("stream changed a frozen view")
	}
	a.busy = false
	if !a.canContinue() {
		t.Fatal("hidden selection blocked agent continuation")
	}
}
