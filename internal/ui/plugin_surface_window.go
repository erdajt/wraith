package ui

import (
	"fmt"

	"github.com/erdajt/wraith/internal/plugin"
)

// surfaceWindowAction keeps user splits, closes and floating resizes visible to Lua.
func (a *App) surfaceWindowAction(action string) bool {
	view := a.focusedSurfaceView()
	if action == "o" {
		for handle, id := range a.surfaceWindows {
			if id != a.windowID {
				if err := a.closeSurfaceWindow(handle); err != nil {
					a.notice = err.Error()
					return true
				}
			}
		}
		if view != nil && view.window.Kind == "float" {
			window := view.window
			window.Kind, window.Position, window.RelativeTo = "split", "right", ""
			window.Row, window.Column = 0, 0
			delta, err := a.plugins.Surfaces().HostApply(plugin.SurfaceMutation{Operation: "window.configure", ID: window.ID, Owner: window.Owner, Expected: window.Revision, Window: &window})
			if err != nil {
				a.notice = err.Error()
				return true
			}
			view.window = *delta.Window
		}
		return false
	}
	if view == nil {
		return false
	}
	var err error
	switch action {
	case "s", "v":
		window := view.window
		window.ID, window.Revision, window.Kind = "", 0, "split"
		window.RelativeTo, window.Focus, window.Position = view.window.ID, true, "below"
		if view.window.Kind == "float" {
			window.RelativeTo = ""
		}
		window.Width, window.Height, window.Row, window.Column = 0, 0, 0, 0
		if action == "v" {
			window.Position = "right"
		}
		var id string
		id, err = a.plugins.Surfaces().Reserve(window.Owner, "window")
		if err == nil {
			err = a.applySurfaceMutation(a.plugins, plugin.SurfaceMutation{Operation: "window.create", ID: id, Owner: window.Owner, Lease: a.plugins.Surfaces().OwnerLease(window.Owner), Window: &window})
			if err != nil {
				a.plugins.Surfaces().DiscardReservations([]string{id})
			}
		}
	case "c":
		err = a.closeSurfaceWindow(view.window.ID)
	case "+", "-", ">", "<":
		if view.window.Kind != "float" {
			return false
		}
		window := view.window
		change := 1
		if action == "-" || action == "<" {
			change = -1
		}
		if action == ">" || action == "<" {
			window.Width = max(3, window.Width+change)
		} else {
			window.Height = max(3, window.Height+change)
		}
		var delta plugin.SurfaceDelta
		delta, err = a.plugins.Surfaces().HostApply(plugin.SurfaceMutation{Operation: "window.configure", ID: window.ID, Owner: window.Owner, Expected: window.Revision, Window: &window})
		if err == nil {
			a.applySurfaceDelta(delta, false)
		}
	default:
		return false
	}
	if err != nil {
		a.notice = "buffer window: " + err.Error()
	} else {
		a.notice = fmt.Sprintf("%d windows open", len(a.windows))
	}
	return true
}

// closeSurfaceWindow disposes a view while leaving its shared buffer available to other views.
func (a *App) closeSurfaceWindow(handle string) error {
	window, err := a.plugins.Surfaces().Window(handle)
	if err != nil {
		return err
	}
	delta, err := a.plugins.Surfaces().HostApply(plugin.SurfaceMutation{Operation: "window.delete", ID: handle, Owner: window.Owner, Expected: window.Revision})
	if err != nil {
		return err
	}
	a.applySurfaceDelta(delta, false)
	return nil
}

// syncSurfaceGeometry publishes user-resized tiled dimensions without replaying layout intents.
func (a *App) syncSurfaceGeometry() {
	for _, place := range a.windowPlaces() {
		view := a.windows[place.id].surface
		if view == nil || view.window.Kind != "split" {
			continue
		}
		window := view.window
		window.Width, window.Height = place.width, place.height
		delta, err := a.plugins.Surfaces().HostApply(plugin.SurfaceMutation{Operation: "window.configure", ID: window.ID, Owner: window.Owner, Expected: window.Revision, Window: &window})
		if err != nil {
			a.notice = "buffer geometry: " + err.Error()
			continue
		}
		view.window = *delta.Window
	}
}
