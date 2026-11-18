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
			return "output." + editorMode(a.popup.Reader.Buffer.Mode, a.popup.Reader.Buffer.Pending)
		}
		return "picker"
	}
	if strings.HasPrefix(a.focus, "terminal") {
		index := 0
		if a.focus == "terminal1" {
			index = 1
		}
		if p := a.panes[index]; p != nil && p.Reader != nil {
			return "terminal.copy." + editorMode(p.Reader.Buffer.Mode, p.Reader.Buffer.Pending)
		}
		return "terminal"
	}
	if a.commandMode {
		return "command"
	}
	if a.searchMode {
		return "chat.search"
	}
	if a.focus == "chat" {
		mode := "normal"
		if a.visualMode != "" {
			mode = a.visualMode
		}
		if a.chatG || a.chatZ {
			return "chat.operator"
		}
		return "chat." + editorMode(mode, a.chatMotionPending)
	}
	return "prompt." + editorMode(a.Prompt.Mode, a.Prompt.Pending)
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
		keys := make([]tea.KeyMsg, 0, len(k.Runes))
		for _, r := range k.Runes {
			keys = append(keys, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
		}
		return a.dispatchKeys(queuedKeysMsg{Keys: keys, Owner: a.keyOwner(), Mapped: true})
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
	if len(a.keyPending) > 0 && a.keyPendingScope != a.keyScope() {
		a.keyPending = nil
		a.notice = "key sequence cancelled after focus changed"
	}
	if k.Paste {
		keys := append(append([]tea.KeyMsg(nil), a.keyPending...), k)
		a.keyPending = nil
		return a.replayKeys(keys, a.keyScope())
	}
	return a.mapKey(k, len(a.keyPending) > 0 && a.keyPendingDefault, false)
}

// mapKey resolves one route while keeping fallback chords separate from user mappings.
func (a *App) mapKey(k tea.KeyMsg, defaults, expired bool) tea.Cmd {
	key := k.String()
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
	if defaults {
		match = a.keymaps.LookupDefault(a.inputMode(), names)
	}
	if match.Pending && !expired {
		a.keyPending, a.keyPendingScope, a.keyPendingDefault = keys, scope, defaults
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
		if match.Input != "" {
			return a.feedInput(match.Input)
		}
		return a.runAction(match.Action, "")
	}
	if defaults {
		return a.dispatchKeys(queuedKeysMsg{Keys: keys, Owner: a.keyOwner(), Raw: true})
	}
	return a.replayKeys(keys, scope)
}

// replayKeys forwards a failed sequence once without remapping or crossing focus boundaries.
func (a *App) replayKeys(keys []tea.KeyMsg, scope string) tea.Cmd {
	if a.keyScope() != scope {
		return nil
	}
	return a.dispatchKeys(queuedKeysMsg{Keys: keys, Owner: a.keyOwner()})
}

// keymapTimeout resolves an ambiguous exact match or replays an incomplete prefix once.
func (a *App) keymapTimeout(serial uint64) tea.Cmd {
	if serial != a.keySerial || len(a.keyPending) == 0 {
		return nil
	}
	keys, scope, defaults := a.keyPending, a.keyPendingScope, a.keyPendingDefault
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
	if defaults {
		match = a.keymaps.LookupDefault(a.inputMode(), names)
	}
	if match.Exact {
		if match.Action == "focus.next" && len(keys) == 1 {
			if cmd, handled := a.slashKey(keys[0]); handled {
				return cmd
			}
		}
		if match.Input != "" {
			return a.feedInput(match.Input)
		}
		return a.runAction(match.Action, "")
	}
	return a.dispatchKeys(queuedKeysMsg{Keys: keys, Owner: a.keyOwner(), Raw: defaults, Expired: true})
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
