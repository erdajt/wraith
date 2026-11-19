package ui

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/erdajt/wraith/internal/plugin"
)

// pluginBufferKey applies native Vim input to one view and publishes shared text by revision.
func (a *App) pluginBufferKey(k tea.KeyMsg) tea.Cmd {
	view := a.focusedSurfaceView()
	if view == nil {
		return nil
	}
	buffer, ok := a.surfaceBuffers[view.window.BufferID]
	if !ok {
		a.reconcileSurfaces()
		return nil
	}
	b := view.reader.Buffer
	before := b.String()
	key := k.String()
	if !buffer.Editable {
		text, _ := view.reader.key(k, max(1, a.chatHeight()-2))
		a.syncSurfaceCursor(view)
		if text != "" {
			return a.copyText(text, b.Linewise)
		}
		return nil
	}
	if b.Mode != "insert" && b.Mode != "search" && (key == "ctrl+d" || key == "ctrl+u" || key == "ctrl+f" || key == "ctrl+b" || key == "pgup" || key == "pgdown") {
		view.reader.key(k, max(1, a.chatHeight()-2))
	} else {
		text := ""
		if k.Type == tea.KeyRunes || k.Type == tea.KeySpace {
			text = string(k.Runes)
		}
		b.Key(key, text)
	}
	if b.String() != before {
		mutation := plugin.SurfaceMutation{Operation: "buffer.set_lines", ID: buffer.ID, Owner: buffer.Owner, Expected: buffer.Revision, Start: 0, End: len(buffer.Lines), Lines: strings.Split(b.String(), "\n")}
		delta, err := a.plugins.Surfaces().HostApply(mutation)
		if err != nil {
			if current, readErr := a.plugins.Surfaces().Buffer(buffer.ID); readErr == nil {
				a.cacheSurfaceBuffer(current, true)
			} else {
				a.reconcileSurfaces()
			}
			a.notice = "buffer edit failed: " + err.Error()
			return nil
		}
		a.applySurfaceDelta(delta, false)
	}
	a.syncSurfaceCursor(view)
	if b.Yanked && len(b.Register) > 0 {
		return a.copyText(string(b.Register), b.Linewise)
	}
	return nil
}

// syncSurfaceCursor publishes rune coordinates without altering another view's navigation.
func (a *App) syncSurfaceCursor(view *surfaceView) {
	if a.surfaceReplay {
		return
	}
	position := plugin.SurfacePosition{}
	for _, ch := range view.reader.Buffer.Text[:min(view.reader.Buffer.Pos, len(view.reader.Buffer.Text))] {
		if ch == '\n' {
			position.Line++
			position.Column = 0
		} else {
			position.Column++
		}
	}
	mode := "normal"
	if view.reader.Buffer.Mode == "insert" {
		mode = "insert"
	}
	if position == view.window.Cursor && mode == view.window.Mode {
		return
	}
	window := view.window
	window.Cursor, window.Mode = position, mode
	delta, err := a.plugins.Surfaces().HostApply(plugin.SurfaceMutation{Operation: "window.configure", ID: window.ID, Owner: window.Owner, Expected: window.Revision, Window: &window})
	if err == nil && delta.Window != nil {
		view.window = *delta.Window
	} else if err != nil {
		a.notice = "buffer cursor: " + err.Error()
	}
}
