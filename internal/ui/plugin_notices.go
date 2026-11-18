package ui

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/erdajt/wraith/internal/plugin"
)

type pluginNoticeMsg struct {
	Runtime *plugin.Runtime
	Text    string
}

// watchPluginNotices waits outside the ui for cleanup failures, including retired configurations.
func (a *App) watchPluginNotices(runtime *plugin.Runtime) tea.Cmd {
	if runtime == nil {
		return nil
	}
	return func() tea.Msg {
		select {
		case text, ok := <-runtime.Notices():
			if ok {
				return pluginNoticeMsg{Runtime: runtime, Text: text}
			}
		case <-a.done:
		}
		return nil
	}
}

// applyPluginNotice labels retired configurations and keeps terminal controls out of diagnostics.
func (a *App) applyPluginNotice(m pluginNoticeMsg) tea.Cmd {
	origin := "lua: "
	if m.Runtime != a.plugins {
		origin = "lua: previous configuration cleanup: "
	}
	a.notice = origin + strings.ReplaceAll(clean(m.Text), "\n", " ")
	a.pluginDiagnostics = append(a.pluginDiagnostics, a.notice)
	if len(a.pluginDiagnostics) > 64 {
		a.pluginDiagnostics = a.pluginDiagnostics[len(a.pluginDiagnostics)-64:]
	}
	return a.watchPluginNotices(m.Runtime)
}
