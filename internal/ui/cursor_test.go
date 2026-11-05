package ui

import (
	"github.com/charmbracelet/x/ansi"
	"github.com/erdajt/wraith/internal/session"
	"testing"
)

func TestThemedCursorPreservesWideCharacters(t *testing.T) {
	a := fixture(t)
	for _, column := range []int{0, 1, 2, 3} {
		text := cursorText("界ab", column, a.Config.Palette())
		if ansi.Strip(text) != "界ab" || ansi.StringWidth(text) != 4 {
			t.Fatalf("cursor damaged text at %d: %q", column, text)
		}
	}
}

func TestConversationCursorSelectsMessageUnderIt(t *testing.T) {
	a := fixture(t)
	a.focus = "chat"
	a.follow = false
	a.Session.Messages = []session.Message{{Role: "user", Content: "first"}, {Role: "assistant", Content: "second"}}
	a.key(key("gg"))
	a.key(key("3j"))
	if a.selected != 1 || a.chatCursor != 3 {
		t.Fatalf("cursor selection: %d %d", a.selected, a.chatCursor)
	}
}
