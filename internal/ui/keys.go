package ui

import (
	"strconv"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
)

func (a *App) key(k tea.KeyMsg) tea.Cmd {
	if k.Type == tea.KeyRunes && len(k.Runes) > 1 && !k.Paste && !k.Alt && (a.focus == "chat" || a.Prompt.Mode == "normal") {
		var commands []tea.Cmd
		for _, r := range k.Runes {
			if cmd := a.key(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}}); cmd != nil {
				commands = append(commands, cmd)
			}
		}
		return tea.Sequence(commands...)
	}
	key := k.String()
	if key == a.Config.Keys["quit"] || (key == "q" && a.popup == nil && !a.commandMode && !a.searchMode && (a.focus == "chat" || (a.focus == "prompt" && a.Prompt.Mode == "normal"))) {
		if a.busy && !a.compacting && (a.partial.Content != "" || a.partial.Thinking != "") {
			a.partial.Interrupted = true
			a.Session.Messages = append(a.Session.Messages, a.partial)
			if err := a.Store.Save(a.Session); err != nil {
				a.Session.Messages = a.Session.Messages[:len(a.Session.Messages)-1]
				a.notice = "save failed; session remains open: " + err.Error()
				return nil
			}
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
	if !a.commandMode && !a.searchMode && (a.focus == "chat" || a.Prompt.Mode == "normal") && (key == a.Config.Leader || a.Config.Leader == "space" && key == " ") {
		a.openLeader("")
		return nil
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
			if strings.TrimSpace(value) == "w" {
				return a.submit()
			}
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
	if cmd, handled := a.slashKey(k); handled {
		return cmd
	}
	if key == a.Config.Keys["focus"] {
		a.chatCount = ""
		a.chatG = false
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
		if key == a.Config.Keys["thinking_toggle"] {
			a.toggleThinking(a.selected)
			return nil
		}
		if len(key) == 1 && key[0] >= '0' && key[0] <= '9' {
			if len(a.chatCount) < 6 {
				a.chatCount += key
			}
			return nil
		}
		count, _ := strconv.Atoi(a.chatCount)
		count = max(1, count)
		a.chatCount = ""
		if key != "g" {
			a.chatG = false
		}
		limit := max(0, len(a.transcript(max(10, a.mainWidth()-5)))-a.bodyHeight())
		if a.follow {
			a.offset = limit
		}
		a.offset = min(a.offset, limit)
		defer func() { a.offset = min(a.offset, limit) }()
		switch key {
		case "j", "down":
			a.offset += count
			a.follow = false
		case "k", "up":
			a.offset = max(0, a.offset-count)
			a.follow = false
		case "ctrl+d", "pgdown":
			a.offset += count * max(1, a.bodyHeight()/2)
			a.follow = false
		case "ctrl+u", "pgup":
			a.offset = max(0, a.offset-count*max(1, a.bodyHeight()/2))
			a.follow = false
		case "g":
			if a.chatG {
				a.offset = 0
				a.follow = false
				a.chatG = false
			} else {
				a.chatG = true
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
		case "enter":
			return a.selectedBlocks()
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
	a.completionQuery = strings.TrimPrefix(a.Prompt.String(), "/")
	a.completionIndex = 0
	return nil
}
