package ui

import (
	"errors"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/erdajt/wraith/internal/terminal"
)

func (a *App) terminalKey(k tea.KeyMsg) tea.Cmd {
	key := k.String()
	if strings.HasPrefix(a.focus, "terminal") {
		index := 0
		if a.focus == "terminal1" {
			index = 1
		}
		p := a.panes[index]
		if key == a.Config.Keys["terminal_unfocus"] {
			a.focus = "prompt"
			return nil
		}
		if p == nil || p.PTY == nil {
			return nil
		}
		if p.Pending != "" {
			if key == "enter" {
				data, err := stagedInput(p.Pending, p.Screen.Bracketed)
				if err != nil {
					a.notice = err.Error()
					return nil
				}
				if err := p.PTY.Write(data); err != nil {
					a.notice = err.Error()
				} else {
					a.notice = "command inserted into shell; review it, then press enter to execute"
					p.Pending = ""
				}
			} else if key == "esc" {
				p.Pending = ""
				a.notice = "staged command dismissed"
			} else if key == "e" {
				return a.editStaged(index)
			}
			return nil
		}
		if key == "shift+pgup" {
			p.Screen.Scroll(p.Screen.Height / 2)
			return nil
		}
		if key == "shift+pgdown" {
			p.Screen.Scroll(-p.Screen.Height / 2)
			return nil
		}
		p.Screen.Offset = 0
		if err := p.PTY.Write(terminal.Key(k, p.Screen.Application, p.Screen.Bracketed)); err != nil {
			a.notice = err.Error()
		}
		return nil
	}
	return nil
}

func stagedInput(command string, bracketed bool) ([]byte, error) {
	if strings.ContainsAny(command, "\x00\x1b\r") {
		return nil, errors.New("command contains terminal control bytes; edit before staging")
	}
	if bracketed {
		return []byte("\x1b[200~" + command + "\x1b[201~"), nil
	}
	if strings.Contains(command, "\n") {
		return nil, errors.New("shell has no bracketed paste; use the block run action for multiline commands")
	}
	return []byte(command), nil
}
