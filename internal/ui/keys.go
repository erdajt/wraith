package ui

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"
)

func (a *App) key(k tea.KeyMsg) tea.Cmd {
	if k.Type == tea.KeyRunes && len(k.Runes) > 1 && !k.Paste && !k.Alt && (a.focus == "chat" || a.Prompt.Mode == "normal") {

		for i, r := range k.Runes {
			if cmd := a.key(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}}); cmd != nil {
				if i == len(k.Runes)-1 {
					return cmd
				}
				rest := append([]rune(nil), k.Runes[i+1:]...)
				return tea.Sequence(cmd, func() tea.Msg { return tea.KeyMsg{Type: tea.KeyRunes, Runes: rest} })
			}
		}
		return nil
	}
	key := k.String()
	if key == a.Config.Keys["quit"] || (key == "q" && a.chatMotionPending == "" && a.Prompt.Pending == "" && a.popup == nil && !a.commandMode && !a.searchMode && (a.focus == "chat" || (a.focus == "prompt" && a.Prompt.Mode == "normal"))) {
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
	if a.chatMotionPending == "" && a.Prompt.Pending == "" && !a.commandMode && !a.searchMode && (a.focus == "chat" || a.Prompt.Mode == "normal") && (key == a.Config.Leader || a.Config.Leader == "space" && key == " ") {
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
			a.ensureChatCursor()
		}
		return nil
	}
	if key == a.Config.Keys["editor"] {
		if a.focus == "chat" {
			return a.editResponse()
		}
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
		return a.chatKey(k)
	}
	text := ""
	if k.Type == tea.KeyRunes || k.Type == tea.KeySpace {
		text = string(k.Runes)
	}
	pending, mode := a.Prompt.Pending, a.Prompt.Mode
	a.Prompt.Key(key, text)
	if a.Prompt.Pending == "" && len(a.Prompt.Register) > 0 && (key == "Y" && mode == "normal" || strings.HasPrefix(pending, "y") || key == "y" && strings.HasPrefix(mode, "visual")) {
		return a.copyText(string(a.Prompt.Register), a.Prompt.Linewise)
	}
	a.completionQuery = strings.TrimPrefix(a.Prompt.String(), "/")
	a.completionIndex = 0
	return nil
}
