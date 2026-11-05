package ui

import (
	"sort"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
)

func (a *App) openLeader(prefix string) {
	items := []item{}
	for sequence, command := range a.Config.Bindings {
		if prefix != "" && !strings.HasPrefix(sequence, prefix+" ") {
			continue
		}
		items = append(items, item{Label: sequence, Value: command, Detail: "/" + command})
	}
	sort.Slice(items, func(i, j int) bool { return items[i].Label < items[j].Label })
	a.popup = &menu{Kind: "leader", Title: "leader · " + a.Config.Leader + " " + prefix, Prefix: prefix, Items: items}
}

func (a *App) leaderKey(k tea.KeyMsg) (tea.Cmd, bool) {
	if k.Type != tea.KeyRunes {
		return nil, false
	}
	prefix := strings.TrimSpace(a.popup.Prefix + " " + k.String())
	if command, ok := a.Config.Bindings[prefix]; ok {
		a.popup = nil
		return a.runCommand(command), true
	}
	for sequence := range a.Config.Bindings {
		if strings.HasPrefix(sequence, prefix+" ") {
			a.openLeader(prefix)
			return nil, true
		}
	}
	return nil, false
}

func (a *App) menuSearch(k tea.KeyMsg) bool {
	m := a.popup
	key := k.String()
	if !m.Searching {
		if key == "/" {
			if m.All == nil {
				m.All = append([]item{}, m.Items...)
			}
			m.Searching = true
			return true
		}
		if key == "esc" && m.Query != "" {
			m.Items = m.All
			m.Query = ""
			m.Index = 0
			return true
		}
		return false
	}
	switch key {
	case "enter", "down", "up":
		m.Searching = false
		return key == "enter"
	case "esc":
		m.Searching = false
		m.Query = ""
		m.Items = m.All
		m.Index = 0
		return true
	case "backspace":
		query := []rune(m.Query)
		if len(query) > 0 {
			m.Query = string(query[:len(query)-1])
		}
	default:
		if k.Type == tea.KeyRunes || k.Type == tea.KeySpace {
			m.Query += string(k.Runes)
		}
	}
	m.Items = nil
	for _, it := range m.All {
		if strings.Contains(strings.ToLower(it.Label+" "+it.Detail), strings.ToLower(m.Query)) {
			m.Items = append(m.Items, it)
		}
	}
	m.Index = 0
	return true
}
