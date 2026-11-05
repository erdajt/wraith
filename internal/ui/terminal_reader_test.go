package ui

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/erdajt/wraith/internal/terminal"
)

func TestTerminalCopyModeSnapshotsSearchesAndYanks(t *testing.T) {
	a := fixture(t)
	a.Config.Clipboard = nil
	s := terminal.NewScreen(40, 4, 50)
	s.Feed([]byte("first\r\ncall(a, bee)\r\nlast"))
	a.panes[1] = &pane{Screen: s}
	a.terminalReaderKey(1, tea.KeyMsg{Type: tea.KeyEsc, Alt: true})
	a.terminalReaderKey(1, tea.KeyMsg{Type: tea.KeyEsc})
	r := a.panes[1].Reader
	if r == nil {
		t.Fatal("triple escape did not enter copy mode")
	}
	a.terminalReaderKey(1, key("/call"))
	a.terminalReaderKey(1, tea.KeyMsg{Type: tea.KeyEnter})
	a.terminalReaderKey(1, key("yf,"))
	if string(a.Prompt.Register) != "call(a," {
		t.Fatalf("copied %q", a.Prompt.Register)
	}
	s.Feed([]byte(" changed"))
	if r.Buffer.String() != "first\ncall(a, bee)\nlast" {
		t.Fatal("live output changed snapshot")
	}
	a.terminalReaderKey(1, key("i"))
	if a.panes[1].Reader != nil {
		t.Fatal("copy mode did not exit")
	}
}
