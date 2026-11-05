package ui

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"
)

func (a *App) key(k tea.KeyMsg) tea.Cmd {
	key := k.String()
	if key == a.Config.Keys["quit"] {
		if a.busy && !a.compacting && (a.partial.Content != "" || a.partial.Thinking != "") {
			a.partial.Interrupted = true
			a.Session.Messages = append(a.Session.Messages, a.partial)
			_ = a.Store.Save(a.Session)
		}
		a.Close()
		return tea.Quit
	}
	if key == a.Config.Keys["terminal_overlay"] {
		return a.toggleTerminal(0)
	}
	if key == a.Config.Keys["terminal_split"] {
		return a.toggleTerminal(1)
	}
	if a.popup != nil {
		return a.menuKey(k)
	}
	if strings.HasPrefix(a.focus, "terminal") {
		return a.terminalKey(k)
	}
	if key == a.Config.Keys["cancel"] {
		if a.cancel != nil && a.busy {
			a.cancel()
			a.notice = "cancelling…"
		} else {
			a.commandMode = false
			a.searchMode = false
			a.Prompt.Key("esc", "")
		}
		return nil
	}
	if a.commandMode || a.searchMode {
		if key == "esc" {
			a.commandMode = false
			a.searchMode = false
			return nil
		}
		if key == "enter" {
			value := a.command
			a.command = ""
			if a.searchMode {
				a.search = value
				a.searchMode = false
				a.findNext(false)
				return nil
			}
			a.commandMode = false
			return a.runCommand(value)
		}
		if key == "backspace" {
			r := []rune(a.command)
			if len(r) > 0 {
				a.command = string(r[:len(r)-1])
			}
		} else if k.Type == tea.KeyRunes || k.Type == tea.KeySpace {
			a.command += string(k.Runes)
		}
		return nil
	}
	if key == a.Config.Keys["focus"] {
		if a.focus == "chat" {
			a.focus = "prompt"
		} else {
			a.focus = "chat"
		}
		return nil
	}
	if key == a.Config.Keys["editor"] {
		return a.edit("prompt", "", "", -1)
	}
	if key == a.Config.Keys["send"] {
		return a.submit()
	}
	if (a.focus == "chat" || a.Prompt.Mode == "normal") && key == ":" {
		a.commandMode = true
		a.command = ""
		return nil
	}
	if a.focus == "chat" {
		limit := max(0, len(a.transcript(max(10, a.mainWidth()-5)))-a.bodyHeight())
		if a.follow {
			a.offset = limit
		}
		a.offset = min(a.offset, limit)
		defer func() { a.offset = min(a.offset, limit) }()
		switch key {
		case "j", "down":
			a.offset++
			a.follow = false
		case "k", "up":
			a.offset = max(0, a.offset-1)
			a.follow = false
		case "ctrl+d", "pgdown":
			a.offset += max(1, a.bodyHeight()/2)
			a.follow = false
		case "ctrl+u", "pgup":
			a.offset = max(0, a.offset-max(1, a.bodyHeight()/2))
			a.follow = false
		case "g":
			if a.phase == "g" {
				a.offset = 0
				a.follow = false
				a.phase = ""
			} else {
				a.phase = "g"
			}
		case "G":
			a.follow = true
		case "]":
			a.selected = min(len(a.Session.Messages)-1, a.selected+1)
			a.jumpSelected()
		case "[":
			a.selected = max(0, a.selected-1)
			a.jumpSelected()
		case "/":
			a.searchMode = true
			a.command = ""
		case "n":
			a.findNext(false)
		case "N":
			a.findNext(true)
		case "e":
			return a.editResponse()
		case "b":
			return a.runCommand("blocks")
		case "i":
			a.focus = "prompt"
			a.Prompt.Mode = "insert"
		case "?":
			return a.runCommand("help")
		}
		return nil
	}
	text := ""
	if k.Type == tea.KeyRunes || k.Type == tea.KeySpace {
		text = string(k.Runes)
	}
	a.Prompt.Key(key, text)
	return nil
}
