package ui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/erdajt/wraith/internal/session"
)

func TestCountedTranscriptMotions(t *testing.T) {
	a := fixture(t)
	a.Session.Messages = []session.Message{{Role: "assistant", Content: strings.Repeat("line\n", 300)}}
	a.focus, a.follow, a.offset = "chat", false, 150
	for _, step := range []struct {
		keys   string
		offset int
	}{{"10k", 140}, {"1k", 139}, {"100k", 39}, {"10j", 49}, {"999999j", 0}} {
		a.key(key(step.keys))
		want := step.offset
		if step.keys == "999999j" {
			want = len(a.transcript(max(10, a.mainWidth()-5))) - a.bodyHeight()
		}
		if a.offset != want {
			t.Fatalf("%s: got %d, want %d", step.keys, a.offset, want)
		}
	}
	a.key(key("gg"))
	if a.offset != 0 || a.follow {
		t.Fatal("batched gg did not reach the beginning")
	}
}

func TestNormalQuitDoesNotConsumeInsertOrShellInput(t *testing.T) {
	a := fixture(t)
	a.key(key("q"))
	if a.Prompt.String() != "q" {
		t.Fatal("insert q was consumed")
	}
	a.Prompt.Mode = "normal"
	cmd := a.key(key("q"))
	if cmd == nil {
		t.Fatal("normal q did not quit")
	}
	if _, ok := cmd().(tea.QuitMsg); !ok {
		t.Fatal("expected quit message")
	}
}

func TestBatchedNormalMotionsDoNotInsertDigits(t *testing.T) {
	a := fixture(t)
	a.Prompt.Set("one\ntwo\nthree")
	a.Prompt.Mode = "normal"
	a.key(key("10k"))
	if a.Prompt.String() != "one\ntwo\nthree" {
		t.Fatal("motion inserted text")
	}
}

func TestSlashSearchFollowsFocusedSurface(t *testing.T) {
	a := fixture(t)
	a.Prompt.Set("alpha beta alpha")
	a.Prompt.Mode, a.Prompt.Pos = "normal", 0
	a.key(key("/"))
	a.key(key("beta"))
	a.key(tea.KeyMsg{Type: tea.KeyEnter})
	if a.Prompt.Pos != 6 || a.Prompt.Search != "beta" || a.search != "" {
		t.Fatal("prompt search escaped its buffer")
	}
	a.Session.Messages = []session.Message{{Role: "user", Content: "needle"}, {Role: "assistant", Content: strings.Repeat("answer\n", 50)}}
	a.focus = "chat"
	a.key(key("/"))
	a.key(key("needle"))
	a.key(tea.KeyMsg{Type: tea.KeyEnter})
	if a.search != "needle" || a.Prompt.Search != "beta" || a.follow {
		t.Fatal("transcript search changed prompt search")
	}
}
