package ui

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"
)

// keyDefault routes unmatched input to the focused vim editor, picker or terminal.
func (a *App) keyDefault(k tea.KeyMsg) tea.Cmd {
	key := k.String()
	if a.pluginDialog != nil {
		return a.pluginDialogKey(k)
	}
	if view := a.focusedSurfaceView(); view != nil && !a.commandMode {
		b := view.reader.Buffer
		if b.Mode == "normal" && b.Pending == "" && (key == a.Config.Leader || a.Config.Leader == "space" && key == " ") {
			a.openLeader("")
			return nil
		}
		if b.Mode == "normal" && b.Pending == "" && key == ":" {
			a.commandMode, a.command = true, ""
			return nil
		}
		if b.Mode == "normal" && b.Pending == "" && key == "q" {
			return a.closeWorkspace(closeRequest{})
		}
		return a.pluginBufferKey(k)
	}
	if key == a.Config.Keys["quit"] || (key == "q" && a.chatMotionPending == "" && a.Prompt.Pending == "" && a.popup == nil && !a.commandMode && !a.searchMode && ((a.focus == "chat" && a.visualMode == "") || (a.focus == "prompt" && a.Prompt.Mode == "normal"))) {
		if key == "q" {
			return a.runAction("window.close", "")
		}
		return a.quit()
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

	if a.inputIndex >= 0 && a.historyKey(k) {
		return nil
	}
	if cmd, handled := a.slashKey(k); handled {
		return cmd
	}
	if a.historyKey(k) {
		return nil
	}

	if (a.focus == "chat" || a.Prompt.Mode == "normal") && a.chatMotionPending == "" && a.Prompt.Pending == "" && key == ":" {
		a.commandMode = true
		a.command = ""
		return nil
	}

	if a.focus == "chat" {
		return a.chatKey(k)
	}

	return a.promptInput(k)
}

// promptInput applies native editing and clipboard changes without submitting the draft.
func (a *App) promptInput(k tea.KeyMsg) tea.Cmd {
	key := k.String()
	text := ""
	if k.Type == tea.KeyRunes || k.Type == tea.KeySpace {
		text = string(k.Runes)
	}
	a.inputIndex = -1
	a.Prompt.Key(key, text)
	if a.Prompt.Yanked && len(a.Prompt.Register) > 0 {
		return a.copyText(string(a.Prompt.Register), a.Prompt.Linewise)
	}

	a.completionQuery = strings.TrimPrefix(a.Prompt.String(), "/")
	a.completionIndex = 0
	return nil
}
