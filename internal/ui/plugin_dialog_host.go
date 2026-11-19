package ui

import (
	"errors"
	"slices"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/erdajt/wraith/internal/plugin"
)

type pluginDecisionMsg struct {
	runtime *plugin.Runtime
	dialog  plugin.Dialog
	data    map[string]any
}

// openPluginDialog creates an owned modal buffer without replacing an existing approval or picker.
func (a *App) openPluginDialog(runtime *plugin.Runtime, spec plugin.Dialog) tea.Cmd {
	if runtime != a.plugins || !runtime.Surfaces().OwnerValid(spec.Owner, spec.Lease) || spec.Invocation != "" && !runtime.InvocationValid(spec.Invocation) {
		runtime.DiscardScheduled([]string{spec.ID})
		return nil
	}
	if a.pluginDialog != nil || a.popup != nil || a.editing || a.proposal != nil {
		return a.submitPluginDecision(pluginDecisionMsg{runtime, spec, map[string]any{"cancelled": true, "reason": "busy"}})
	}

	d := &pluginDialog{spec: spec, runtime: runtime, reader: newReader(spec.Initial)}
	d.buffer = plugin.SurfaceBuffer{Owner: spec.Owner, Name: spec.Title, Editable: spec.Kind == "input", Lines: strings.Split(spec.Initial, "\n")}
	if spec.Kind == "select" {
		d.filter()
	} else {
		d.reader.Buffer.Mode = "insert"
		d.reader.Buffer.Pos = len(d.reader.Buffer.Text)
	}
	store := runtime.Surfaces()
	id, err := store.Reserve(spec.Owner, "buffer")
	if err == nil {
		d.buffer.ID = id
		var delta plugin.SurfaceDelta
		delta, err = store.Apply(plugin.SurfaceMutation{Operation: "buffer.create", ID: id, Owner: spec.Owner, Lease: spec.Lease, Buffer: &d.buffer})
		if err == nil {
			d.buffer = *delta.Buffer
		}
	}
	if err != nil {
		store.DiscardReservations([]string{id})
		return a.submitPluginDecision(pluginDecisionMsg{runtime, spec, map[string]any{"cancelled": true, "reason": err.Error()}})
	}
	a.pluginDialog = d
	a.keyPending = nil
	return nil
}

// finishPluginDialog releases the modal before dispatching its once-only decision callback.
func (a *App) finishPluginDialog(data map[string]any) tea.Cmd {
	d := a.pluginDialog
	if d == nil {
		return nil
	}
	a.pluginDialog = nil
	a.keyPending = nil
	if buffer, err := d.runtime.Surfaces().Buffer(d.buffer.ID); err == nil {
		_, _ = d.runtime.Surfaces().Apply(plugin.SurfaceMutation{Operation: "buffer.delete", ID: buffer.ID, Owner: d.spec.Owner, Lease: d.spec.Lease, Expected: buffer.Revision})
	}
	return a.submitPluginDecision(pluginDecisionMsg{d.runtime, d.spec, data})
}

// submitPluginDecision retries queue pressure without dropping or invoking a decision twice.
func (a *App) submitPluginDecision(m pluginDecisionMsg) tea.Cmd {
	if m.runtime != a.plugins || !m.runtime.Surfaces().OwnerValid(m.dialog.Owner, m.dialog.Lease) || m.dialog.Invocation != "" && !m.runtime.InvocationValid(m.dialog.Invocation) {
		m.runtime.DiscardScheduled([]string{m.dialog.ID})
		return nil
	}
	m.data["id"] = m.dialog.ID
	if _, ok := m.data["cancelled"]; !ok {
		m.data["cancelled"] = false
	}
	result, err := m.runtime.Submit(m.dialog.ID, "", a.pluginSnapshot("", "", m.data), a.Config)
	if errors.Is(err, plugin.ErrQueueFull) {
		return tea.Tick(10*time.Millisecond, func(time.Time) tea.Msg { return m })
	}
	if err != nil {
		m.runtime.DiscardScheduled([]string{m.dialog.ID})
		a.notice = "lua dialog: " + err.Error()
		return nil
	}
	pending := pluginResultMsg{Runtime: m.runtime, Owner: a.keyOwner(), Draft: a.Prompt.String(), Digest: configDigest(a.Config), Target: a.pluginTarget(), Keys: keymapDigest(a.Config), LeaseOwner: m.dialog.Owner, Lease: m.dialog.Lease}
	return func() tea.Msg {
		select {
		case pending.Result = <-result:
			return pending
		case <-a.done:
			return nil
		}
	}
}

// reconcilePluginDialog dismisses disposed owners and yields to native approval dialogs.
func (a *App) reconcilePluginDialog() tea.Cmd {
	d := a.pluginDialog
	if d == nil {
		return nil
	}
	if d.runtime != a.plugins || !d.runtime.Surfaces().OwnerValid(d.spec.Owner, d.spec.Lease) {
		d.runtime.DiscardScheduled([]string{d.spec.ID})
		a.pluginDialog = nil
		return nil
	}
	if d.spec.Invocation != "" && !d.runtime.InvocationValid(d.spec.Invocation) {
		return a.finishPluginDialog(map[string]any{"cancelled": true, "reason": "invocation ended"})
	}
	if a.popup != nil || a.proposal != nil || a.editing {
		return a.finishPluginDialog(map[string]any{"cancelled": true, "reason": "superseded"})
	}
	buffer, err := d.runtime.Surfaces().Buffer(d.buffer.ID)
	if err != nil {
		return a.finishPluginDialog(map[string]any{"cancelled": true, "reason": "buffer disposed"})
	}
	if buffer.Revision != d.buffer.Revision {
		if d.spec.Kind == "select" && !slices.Equal(buffer.Lines, d.buffer.Lines) {
			return a.finishPluginDialog(map[string]any{"cancelled": true, "reason": "selection buffer changed"})
		}
		d.buffer = buffer
		d.reader.Buffer.SyncText(strings.Join(buffer.Lines, "\n"), true)
	}
	return nil
}

// syncPluginDialogText publishes modal edits through the same revision checks as ordinary buffers.
func (a *App) syncPluginDialogText(d *pluginDialog) error {
	store := d.runtime.Surfaces()
	current, err := store.Buffer(d.buffer.ID)
	if err != nil {
		return err
	}
	delta, err := store.Apply(plugin.SurfaceMutation{
		Operation: "buffer.set_lines", ID: d.buffer.ID, Owner: d.spec.Owner, Lease: d.spec.Lease,
		Expected: d.buffer.Revision, Start: 0, End: len(current.Lines), Lines: d.buffer.Lines,
	})
	if err != nil {
		return err
	}
	d.buffer = *delta.Buffer
	return nil
}
