package ui

import (
	"errors"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/erdajt/wraith/internal/plugin"
)

type pluginInvocationMsg struct {
	Runtime *plugin.Runtime
	Start   plugin.InvocationStart
}

type pluginInvocationResultMsg struct{ Result pluginResultMsg }

// watchPluginInvocations waits for worker requests without entering the Lua actor.
func (a *App) watchPluginInvocations(runtime *plugin.Runtime) tea.Cmd {
	if runtime == nil {
		return nil
	}
	return func() tea.Msg {
		select {
		case start, ok := <-runtime.InvocationStarts():
			if ok {
				return pluginInvocationMsg{runtime, start}
			}
		case <-runtime.Done():
		case <-a.done:
		}
		return nil
	}
}

// callPluginInvocation combines detached provider input with the current workspace snapshot.
func (a *App) callPluginInvocation(m pluginInvocationMsg) tea.Cmd {
	if m.Runtime != a.plugins || !m.Runtime.InvocationValid(m.Start.ID) {
		m.Runtime.AbortInvocation(m.Start.ID, errors.New("extension runtime changed"))
		if m.Runtime == a.plugins {
			return a.watchPluginInvocations(m.Runtime)
		}
		return nil
	}
	result, err := m.Runtime.SubmitInvocation(m.Start, a.pluginSnapshot("", "", nil), a.Config)
	if errors.Is(err, plugin.ErrQueueFull) {
		return tea.Tick(10*time.Millisecond, func(time.Time) tea.Msg { return m })
	}
	if err != nil {
		m.Runtime.AbortInvocation(m.Start.ID, err)
		return a.watchPluginInvocations(m.Runtime)
	}
	pending := pluginResultMsg{
		Runtime: m.Runtime, Invocation: m.Start.ID, Explicit: true,
		Owner: a.keyOwner(), Draft: a.Prompt.String(), Digest: configDigest(a.Config),
		Target: a.pluginTarget(), Keys: keymapDigest(a.Config),
	}
	return func() tea.Msg {
		select {
		case pending.Result = <-result:
			return pluginInvocationResultMsg{pending}
		case <-m.Runtime.Done():
		case <-a.done:
		}
		m.Runtime.AbortInvocation(m.Start.ID, errors.New("workspace closed"))
		return nil
	}
}
