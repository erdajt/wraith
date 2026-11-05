package ui

import (
	tea "github.com/charmbracelet/bubbletea"
	"github.com/erdajt/wraith/internal/ollama"
	"strings"
)

func (a *App) menuKey(k tea.KeyMsg) tea.Cmd {
	m := a.popup
	switch k.String() {
	case "esc":
		a.popup = nil
	case "j":
		m.Index = min(len(m.Items)-1, m.Index+1)
	case "k":
		m.Index = max(0, m.Index-1)
	case "enter":
		if m.Index < 0 || m.Index >= len(m.Items) {
			return nil
		}
		v := m.Items[m.Index].Value
		a.popup = nil
		switch m.Kind {
		case "models":
			a.Session.Model = v
			a.Config.Model = v
			a.Config.Thinking = "auto"
			a.Info = ollama.Info{}
			a.persist("model selected")
			return a.probe()
		case "theme":
			return a.set("theme", v)
		case "thinking":
			return a.set("thinking", v)
		case "settings":
			return a.settingMenu(v)
		case "value":
			key, value, _ := strings.Cut(v, "=")
			return a.set(key, value)
		case "compact", "overflow":
			if v == "yes" {
				return a.startCompact()
			}
			return a.declineCompact()
		}
	}
	return nil
}
