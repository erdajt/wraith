package ui

import (
	"strings"
	"testing"

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
