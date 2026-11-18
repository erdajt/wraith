package ui

import (
	"fmt"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/erdajt/wraith/internal/terminal"
)

type pasteMsg struct {
	Index int
	PTY   *terminal.PTY
	Event terminal.PasteEvent
	Open  bool
}

type pasteTimeoutMsg struct {
	Index int
	PTY   *terminal.PTY
	ID    string
}

// insertSaved queues one saved snippet while retaining manual recovery until acknowledgement.
func (a *App) insertSaved(index int) tea.Cmd {
	p := a.panes[index]
	if p == nil || p.Pending == "" || p.PasteID != "" {
		return nil
	}
	id, err := p.PTY.QueuePaste(p.Pending)
	if err != nil {
		a.notice = err.Error() + " · enter inserts manually"
		return nil
	}
	p.PasteID, p.PasteCancelled = id, false
	a.notice = "waiting for an empty shell prompt · esc cancels insertion"
	pty := p.PTY
	return tea.Tick(3*time.Second, func(time.Time) tea.Msg { return pasteTimeoutMsg{Index: index, PTY: pty, ID: id} })
}

// pasted applies acknowledgements only to the matching pane and saved payload.
func (a *App) pasted(m pasteMsg) tea.Cmd {
	p := a.panes[m.Index]
	if p == nil || p.PTY != m.PTY {
		return nil
	}
	if !m.Open {
		p.PTY.Pastes = nil
		if p.PasteID != "" {
			p.PTY.CancelPaste(p.PasteID)
			a.notice = "shell insertion outcome is unknown · restart the shell before reinserting; saved command retained"
		}
		return a.readTerminal(m.Index)
	}
	if m.Event.ID != p.PasteID {
		return a.readTerminal(m.Index)
	}
	p.PasteID = ""
	switch m.Event.Status {
	case "inserted":
		p.Pending = ""
		a.notice = "command inserted into shell; review it, then press enter to execute"
		if p.PasteCancelled {
			a.notice = "command was inserted before cancellation · inspect the shell line before continuing"
		}
	case "occupied":
		a.notice = "shell input is occupied · saved command retained; clear the line before inserting"
	case "cancelled":
		a.notice = "automatic insertion cancelled · saved command retained"
	default:
		a.notice = fmt.Sprintf("shell insertion failed (%s) · saved command retained", m.Event.Status)
	}
	return a.readTerminal(m.Index)
}
