package ui

import (
	"errors"
	"strings"
	"unicode"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/erdajt/wraith/internal/terminal"
)

// terminalKey routes shell input, staged-command actions and scrollback navigation.
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

		if handled, cmd := a.terminalReaderKey(index, k); handled {
			return cmd
		}

		if p.PasteID != "" {
			if key == "esc" || key == "ctrl+c" {
				p.PTY.CancelPaste(p.PasteID)
				p.PasteCancelled = true
				a.notice = "insertion cancellation requested · awaiting shell acknowledgement"
				if key == "ctrl+c" {
					if err := p.PTY.Write([]byte{3}); err != nil {
						a.notice = err.Error()
					}
				}
			} else {
				a.notice = "waiting for insertion acknowledgement · ctrl+c interrupts; /terminal-restart resets the shell"
			}
			return nil
		}
		if p.Pending != "" {
			if key == "enter" {
				if p.PTY.Pastes != nil {
					return a.insertSaved(index)
				}
				data, err := stagedInput(p.Pending, p.Screen.Bracketed)
				if err != nil {
					a.notice = err.Error()
					return nil
				}
				if err := p.PTY.Write(data); err != nil {
					p.PasteID = "manual-write-error"
					a.notice = err.Error() + " · insertion outcome unknown; restart the shell before retrying"
				} else {
					a.notice = "command inserted into shell; review it, then press enter to execute"
					p.Pending = ""
				}
			} else if key == "ctrl+c" {
				if err := p.PTY.Write([]byte{3}); err != nil {
					a.notice = err.Error()
				} else {
					a.notice = "shell input interrupted · enter retries insertion; snippet retained"
				}
			} else if key == "esc" {
				p.Pending = ""
				a.notice = "staged command dismissed"
			} else if key == "r" {
				return a.approve(proposal{Command: p.Pending, Text: p.Pending, Language: "sh", Source: "edited-block"})
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

// stagedInput encodes a reviewable paste, rejecting control bytes and unsafe multiline staging.
func stagedInput(command string, bracketed bool) ([]byte, error) {
	for _, char := range command {
		if unicode.IsControl(char) && char != '\n' && char != '\t' {
			return nil, errors.New("command contains terminal control bytes; edit before staging")
		}
	}

	if bracketed {
		return []byte("\x1b[200~" + command + "\x1b[201~"), nil
	}
	if strings.Contains(command, "\n") {
		return nil, errors.New("shell has no bracketed paste; use the block run action for multiline commands")
	}
	return []byte(command), nil
}
