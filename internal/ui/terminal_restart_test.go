package ui

import (
	"io"
	"testing"
	"time"

	"github.com/erdajt/wraith/internal/terminal"
)

func TestExitedShellRestartsAndKeepsFocus(t *testing.T) {
	a := fixture(t)
	a.Config.Shell = []string{"/bin/sh", "-i"}
	a.openTerminal(1)
	old := a.panes[1].PTY
	if err := old.Write([]byte("exit\n")); err != nil {
		t.Fatal(err)
	}
	deadline := time.After(3 * time.Second)
	for {
		select {
		case event, ok := <-old.Events:
			_, cmd := a.Update(terminalMsg{Index: 1, PTY: old, Event: event, Open: ok})
			if !ok || event.Err != nil {
				if cmd == nil {
					t.Fatal("exit did not schedule a restart")
				}
				a.Update(cmd())
				if a.panes[1].PTY == old || !a.panes[1].Visible || a.focus != "terminal1" {
					t.Fatal("shell replacement lost pane state")
				}
				return
			}
		case <-deadline:
			t.Fatal("shell did not exit")
		}
	}
}

func TestRapidShellFailuresStopRestarting(t *testing.T) {
	a := fixture(t)
	a.panes[0] = &pane{Started: time.Now(), Restarts: 3}
	if a.terminalExited(terminalMsg{Index: 0, Event: terminal.Event{Err: io.EOF}}) != nil {
		t.Fatal("restart loop was not stopped")
	}
}
