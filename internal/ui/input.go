package ui

import (
	"strings"
	"unicode/utf8"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/erdajt/wraith/internal/keymap"
	"github.com/erdajt/wraith/internal/vim"
)

// inputReader returns the focused read-only buffer without exposing a live shell.
func (a *App) inputReader() *reader {
	if a.popup != nil {
		return a.popup.Reader
	}
	if strings.HasPrefix(a.focus, "terminal") {
		index := 0
		if a.focus == "terminal1" {
			index = 1
		}
		if p := a.panes[index]; p != nil {
			return p.Reader
		}
	}
	return nil
}

// editorMode distinguishes literal arguments from operators and count-prefixed motions.
func editorMode(mode, pending string) string {
	if pending != "" {
		if pending == "r" || strings.Contains("fFtTia", pending[len(pending)-1:]) {
			return "argument"
		}
		return "operator"
	}
	if strings.HasPrefix(mode, "visual") {
		return "visual"
	}
	return mode
}

// feedInput applies bounded native editing keys without remapping or invoking workspace actions.
func (a *App) feedInput(sequence string) tea.Cmd {
	keys, err := keymap.Parse(sequence)
	if err != nil {
		a.notice = "input: " + err.Error()
		return nil
	}
	if !a.inputAvailable() {
		a.notice = "native input requires a prompt, conversation or read-only buffer"
		return nil
	}

	prepared := make([]tea.KeyMsg, 0, len(keys))
	for _, name := range keys {
		k := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(name)}
		if utf8.RuneCountInString(name) != 1 {
			found := false
			for _, kind := range append([]tea.KeyType{tea.KeyEsc, tea.KeyEnter, tea.KeyTab, tea.KeyBackspace, tea.KeyDelete, tea.KeyUp, tea.KeyDown, tea.KeyLeft, tea.KeyRight, tea.KeyHome, tea.KeyEnd, tea.KeyPgUp, tea.KeyPgDown}, tea.KeyCtrlR, tea.KeyCtrlD, tea.KeyCtrlU, tea.KeyCtrlF, tea.KeyCtrlB) {
				candidate := tea.KeyMsg{Type: kind}
				if candidate.String() == name {
					k, found = candidate, true
					break
				}
			}
			if !found {
				a.notice = "unsupported native input key: " + name
				return nil
			}
		}
		prepared = append(prepared, k)
	}

	owner := a.keyOwner()
	var commands []tea.Cmd
	for _, k := range prepared {
		if owner != a.keyOwner() || !a.inputAvailable() {
			break
		}
		if r := a.inputReader(); r != nil {
			text, exit := r.key(k, max(1, a.height-8))
			if exit && k.String() == "esc" {
				r.Buffer.Key("esc", "")
			}
			if text != "" {
				commands = append(commands, a.copyText(text, r.Buffer.Linewise))
			}
		} else if a.focus == "chat" {
			if a.searchMode {
				switch k.String() {
				case "esc":
					a.searchMode = false
				case "enter":
					a.search, a.searchMode = a.command, false
					a.command = ""
					a.findNext(false)
				case "backspace":
					runes := []rune(a.command)
					a.command = string(runes[:max(0, len(runes)-1)])
				default:
					a.command += string(k.Runes)
				}
			} else {
				commands = append(commands, a.chatInput(k))
			}
		} else {
			commands = append(commands, a.promptInput(k))
		}
	}
	return tea.Sequence(commands...)
}

// inputAvailable excludes approvals, pickers, command lines and live terminal input.
func (a *App) inputAvailable() bool {
	if a.editing || a.proposal != nil || a.commandMode {
		return false
	}
	if a.popup != nil || strings.HasPrefix(a.focus, "terminal") {
		return a.inputReader() != nil
	}
	return a.focus == "chat" || a.focus == "prompt"
}

// setInputMode changes the native editor mode without sending text or leaving its surface.
func (a *App) setInputMode(mode string) tea.Cmd {
	if !a.inputAvailable() {
		a.notice = "mode changes require an editable or read-only buffer"
		return nil
	}
	if mode != "normal" && mode != "insert" && mode != "visual" && mode != "visual-line" && mode != "search" {
		a.notice = "invalid input mode: " + mode
		return nil
	}
	if mode == "insert" && (a.focus != "prompt" || a.inputReader() != nil) {
		a.notice = "insert mode requires the prompt"
		return nil
	}
	if a.focus == "prompt" && a.inputReader() == nil && a.Prompt.Mode == mode && a.Prompt.Pending == "" && a.Prompt.Count == "" {
		return nil
	}
	if a.focus == "chat" && a.inputReader() == nil {
		a.chatZ, a.chatG, a.searchMode = false, false, false
		a.visualMode, a.command, a.chatCount, a.chatMotionPending, a.chatMotionCount = "", "", "", "", ""
	}
	sequence := "<Esc>"
	switch mode {
	case "insert":
		sequence += "i"
	case "visual":
		sequence += "v"
	case "visual-line":
		sequence += "V"
	case "search":
		sequence += "/"
	}
	return a.feedInput(sequence)
}

// inputSnapshot reports zero-based editor coordinates and pending native state.
func (a *App) inputSnapshot() map[string]any {
	snapshot := map[string]any{"mode": a.inputMode(), "available": a.inputAvailable()}
	var b *vim.Buffer
	if r := a.inputReader(); r != nil {
		b = r.Buffer
	} else if a.focus == "prompt" {
		b = a.Prompt
	} else if a.focus == "chat" {
		snapshot["row"], snapshot["column"] = a.chatCursor, a.chatColumn
		snapshot["count"], snapshot["pending"] = a.chatCount, a.chatMotionPending
		snapshot["operator_count"], snapshot["pending_g"], snapshot["pending_z"] = a.chatMotionCount, a.chatG, a.chatZ
		snapshot["search"] = a.search
		if a.searchMode {
			snapshot["search"] = a.command
		}
		snapshot["anchor_row"], snapshot["anchor_column"] = a.visualRow, a.visualColumn
	}
	if b != nil {
		left, right := b.Selection()
		snapshot["cursor"], snapshot["selection_start"], snapshot["selection_end"] = b.Pos, left, right
		snapshot["count"], snapshot["pending"], snapshot["search"] = b.Count, b.Pending, b.SearchQuery()
	}
	return snapshot
}
