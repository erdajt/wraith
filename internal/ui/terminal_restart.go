package ui

import (
	"errors"
	"fmt"
	"io"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/erdajt/wraith/internal/terminal"
)

type restartTerminalMsg struct {
	Index int
	PTY   *terminal.PTY
}

func (a *App) terminalExited(m terminalMsg) tea.Cmd {
	p := a.panes[m.Index]
	if time.Since(p.Started) > 2*time.Second {
		p.Restarts = 0
	}
	if (m.Event.Err != nil && !errors.Is(m.Event.Err, io.EOF)) || p.Restarts >= 3 {
		p.Err = "shell stopped · /terminal-restart " + fmt.Sprint(m.Index+1)
		return nil
	}
	p.Err = "restarting shell…"
	return tea.Tick(150*time.Millisecond, func(time.Time) tea.Msg { return restartTerminalMsg{m.Index, m.PTY} })
}

func (a *App) restartExitedTerminal(m restartTerminalMsg) tea.Cmd {
	select {
	case <-a.done:
		return nil
	default:
	}
	p := a.panes[m.Index]
	if p == nil || p.PTY != m.PTY {
		return nil
	}
	focus, visible, attempts := a.focus, p.Visible, p.Restarts+1
	p.PTY.Close()
	a.panes[m.Index] = nil
	cmd := a.openTerminal(m.Index)
	if replacement := a.panes[m.Index]; replacement != nil {
		replacement.Visible, replacement.Restarts = visible, attempts
	}
	a.focus = focus
	return cmd
}
