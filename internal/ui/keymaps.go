package ui

import (
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

type keymapTimeoutMsg uint64

// inputMode identifies the editor that owns unmapped input, including pending vim operators.
func (a *App) inputMode() string {
	if a.popup != nil {
		if a.popup.Reader != nil {
			return "output"
		}
		return "picker"
	}
	if strings.HasPrefix(a.focus, "terminal") {
		index := 0
		if a.focus == "terminal1" {
			index = 1
		}
		if p := a.panes[index]; p != nil && p.Reader != nil {
			return "terminal.copy"
		}
		return "terminal"
	}
	if a.commandMode {
		return "command"
	}
	if a.searchMode {
		return "search"
	}
	if a.focus == "chat" {
		if a.chatMotionPending != "" || a.chatCount != "" || a.chatG || a.chatZ {
			return "chat.operator"
		}
		if a.visualMode != "" {
			return "chat.visual"
		}
		return "chat.normal"
	}
	if a.Prompt.Pending != "" || a.Prompt.Count != "" {
		return "prompt.operator"
	}
	if strings.HasPrefix(a.Prompt.Mode, "visual") {
		return "prompt.visual"
	}
	return "prompt." + a.Prompt.Mode
}

// keyOwner identifies the session, window and input surface that owns a sequence.
func (a *App) keyOwner() string {
	popup := ""
	if a.popup != nil {
		popup = a.popup.Kind
	}
	return fmt.Sprintf("%s:%d:%s:%s", a.Session.ID, a.windowID, a.focus, popup)
}

// keyScope invalidates pending mappings when input mode changes.
func (a *App) keyScope() string { return a.keyOwner() + ":" + a.inputMode() }

// key resolves user mappings before forwarding unmatched input to its original editor.
func (a *App) key(k tea.KeyMsg) tea.Cmd {
	if k.Type == tea.KeyRunes && len(k.Runes) > 1 && !k.Paste && !k.Alt {
		var cmds []tea.Cmd
		for _, r := range k.Runes {
			cmds = append(cmds, a.key(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}}))
		}
		return tea.Batch(cmds...)
	}
	key := k.String()
	if key == "ctrl+q" || key == a.Config.Keys["quit"] {
		a.keyPending = nil
		return a.quit()
	}
	if key == "ctrl+c" || key == a.Config.Keys["cancel"] {
		a.keyPending = nil
		return a.cancelInput()
	}
	if k.Paste {
		a.keyPending = nil
		return a.keyDefault(k)
	}
	scope := a.keyScope()
	if len(a.keyPending) > 0 && scope != a.keyPendingScope {
		a.keyPending = nil
		a.notice = "key sequence cancelled after focus changed"
	}
	if len(a.keyPending) > 0 && key == "esc" {
		a.keyPending = nil
		a.notice = "key sequence cancelled"
		return nil
	}
	keys := append(append([]tea.KeyMsg(nil), a.keyPending...), k)
	names := make([]string, len(keys))
	for i, value := range keys {
		names[i] = value.String()
	}
	match := a.keymaps.Lookup(a.inputMode(), names)
	if match.Pending {
		a.keyPending, a.keyPendingScope = keys, scope
		a.keySerial++
		serial := a.keySerial
		var hints []string
		for _, hint := range match.Hints {
			hints = append(hints, hint.Keys+" "+hint.Description)
		}
		a.notice = strings.Join(hints, " · ")
		return tea.Tick(time.Duration(a.Config.KeyTimeout)*time.Millisecond, func(time.Time) tea.Msg { return keymapTimeoutMsg(serial) })
	}
	a.keyPending = nil
	if match.Exact {
		if match.Action == "focus.next" && len(keys) == 1 {
			if cmd, handled := a.slashKey(k); handled {
				return cmd
			}
		}
		return a.runAction(match.Action, "")
	}
	return a.replayKeys(keys, scope)
}

// replayKeys forwards a failed sequence once without remapping or crossing focus boundaries.
func (a *App) replayKeys(keys []tea.KeyMsg, scope string) tea.Cmd {
	var cmds []tea.Cmd
	if a.keyScope() != scope {
		return nil
	}
	owner := a.keyOwner()
	for _, key := range keys {
		if a.keyOwner() != owner {
			a.notice = "remaining key sequence cancelled after focus changed"
			break
		}
		cmds = append(cmds, a.keyDefault(key))
	}
	return tea.Batch(cmds...)
}

// keymapTimeout resolves an ambiguous exact match or replays an incomplete prefix once.
func (a *App) keymapTimeout(serial uint64) tea.Cmd {
	if serial != a.keySerial || len(a.keyPending) == 0 {
		return nil
	}
	keys, scope := a.keyPending, a.keyPendingScope
	a.keyPending = nil
	if a.keyScope() != scope {
		a.notice = "key sequence cancelled after focus changed"
		return nil
	}
	names := make([]string, len(keys))
	for i, key := range keys {
		names[i] = key.String()
	}
	match := a.keymaps.Lookup(a.inputMode(), names)
	if match.Exact {
		return a.runAction(match.Action, "")
	}
	return a.replayKeys(keys, scope)
}

// cancelInput keeps cancellation available even when the configurable binding changes.
func (a *App) cancelInput() tea.Cmd {
	if a.popup == nil && strings.HasPrefix(a.focus, "terminal") {
		return a.terminalKey(tea.KeyMsg{Type: tea.KeyCtrlC})
	}
	a.stopAgent()
	if a.cancel != nil && a.busy {
		a.cancel()
		a.notice = "cancelling…"
	} else {
		a.commandMode, a.searchMode = false, false
		a.Prompt.Key("esc", "")
	}
	return nil
}
