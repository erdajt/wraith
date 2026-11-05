package ui

import (
	"fmt"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/erdajt/wraith/internal/terminal"
	"os"
	"path/filepath"
	"strings"
)

func (a *App) toggleTerminal(index int) tea.Cmd {
	p := a.panes[index]
	if p != nil && p.Visible {
		p.Visible = false
		a.focus = "prompt"
		return nil
	}
	return a.openTerminal(index)
}

func (a *App) openTerminal(index int) tea.Cmd {
	if a.demo {
		a.notice = "shells are disabled in demo mode"
		return nil
	}
	if p := a.panes[index]; p != nil {
		p.Visible = true
		a.focus = fmt.Sprintf("terminal%d", index)
		a.resizePanes()
		return nil
	}
	a.panes[index] = &pane{Visible: true}
	w, h := a.paneSize(index)
	dir := a.Config.Terminal.Directory
	if strings.HasPrefix(dir, "~/") {
		home, _ := os.UserHomeDir()
		dir = filepath.Join(home, dir[2:])
	}
	p, err := terminal.Start(a.Config.Shell, dir, w-2, h-2)
	if err != nil {
		a.panes[index] = nil
		a.notice = "shell: " + err.Error()
		return nil
	}
	a.panes[index].PTY = p
	a.panes[index].Screen = terminal.NewScreen(w-2, h-2, a.Config.Terminal.Scrollback)
	a.focus = fmt.Sprintf("terminal%d", index)
	a.notice = "shell active · " + a.Config.Keys["terminal_unfocus"] + " returns focus · shift+pgup scrolls"
	return a.readTerminal(index)
}

func (a *App) readTerminal(index int) tea.Cmd {
	p := a.panes[index].PTY
	return func() tea.Msg {
		select {
		case event, ok := <-p.Events:
			return terminalMsg{Index: index, PTY: p, Event: event, Open: ok}
		case <-a.done:
			return nil
		}
	}
}

func (a *App) resizePanes() {
	for i, p := range a.panes {
		if p == nil || p.PTY == nil {
			continue
		}
		w, h := a.paneSize(i)
		p.Screen.Resize(w-2, h-2)
		if err := p.PTY.Resize(w-2, h-2); err != nil && p.Err == "" {
			a.notice = err.Error()
		}
	}
}
