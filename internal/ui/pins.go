package ui

import (
	tea "github.com/charmbracelet/bubbletea"
	"strconv"
)

func (a *App) pinKey(k tea.KeyMsg) (tea.Cmd, bool) {
	key := k.String()
	edit, remove := key == a.Config.Keys["pin_edit"], key == a.Config.Keys["pin_remove"]
	if !edit && !remove {
		return nil, false
	}
	m := a.popup
	if m.Index < 0 || m.Index >= len(m.Items) {
		return nil, true
	}
	index, err := strconv.Atoi(m.Items[m.Index].Value)
	if err != nil || index < 0 || index >= len(a.Session.Pins) {
		return nil, true
	}
	if edit {
		a.popup = nil
		return a.edit("pin", "", "", index), true
	}
	if a.demo {
		a.notice = "demo pins are read-only"
		return nil, true
	}
	candidate := a.Session.Clone()
	candidate.Pins = append(candidate.Pins[:index], candidate.Pins[index+1:]...)
	if err := a.Store.Save(candidate); err != nil {
		a.notice = err.Error()
		return nil, true
	}
	a.Session = candidate
	a.memoryCommand("pins", "")
	a.popup.Index = min(m.Index, max(0, len(a.popup.Items)-1))
	return nil, true
}
