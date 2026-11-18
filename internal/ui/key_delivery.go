package ui

import tea "github.com/charmbracelet/bubbletea"

type queuedKeysMsg struct {
	Keys   []tea.KeyMsg
	Owner  string
	Mapped bool
}

// dispatchKeys preserves input order across editor handoffs and asynchronous commands.
func (a *App) dispatchKeys(m queuedKeysMsg) tea.Cmd {
	select {
	case <-a.done:
		return nil
	default:
	}
	if a.keyOwner() != m.Owner {
		a.notice = "remaining key sequence cancelled after focus changed"
		return nil
	}
	if a.editing {
		a.editorKeys = append(a.editorKeys, m)
		return nil
	}

	var timers []tea.Cmd
	for i, key := range m.Keys {
		if a.keyOwner() != m.Owner {
			a.notice = "remaining key sequence cancelled after focus changed"
			break
		}
		var cmd tea.Cmd
		if m.Mapped {
			cmd = a.key(key)
		} else {
			cmd = a.keyDefault(key)
		}
		if cmd == nil {
			if m.Mapped && a.popup != nil && a.popup.Kind == "leader" {
				m.Owner = a.keyOwner()
			}
			continue
		}
		if m.Mapped && len(a.keyPending) > 0 {
			timers = append(timers, cmd)
			continue
		}
		if i+1 < len(m.Keys) {
			rest := queuedKeysMsg{Keys: append([]tea.KeyMsg(nil), m.Keys[i+1:]...), Owner: m.Owner, Mapped: m.Mapped}
			cmd = tea.Sequence(cmd, func() tea.Msg { return rest })
		}
		return tea.Batch(append(timers, cmd)...)
	}
	return tea.Batch(timers...)
}
