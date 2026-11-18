package ui

import (
	"reflect"

	tea "github.com/charmbracelet/bubbletea"
)

// Update records lifecycle transitions and queues plugin events after normal state handling.
func (a *App) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	depth := a.pluginDepth
	if result, ok := msg.(pluginResultMsg); ok {
		a.pluginDepth = result.Depth
	}
	defer func() { a.pluginDepth = depth }()
	if a.plugins == nil {
		return a.update(msg)
	}
	session, window, places := a.Session.ID, a.windowID, a.windowPlaces()
	stream, streaming := msg.(streamMsg)
	finished := streaming && stream.Final && stream.Retry == 0 && stream.Serial == a.streamSerial && a.busy && !a.compacting && !a.executing
	model, cmd := a.update(msg)
	select {
	case <-a.done:
		return model, cmd
	default:
	}
	cmds := []tea.Cmd{cmd}
	if session != a.Session.ID {
		cmds = append(cmds, a.callPlugin("", "session_changed", ""))
	}
	if window != a.windowID || !reflect.DeepEqual(places, a.windowPlaces()) {
		cmds = append(cmds, a.callPlugin("", "window_changed", ""))
	}
	if finished {
		cmds = append(cmds, a.callPlugin("", "response_finished", ""))
	}
	return model, tea.Batch(cmds...)
}
