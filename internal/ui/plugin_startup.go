package ui

import (
	"errors"
	"fmt"

	"github.com/erdajt/wraith/internal/plugin"
)

// preparePluginSurfaces commits startup display state only inside the isolated candidate runtime.
func preparePluginSurfaces(runtime *plugin.Runtime, nativeWindows int) ([]plugin.SurfaceDelta, error) {
	if runtime == nil {
		return nil, nil
	}
	store := runtime.Surfaces()
	var deltas []plugin.SurfaceDelta
	for _, mutation := range runtime.Manifest().Surface {
		if window := mutation.Window; window != nil {
			if mutation.Operation == "window.create" && nativeWindows+len(store.Index().Windows) >= 64 {
				return nil, errors.New("workspace window limit reached during plugin startup")
			}
			if window.RelativeTo != "" {
				relative, err := store.Window(window.RelativeTo)
				if err != nil || relative.Kind == "float" || relative.ID == mutation.ID {
					return nil, errors.New("startup relative window must identify another live tiled window")
				}
			}
		}
		delta, err := store.Apply(mutation)
		if err != nil {
			return nil, fmt.Errorf("plugin startup %s: %w", mutation.Operation, err)
		}
		deltas = append(deltas, delta)
	}
	return deltas, nil
}

// installPluginSurfaces publishes accepted startup views before ready callbacks or workers run.
func (a *App) installPluginSurfaces(deltas []plugin.SurfaceDelta) {
	if a.surfaceRuntime != a.plugins {
		for id := range a.surfaceWindows {
			a.removeSurfaceWindow(id)
		}
		a.surfaceBuffers, a.surfaceRegions, a.surfaceWindows = nil, nil, nil
		a.surfaceKeymaps = nil
		a.surfaceRuntime = a.plugins
	}
	a.surfaceReplay = true
	for _, delta := range deltas {
		a.applySurfaceDelta(delta, true)
	}
	a.reconcileSurfaces()
	a.surfaceReplay = false
	for _, view := range a.windows {
		if view.surface != nil {
			a.syncSurfaceCursor(view.surface)
		}
	}
	a.syncNativeSurfaces()
	if a.plugins != nil {
		a.plugins.CommitStartup()
		a.pluginManifest.Surface = nil
	}
}
