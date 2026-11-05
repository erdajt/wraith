package ui

import (
	"strings"
	"testing"

	"github.com/erdajt/wraith/internal/contextual"
	"github.com/erdajt/wraith/internal/terminal"
)

func TestTerminalEvidenceRequiresSharingAndDeduplicatesSnapshots(t *testing.T) {
	a := fixture(t)
	a.loading = true
	s := terminal.NewScreen(40, 4, 50)
	s.Feed([]byte("private terminal output"))
	a.panes[0] = &pane{Screen: s, Reader: newReader(terminalText(s))}
	a.terminalReaderKey(0, key("Y"))
	if len(a.Session.Messages) != 0 {
		t.Fatal("copy shared terminal output")
	}
	a.terminalReaderKey(0, key(a.Config.Keys["output_share"]))
	if len(a.Session.Messages) != 1 || !strings.Contains(a.Session.Messages[0].Content, "private terminal output") {
		t.Fatal("share lost terminal evidence")
	}
	if err := a.syncTerminalEvidence(); err != nil {
		t.Fatal(err)
	}
	if len(a.Session.Messages) != 1 {
		t.Fatal("unchanged terminal snapshot duplicated")
	}
	s.Feed([]byte(" changed"))
	if err := a.syncTerminalEvidence(); err != nil {
		t.Fatal(err)
	}
	if len(a.Session.Messages) != 2 {
		t.Fatal("changed snapshot was not shared")
	}
}

func TestAutomaticTerminalEvidenceOffersCompactionBeforeRequest(t *testing.T) {
	a := fixture(t)
	a.Config.Execution.AutoShare = true
	a.Prompt.Set("inspect the terminal")
	a.Config.Context = contextual.ChatUsed(a.Session, a.Config, a.Instructions) + a.Config.Predict + 180
	s := terminal.NewScreen(80, 10, 100)
	s.Feed([]byte(strings.Repeat("evidence ", 200)))
	a.panes[0] = &pane{Screen: s}
	a.submit()
	if a.busy || a.popup == nil || a.popup.Kind != "overflow" || !a.pendingAgent {
		t.Fatal("shared terminal evidence bypassed overflow handling")
	}
	if len(a.Session.Messages) != 2 || !strings.Contains(a.Session.Messages[1].Content, "terminal pane") {
		t.Fatal("prompt or terminal evidence was lost")
	}
}
