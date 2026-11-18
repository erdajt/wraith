package ui

import tea "github.com/charmbracelet/bubbletea"

type queuedKeysMsg struct {
	Keys         []tea.KeyMsg
	Owner        string
	Mapped       bool
	Raw, Expired bool
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
		a.editorDeferred = append(a.editorDeferred, m)
		return nil
	}

	var cmds []tea.Cmd
	for i, key := range m.Keys {
		select {
		case <-a.done:
			return tea.Batch(cmds...)
		default:
		}
		if a.keyOwner() != m.Owner {
			a.notice = "remaining key sequence cancelled after focus changed"
			break
		}
		var cmd tea.Cmd
		if m.Mapped {
			cmd = a.key(key)
		} else if key.Paste && len(a.keyPending) > 0 {
			keys := append(append([]tea.KeyMsg(nil), a.keyPending...), key)
			a.keyPending = nil
			cmd = a.dispatchKeys(queuedKeysMsg{Keys: keys, Owner: m.Owner, Raw: true})
		} else if m.Raw || key.Paste {
			cmd = a.keyDefault(key)
		} else {
			cmd = a.mapKey(key, true, m.Expired && i == len(m.Keys)-1)
		}
		if cmd == nil {
			if a.popup != nil && a.popup.Kind == "leader" {
				m.Owner = a.keyOwner()
			}
			continue
		}
		cmds = append(cmds, cmd)
		if a.editing && i+1 < len(m.Keys) {
			m.Keys = append([]tea.KeyMsg(nil), m.Keys[i+1:]...)
			a.editorDeferred = append(a.editorDeferred, m)
			break
		}
	}
	return tea.Batch(cmds...)
}
