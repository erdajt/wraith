package ui

import (
	tea "github.com/charmbracelet/bubbletea"
	"github.com/erdajt/wraith/internal/plugin"
)

type pluginScheduledMsg struct {
	Runtime *plugin.Runtime
	IDs     []string
}

// runPluginSchedule captures fresh workspace state after the preceding callback's effects finish.
func (a *App) runPluginSchedule(m pluginScheduledMsg) tea.Cmd {
	if len(m.IDs) == 0 {
		return nil
	}
	if m.Runtime != a.plugins {
		m.Runtime.DiscardScheduled(m.IDs)
		return nil
	}
	cmd := a.callPlugin(m.IDs[0], "", "")
	if cmd == nil {
		m.Runtime.DiscardScheduled(m.IDs[:1])
		return func() tea.Msg { return pluginScheduledMsg{Runtime: m.Runtime, IDs: m.IDs[1:]} }
	}
	return func() tea.Msg {
		if result, ok := cmd().(pluginResultMsg); ok {
			result.PendingSchedules = m.IDs[1:]
			return result
		}
		m.Runtime.DiscardScheduled(m.IDs[:1])
		return pluginScheduledMsg{Runtime: m.Runtime, IDs: m.IDs[1:]}
	}
}

// remainingPluginSchedules keeps already queued siblings independent of one callback's failure.
func remainingPluginSchedules(m pluginResultMsg) pluginScheduledMsg {
	for current := &m; current != nil; current = current.Parent {
		if len(current.PendingSchedules) > 0 {
			return pluginScheduledMsg{Runtime: current.Runtime, IDs: current.PendingSchedules}
		}
	}
	return pluginScheduledMsg{}
}

// discardPluginSchedules releases deferred callbacks when any part of their effect chain stops.
func discardPluginSchedules(m pluginResultMsg) {
	for current := &m; current != nil; current = current.Parent {
		ids := append([]string(nil), current.Scheduled...)
		for _, effect := range current.Result.Effects {
			if effect.Name == "schedule.callback" || effect.Name == "dialog.open" {
				ids = append(ids, effect.Argument)
			}
		}
		if current.Runtime != nil && len(ids) > 0 {
			current.Runtime.DiscardScheduled(ids)
		}
	}
}
