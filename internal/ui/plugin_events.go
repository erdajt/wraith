package ui

import (
	"errors"
	"reflect"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/erdajt/wraith/internal/plugin"
)

// Update records lifecycle transitions and queues plugin events after normal state handling.
func (a *App) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	a.reconcileOutputRead()
	defer a.reconcileOutputRead()
	a.reconcileWorkspaceQueries(false)
	depth := a.pluginDepth
	if result, ok := msg.(pluginResultMsg); ok {
		a.pluginDepth = result.Depth
	}
	defer func() { a.pluginDepth = depth }()
	if a.plugins == nil {
		return a.update(msg)
	}
	before := a.plugins.Surfaces().Index()
	a.observeWorkspaceServices()
	a.reconcileSurfaces()
	dialogCommand := a.reconcilePluginDialog()
	session, window, places := a.Session.ID, a.windowID, a.windowPlaces()
	stream, streaming := msg.(streamMsg)
	finished := streaming && stream.Final && stream.Retry == 0 && stream.Serial == a.streamSerial && a.busy && !a.compacting && !a.executing
	model, cmd := a.update(msg)
	a.observeWorkspaceServices()
	a.reconcileWorkspaceQueries(false)
	select {
	case <-a.done:
		return model, cmd
	default:
	}
	a.reconcileSurfaces()
	a.syncNativeSurfaces()
	cmds := []tea.Cmd{cmd, dialogCommand, a.reconcilePluginDialog()}
	cmds = append(cmds, a.flushWorkspaceEvents())
	if session != a.Session.ID {
		cmds = append(cmds, a.callPlugin("", "session_changed", ""))
	}
	after := a.plugins.Surfaces().Index()
	if window != a.windowID || !reflect.DeepEqual(places, a.windowPlaces()) || !reflect.DeepEqual(before.Windows, after.Windows) {
		cmds = append(cmds, a.callPlugin("", "window_changed", ""))
	}
	if surfaceTextChanged(before.Buffers, after.Buffers) {
		a.bufferEventPending = true
		a.bufferEventRuntime = a.plugins
	}
	cmds = append(cmds, a.flushBufferEvent())
	if finished {
		cmds = append(cmds, a.callPlugin("", "response_finished", ""))
	}
	return model, tea.Batch(cmds...)
}

// surfaceTextChanged ignores styling updates so decoration observers do not trigger themselves.
func surfaceTextChanged(before, after []plugin.SurfaceBufferInfo) bool {
	if len(before) != len(after) {
		return true
	}
	for i, buffer := range before {
		if buffer.ID != after[i].ID || buffer.TextRevision != after[i].TextRevision {
			return true
		}
	}
	return false
}

// flushBufferEvent retries the latest full index after queue pressure without retaining every edit.
func (a *App) flushBufferEvent() tea.Cmd {
	if !a.bufferEventPending {
		return nil
	}
	if a.bufferEventRuntime != a.plugins {
		a.bufferEventPending = false
		return nil
	}
	data := map[string]any{"buffers": a.plugins.Surfaces().Index().Buffers}
	cmd, err := a.queuePluginData("", "buffer_changed", "", data)
	if errors.Is(err, plugin.ErrQueueFull) {
		return nil
	}
	a.bufferEventPending = false
	if err != nil {
		a.notice = "lua buffer event: " + err.Error()
	}
	return cmd
}
