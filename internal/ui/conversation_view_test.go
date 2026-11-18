package ui

import (
	"strings"
	"testing"

	"github.com/erdajt/wraith/internal/session"
)

func TestConversationAnchorSurvivesRepeatedTextAndWideReflow(t *testing.T) {
	a := fixture(t)
	a.Session.Messages = []session.Message{{Role: "assistant", Content: "repeated\n" + strings.Repeat("界ab", 25) + "\nrepeated"}}
	lines := a.transcript(45)
	for i, row := range lines {
		if row.Section == "body" && row.Source == 1 && row.Cell > 0 {
			a.chatCursor, a.chatColumn, a.offset = i, 4, i
			break
		}
	}
	a.follow = false
	want := anchorAt(lines, a.chatCursor, a.chatColumn)
	a.visualMode, a.visualRow, a.visualColumn = "visual", a.chatCursor, a.chatColumn
	a.transcript(18)
	got := anchorAt(a.rows, a.chatCursor, a.chatColumn)
	if got != want || a.visualRow != a.chatCursor || a.visualColumn != a.chatColumn {
		t.Fatalf("anchor changed across wrapping: %+v -> %+v", want, got)
	}
	a.transcript(90)
	if got := anchorAt(a.rows, a.chatCursor, a.chatColumn); got != want {
		t.Fatalf("anchor changed on expansion: %+v", got)
	}
}

func TestConversationViewsRemainIndependentWhenThinkingChanges(t *testing.T) {
	a := fixture(t)
	a.Session.Messages = []session.Message{{Role: "assistant", Thinking: strings.Repeat("reasoning ", 40), Content: "first\nsecond\nthird"}}
	a.showThinking = false
	other := &conversationView{chatCursor: -1, search: "second", follow: true}
	lines := a.windowTranscript(other, 24, false)
	for i, row := range lines {
		if row.Section == "body" && row.Source == 1 {
			other.chatCursor, other.offset, other.follow = i, i, false
		}
	}
	want := anchorAt(lines, other.chatCursor, 0)
	a.showThinking = true
	a.windowTranscript(other, 24, false)
	if got := anchorAt(other.rows, other.chatCursor, 0); got != want {
		t.Fatalf("thinking expansion lost content: %+v -> %+v", want, got)
	}
	if other.search != "second" || a.search != "" || a.chatCursor != -1 {
		t.Fatal("view rendering changed another window's navigation")
	}
	a.Session.Messages = nil
	a.windowTranscript(other, 24, false)
	if other.chatCursor != 0 || other.offset != 0 {
		t.Fatal("empty buffer did not clamp navigation")
	}
}
