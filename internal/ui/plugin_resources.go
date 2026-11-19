package ui

import (
	"errors"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/erdajt/wraith/internal/plugin"
)

type pluginResourceMsg struct {
	Runtime *plugin.Runtime
	Event   plugin.ResourceEvent
}

type pluginResourceResultMsg struct{ Result pluginResultMsg }

// watchPluginResources receives one managed event without touching Lua or workspace state.
func (a *App) watchPluginResources(runtime *plugin.Runtime) tea.Cmd {
	if runtime == nil {
		return nil
	}
	return func() tea.Msg {
		select {
		case event, ok := <-runtime.Events():
			if ok {
				return pluginResourceMsg{Runtime: runtime, Event: event}
			}
		case <-a.done:
		}
		return nil
	}
}

// callPluginResource combines immutable service data with the current workspace snapshot.
func (a *App) callPluginResource(m pluginResourceMsg) tea.Cmd {
	if m.Runtime != a.plugins || !m.Runtime.ResourceValid(m.Event) {
		if m.Runtime == a.plugins {
			return a.watchPluginResources(m.Runtime)
		}
		return nil
	}
	result, err := m.Runtime.SubmitResource(m.Event, a.pluginSnapshot("", "", nil), a.Config)
	if errors.Is(err, plugin.ErrQueueFull) {
		return tea.Tick(10*time.Millisecond, func(time.Time) tea.Msg { return m })
	}
	if err != nil {
		a.notice = "lua service: " + err.Error()
		return a.watchPluginResources(m.Runtime)
	}
	pending := pluginResultMsg{
		Runtime: m.Runtime, Resource: &m.Event, Owner: a.keyOwner(), Draft: a.Prompt.String(),
		Digest: configDigest(a.Config), Target: a.pluginTarget(), Keys: keymapDigest(a.Config),
	}
	return func() tea.Msg {
		select {
		case pending.Result = <-result:
			return pluginResourceResultMsg{Result: pending}
		case <-a.done:
		}
		return nil
	}
}

// startPluginResources activates candidate work only after the workspace has installed its runtime.
func (a *App) startPluginResources() tea.Cmd {
	if a.plugins == nil {
		return nil
	}
	if err := a.plugins.Activate(a.pluginManifest.Resources); err != nil {
		a.notice = "lua service: " + err.Error()
	}
	return a.watchPluginResources(a.plugins)
}

// discardPluginResources releases reserved starts left behind by an interrupted effect chain.
func discardPluginResources(m pluginResultMsg) {
	for current := &m; current != nil; current = current.Parent {
		var ids []string
		for _, effect := range current.Result.Effects {
			if effect.Name == "resource.activate" {
				ids = append(ids, effect.Argument)
			}
		}
		if current.Runtime != nil && len(ids) > 0 {
			current.Runtime.DiscardResources(ids)
		}
	}
}
