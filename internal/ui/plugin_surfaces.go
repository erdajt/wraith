package ui

import (
	"errors"
	"strings"

	"github.com/erdajt/wraith/internal/plugin"
)

type surfaceView struct {
	window   plugin.SurfaceWindow
	reader   *reader
	revision uint64
}

// focusedSurfaceView returns the plugin editor attached to the current conversation leaf.
func (a *App) focusedSurfaceView() *surfaceView {
	if a.focus == "chat" && a.conversationView != nil {
		return a.conversationView.surface
	}
	return nil
}

// applySurfaceMutation validates host placement before committing a revisioned plugin edit.
func (a *App) applySurfaceMutation(runtime *plugin.Runtime, mutation plugin.SurfaceMutation) error {
	if runtime == nil || runtime != a.plugins {
		return errors.New("surface belongs to an inactive runtime")
	}
	if (mutation.Operation == "window.create" || mutation.Operation == "window.configure") && mutation.Window != nil {
		window := mutation.Window
		placement := mutation.Operation == "window.create"
		if !placement {
			current, err := runtime.Surfaces().Window(mutation.ID)
			if err != nil {
				return err
			}
			placement = current.Kind != window.Kind || current.Position != window.Position || current.RelativeTo != window.RelativeTo
		}
		if mutation.Operation == "window.create" && len(a.windows) >= 64 {
			return errors.New("workspace window limit reached")
		}
		if placement && window.RelativeTo != "" {
			relative := a.windows[a.surfaceWindows[window.RelativeTo]]
			if relative == nil || window.RelativeTo == mutation.ID || relative.surface.window.Kind == "float" {
				return errors.New("relative window must identify another live tiled window")
			}
		}
	}
	if (mutation.Operation == "window.focus" || mutation.Window != nil && mutation.Window.Focus) && (a.popup != nil || a.pluginDialog != nil || a.editing || a.proposal != nil) {
		return errors.New("surface focus is unavailable while a dialog owns input")
	}
	delta, err := runtime.Surfaces().Apply(mutation)
	if err != nil {
		return err
	}
	a.applySurfaceDelta(delta, true)
	return nil
}

// applySurfaceDelta updates detached display caches after an accepted store mutation.
func (a *App) applySurfaceDelta(delta plugin.SurfaceDelta, external bool) {
	if a.surfaceBuffers == nil {
		a.surfaceBuffers = map[string]plugin.SurfaceBuffer{}
		a.surfaceRegions = map[string]plugin.SurfaceRegion{}
		a.surfaceWindows = map[string]uint64{}
	}
	a.surfaceRuntime = a.plugins
	for _, id := range delta.RemovedWindows {
		a.removeSurfaceWindow(id)
	}
	for _, id := range delta.RemovedBuffers {
		delete(a.surfaceBuffers, id)
		delete(a.surfaceKeymaps, id)
	}
	for _, id := range delta.RemovedRegions {
		delete(a.surfaceRegions, id)
	}
	if delta.Buffer != nil {
		a.cacheSurfaceBuffer(*delta.Buffer, external)
	}
	for _, buffer := range delta.ChangedBuffers {
		a.cacheSurfaceBuffer(buffer, external)
	}
	for _, window := range delta.ChangedWindows {
		if view := a.windows[a.surfaceWindows[window.ID]]; view != nil && view.surface != nil {
			view.surface.window = window
		}
	}
	if delta.Region != nil {
		a.surfaceRegions[delta.Region.ID] = *delta.Region
	}
	if delta.Window != nil {
		a.cacheSurfaceWindow(*delta.Window, delta.Operation == "window.focus")
	}
	if delta.Buffer != nil || len(delta.ChangedBuffers) > 0 {
		for _, view := range a.windows {
			if view.surface != nil {
				a.syncSurfaceCursor(view.surface)
			}
		}
	}
}

// cacheSurfaceBuffer synchronizes shared document text without borrowing another view's cursor.
func (a *App) cacheSurfaceBuffer(buffer plugin.SurfaceBuffer, external bool) {
	a.surfaceBuffers[buffer.ID] = buffer
	a.adoptNativeSurface(buffer)
	text := strings.Join(buffer.Lines, "\n")
	for _, view := range a.windows {
		if view.surface == nil || view.surface.window.BufferID != buffer.ID {
			continue
		}
		view.surface.reader.Buffer.SyncText(text, external)
		view.surface.revision = buffer.Revision
	}
}

// cacheSurfaceWindow attaches one store window to the native split tree or floating layer.
func (a *App) cacheSurfaceWindow(window plugin.SurfaceWindow, focus bool) {
	id := a.surfaceWindows[window.ID]
	view := a.windows[id]
	if view == nil {
		buffer, ok := a.surfaceBuffers[window.BufferID]
		if !ok {
			var err error
			buffer, err = a.plugins.Surfaces().Buffer(window.BufferID)
			if err != nil {
				return
			}
			a.surfaceBuffers[buffer.ID] = buffer
		}
		editor := newReader(strings.Join(buffer.Lines, "\n"))
		for _, existing := range a.windows {
			if existing.surface != nil && existing.surface.window.BufferID == window.BufferID {
				editor.Buffer = existing.surface.reader.Buffer.CloneView()
				break
			}
		}
		view = &conversationView{surface: &surfaceView{reader: editor, revision: buffer.Revision}, chatCursor: -1}
		a.nextWindowID++
		id = a.nextWindowID
		a.windows[id], a.surfaceWindows[window.ID] = view, id
		focus = focus || window.Focus
	}
	old := view.surface.window
	if old.ID == "" || old.Kind != window.Kind || old.Position != window.Position || old.RelativeTo != window.RelativeTo {
		a.detachSurfaceWindow(id)
		if window.Kind == "split" {
			if a.windowTree == nil {
				a.windowTree = &windowNode{id: id}
			} else {
				relative := a.windowID
				if window.RelativeTo != "" {
					relative = a.surfaceWindows[window.RelativeTo]
				}
				if !a.windowTree.contains(relative) {
					relative = a.windowTree.layout(windowRect{width: 100, height: 100}, a.windowID)[0].id
				}
				a.windowTree.split(relative, id, window.Position == "left" || window.Position == "right")
				if window.Position == "left" || window.Position == "above" {
					a.windowTree.swap(relative, id)
				}
			}
		}
		a.ensureSurfaceFallback()
	}
	if view.surface.window.BufferID != "" && view.surface.window.BufferID != window.BufferID {
		buffer, err := a.plugins.Surfaces().Buffer(window.BufferID)
		if err == nil {
			a.surfaceBuffers[buffer.ID] = buffer
			view.surface.reader = newReader(strings.Join(buffer.Lines, "\n"))
			for _, existing := range a.windows {
				if existing != view && existing.surface != nil && existing.surface.window.BufferID == buffer.ID {
					view.surface.reader.Buffer = existing.surface.reader.Buffer.CloneView()
					break
				}
			}
			view.surface.revision = buffer.Revision
		}
	}
	if window.Cursor != view.surface.window.Cursor || view.surface.window.ID == "" {
		view.surface.reader.Buffer.Pos = surfaceCursorOffset(a.surfaceBuffers[window.BufferID].Lines, window.Cursor)
	}
	focus = focus || window.Focus
	window.Focus = false
	view.surface.window = window
	if focus {
		a.focusWindow(id)
	}
}

// removeSurfaceWindow closes only the view and restores a live conversation when its owner disappears.
func (a *App) removeSurfaceWindow(handle string) {
	id := a.surfaceWindows[handle]
	if id == 0 {
		return
	}
	delete(a.surfaceWindows, handle)
	delete(a.windows, id)
	a.detachSurfaceWindow(id)
	a.ensureSurfaceFallback()
	if a.windowID == id {
		focus := a.focus
		a.focusWindow(a.windowTree.layout(windowRect{width: 100, height: 100}, 0)[0].id)
		if focus != "chat" {
			a.focus = focus
		}
	}
}

// detachSurfaceWindow removes a tiled placement before a close or layout migration.
func (a *App) detachSurfaceWindow(id uint64) {
	if a.windowTree == nil {
		return
	}
	if a.windowTree.id == id {
		a.windowTree = nil
	} else {
		a.windowTree.remove(id)
	}
}

// ensureSurfaceFallback keeps a native conversation available when owned tiled views disappear.
func (a *App) ensureSurfaceFallback() {
	if a.windowTree != nil {
		return
	}
	a.nextWindowID++
	a.windowTree = &windowNode{id: a.nextWindowID}
	a.windows[a.nextWindowID] = &conversationView{chatCursor: -1, selected: -1, follow: true}
}

// reconcileSurfaces drops revoked owners and refreshes changed snapshots outside rendering.
func (a *App) reconcileSurfaces() {
	if a.surfaceRuntime != a.plugins {
		for id := range a.surfaceWindows {
			a.removeSurfaceWindow(id)
		}
		a.surfaceBuffers, a.surfaceRegions, a.surfaceWindows = nil, nil, nil
		a.surfaceKeymaps = nil
		a.surfaceRuntime = a.plugins
	}
	if a.plugins == nil {
		return
	}
	index := a.plugins.Surfaces().Index()
	liveWindows, liveBuffers, liveRegions := map[string]bool{}, map[string]bool{}, map[string]bool{}
	for _, buffer := range index.Buffers {
		liveBuffers[buffer.ID] = true
		if cached, ok := a.surfaceBuffers[buffer.ID]; !ok || cached.Revision != buffer.Revision {
			if value, err := a.plugins.Surfaces().Buffer(buffer.ID); err == nil {
				a.applySurfaceDelta(plugin.SurfaceDelta{Buffer: &value}, true)
			}
		}
	}
	for _, window := range index.Windows {
		liveWindows[window.ID] = true
		view := a.windows[a.surfaceWindows[window.ID]]
		if view == nil || view.surface.window.Revision != window.Revision {
			a.applySurfaceDelta(plugin.SurfaceDelta{Window: &window}, true)
		}
	}
	for _, region := range index.Regions {
		liveRegions[region.ID] = true
		if cached, ok := a.surfaceRegions[region.ID]; !ok || cached.Revision != region.Revision {
			if value, err := a.plugins.Surfaces().Region(region.ID); err == nil {
				a.applySurfaceDelta(plugin.SurfaceDelta{Region: &value}, true)
			}
		}
	}
	for id := range a.surfaceWindows {
		if !liveWindows[id] {
			a.removeSurfaceWindow(id)
		}
	}
	for id := range a.surfaceBuffers {
		if !liveBuffers[id] {
			delete(a.surfaceBuffers, id)
			delete(a.surfaceKeymaps, id)
		}
	}
	for id := range a.surfaceRegions {
		if !liveRegions[id] {
			delete(a.surfaceRegions, id)
		}
	}
}

// surfaceCursorOffset converts a validated line and rune column to the editor's rune offset.
func surfaceCursorOffset(lines []string, cursor plugin.SurfacePosition) int {
	position := 0
	for i, line := range lines {
		if i == cursor.Line {
			return position + min(max(0, cursor.Column), len([]rune(line)))
		}
		position += len([]rune(line)) + 1
	}
	return max(0, position-1)
}
