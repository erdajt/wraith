package ui

import "github.com/erdajt/wraith/internal/plugin"

// discardPluginSurfaces releases invisible creations abandoned by a failed host action chain.
func discardPluginSurfaces(m pluginResultMsg) {
	for current := &m; current != nil; current = current.Parent {
		var ids []string
		for _, effect := range current.Result.Effects {
			if effect.Name != "surface.apply" {
				continue
			}
			mutation, ok := effect.Data.(plugin.SurfaceMutation)
			if ok && (mutation.Operation == "buffer.create" || mutation.Operation == "window.create" || mutation.Operation == "region.create") {
				ids = append(ids, mutation.ID)
			}
		}
		if current.Runtime != nil && len(ids) > 0 {
			current.Runtime.Surfaces().DiscardReservations(ids)
		}
	}
}
