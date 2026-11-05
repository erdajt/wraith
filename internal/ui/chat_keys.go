package ui

import (
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/ansi"
	"strconv"
)

func (a *App) ensureChatCursor() {
	lines := a.transcript(max(10, a.mainWidth()-5))
	if a.chatCursor < 0 {
		a.chatCursor = a.offset
		if a.follow {
			a.chatCursor = max(0, len(lines)-a.bodyHeight())
			a.offset = a.chatCursor
		}
	}
	a.chatCursor = max(0, min(a.chatCursor, len(lines)-1))
	if len(lines) > 0 {
		a.selected = lines[a.chatCursor].Message
	}
	a.revealCursor(len(lines))
}

func (a *App) revealCursor(count int) {
	a.follow = false
	if a.chatCursor < a.offset {
		a.offset = a.chatCursor
	}
	if a.chatCursor >= a.offset+a.bodyHeight() {
		a.offset = a.chatCursor - a.bodyHeight() + 1
	}
	a.offset = max(0, min(a.offset, max(0, count-a.bodyHeight())))
}

func (a *App) chatKey(k tea.KeyMsg) tea.Cmd {
	key := k.String()
	if a.chatMotionPending != "" {
		return a.moveChatText(key, 1)
	}
	if a.chatZ {
		a.chatZ = false
		if key == "a" {
			a.toggleThinking(a.selected)
			return nil
		}
	}
	if key == "z" && a.Config.Keys["thinking_toggle"] == "za" {
		a.chatZ = true
		return nil
	}
	if key == a.Config.Keys["thinking_toggle"] {
		a.toggleThinking(a.selected)
		return nil
	}
	if len(key) == 1 && key[0] >= '0' && key[0] <= '9' && (key != "0" || a.chatCount != "") {
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
	a.ensureChatCursor()
	lines := a.transcript(max(10, a.mainWidth()-5))
	switch key {
	case "v", "V":
		mode := "visual"
		if key == "V" {
			mode = "visual-line"
		}
		if a.visualMode == mode {
			a.visualMode = ""
		} else {
			a.visualMode = mode
			a.visualRow = a.chatCursor
			a.visualColumn = a.chatColumn
		}
	case "esc":
		a.visualMode = ""
	case "y":
		if a.visualMode != "" {
			return a.yank()
		}
		return a.moveChatText(key, count)
	case "Y":
		return a.moveChatText(key, count)
	case "j", "down":
		a.chatCursor += count
	case "k", "up":
		a.chatCursor -= count
	case "h", "left":
		a.chatColumn = max(0, a.chatColumn-count)
	case "l", "right":
		a.chatColumn += count
	case "0", "home":
		a.chatColumn = 0
	case "$", "end":
		if len(lines) > 0 {
			a.chatColumn = max(0, ansi.StringWidth(lines[a.chatCursor].Text)-1)
		}
	case "ctrl+d", "pgdown":
		a.chatCursor += count * max(1, a.bodyHeight()/2)
	case "ctrl+u", "pgup":
		a.chatCursor -= count * max(1, a.bodyHeight()/2)
	case "g":
		if a.chatG {
			a.chatCursor = 0
			a.chatColumn = 0
			a.chatG = false
		} else {
			a.chatG = true
		}
	case "G":
		a.chatCursor = max(0, len(lines)-1)
	case "]":
		a.selected = min(len(a.Session.Messages)-1, a.selected+1)
		a.jumpSelected()
		return nil
	case "[":
		a.selected = max(0, a.selected-1)
		a.jumpSelected()
		return nil
	case "/":
		a.searchMode = true
		a.command = ""
	case "n":
		a.findNext(false)
		return nil
	case "N":
		a.findNext(true)
		return nil
	case "^", "{", "}", "w", "W", "b", "B", "e", "E", "f", "F", "t", "T", "%":
		return a.moveChatText(key, count)
	case "enter":
		if len(lines) > 0 && lines[a.chatCursor].Block >= 0 {
			return a.editCursorBlock(lines[a.chatCursor])
		}
		return a.selectedBlocks()
	case "i":
		a.focus = "prompt"
		a.Prompt.Mode = "insert"
	case "?":
		return a.runCommand("help")
	}
	a.chatCursor = max(0, min(a.chatCursor, len(lines)-1))
	if len(lines) > 0 {
		a.chatColumn = min(a.chatColumn, max(0, ansi.StringWidth(lines[a.chatCursor].Text)-1))
		a.selected = lines[a.chatCursor].Message
	}
	a.revealCursor(len(lines))
	return nil
}
