package plugin

import (
	"maps"
	"slices"

	"github.com/erdajt/wraith/internal/keymap"
)

// syncOwnerBindings records explicit configuration changes without reviving removed mappings.
func (e *engine) syncOwnerBindings(bindings []keymap.Binding, owner string) {
	next := make(map[string]keymap.Binding, len(bindings))
	ids := map[string]bool{}
	for _, binding := range bindings {
		id, _ := bindingIdentity(binding.Mode, binding.Keys)
		next[id], ids[id] = binding, true
	}
	for id := range e.bindingLayers {
		ids[id] = true
	}
	for _, id := range slices.Sorted(maps.Keys(ids)) {
		binding, exists := next[id]
		layers := e.bindingLayers[id]
		var previous ownedBinding
		if len(layers) > 0 {
			previous = layers[len(layers)-1]
		}
		visible := len(layers) > 0 && !previous.deleted
		if visible == exists && (!exists || previous.binding == binding) {
			continue
		}
		if !exists {
			binding = previous.binding
		}
		layers = slices.DeleteFunc(layers, func(layer ownedBinding) bool { return layer.owner == owner })
		e.bindingLayers[id] = append(layers, ownedBinding{owner: owner, binding: binding, deleted: !exists})
		if !e.setup {
			e.dirty = true
		}
	}
}
