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

// terminalExited schedules a shell restart unless an error or rapid-restart limit blocks it.
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

// restartExitedTerminal replaces an exited shell while preserving pane visibility and application focus.
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
	p.PasteID, p.PasteCancelled = "", false
	a.panes[m.Index] = nil
	cmd := a.openTerminal(m.Index)
	if replacement := a.panes[m.Index]; replacement != nil {
		replacement.Visible, replacement.Restarts = visible, attempts
		replacement.Pending, replacement.Path, replacement.Language = p.Pending, p.Path, p.Language
		if replacement.Pending != "" {
			a.notice = "shell restarted · saved command retained for manual insertion"
		}
	} else {
		p.Err = a.notice + " · /terminal-restart " + fmt.Sprint(m.Index+1)
		p.Restarts = 3
		a.panes[m.Index] = p
		if p.Path != "" {
			a.notice += " · saved snippet at " + p.Path
		}
	}
	a.focus = focus
	return cmd
}
