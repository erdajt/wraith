package ui

import (
	"github.com/charmbracelet/x/ansi"
	"github.com/erdajt/wraith/internal/terminal"
	"strings"
	"testing"
)

func TestMultilineStagingCannotExecuteOnInsertion(t *testing.T) {
	command := "printf first\nprintf second"
	if _, err := stagedInput(command, false); err == nil {
		t.Fatal("unprotected multiline input accepted")
	}
	b, err := stagedInput(command, true)
	if err != nil || string(b) != "\x1b[200~"+command+"\x1b[201~" {
		t.Fatalf("bad bracketed input: %q %v", b, err)
	}
	for _, control := range []string{"\x1b[201~", "\r", "\x00"} {
		if _, err := stagedInput("printf x"+control, true); err == nil {
			t.Fatal("terminal control accepted")
		}
	}
}

func TestSideShellUsesHeightBelowHeader(t *testing.T) {
	a := fixture(t)
	w, h := a.paneSize(1)
	if h != a.height-3 {
		t.Fatalf("height %d, expected %d", h, a.height-3)
	}
	a.panes[1] = &pane{Visible: true, Screen: terminal.NewScreen(w-2, h-2, 0)}
	rows := strings.Split(ansi.Strip(a.View()), "\n")
	if !strings.HasSuffix(rows[len(rows)-1], "╯") {
		t.Fatal("side shell did not reach bottom row")
	}
}
