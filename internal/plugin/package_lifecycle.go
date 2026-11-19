package plugin

import (
	"fmt"
	"maps"
	"slices"
	"strings"
)

type packageRecord struct {
	owner        string
	loading      bool
	dependencies []string
}

// clonePackages detaches dependency lists before a registration transaction.
func (e *engine) clonePackages() map[string]packageRecord {
	records := maps.Clone(e.packages)
	for name, record := range records {
		record.dependencies = slices.Clone(record.dependencies)
		records[name] = record
	}
	return records
}

// unloadPlugin restores covered registrations and defers resource disposal until host acceptance.
func (e *engine) unloadPlugin(name string) error {
	record, ok := e.packages[name]
	if !ok {
		return fmt.Errorf("plugin is not loaded: %s", name)
	}
	if record.loading || record.owner == e.owner {
		return fmt.Errorf("cannot unload the currently executing plugin: %s", name)
	}
	for _, dependent := range slices.Sorted(maps.Keys(e.packages)) {
		candidate := e.packages[dependent]
		if slices.Contains(candidate.dependencies, record.owner) {
			return fmt.Errorf("plugin %s is required by %s", name, dependent)
		}
	}
	for id, call := range e.scheduled {
		if call.owner == record.owner {
			delete(e.scheduled, id)
		}
	}
	for id, callback := range e.resources {
		if callback.owner == record.owner {
			e.released = append(e.released, id)
			delete(e.resources, id)
			delete(e.delivered, id)
		}
	}
	e.manifest.Resources = slices.DeleteFunc(e.manifest.Resources, func(id string) bool { return slices.Contains(e.released, id) })
	e.effects = slices.DeleteFunc(e.effects, func(effect Effect) bool {
		return effect.Name == "resource.activate" && slices.Contains(e.released, effect.Argument)
	})
	if _, ok := e.surfaceOwners[record.owner]; ok {
		e.surfaceReleased = append(e.surfaceReleased, record.owner)
		delete(e.surfaceOwners, record.owner)
	}
	e.manifest.Surface = slices.DeleteFunc(e.manifest.Surface, func(mutation SurfaceMutation) bool { return mutation.Owner == record.owner })
	e.removeOwnerRegistrations(record.owner)
	for key, layers := range e.bindingLayers {
		e.bindingLayers[key] = slices.DeleteFunc(layers, func(layer ownedBinding) bool {
			return e.callbackOwners[layer.binding.Action] == record.owner ||
				(strings.HasPrefix(layer.binding.Action, "command.") && !e.actions[layer.binding.Action])
		})
		e.refreshOwnedBinding(key)
	}
	for key, layers := range e.commandLayers {
		e.commandLayers[key] = slices.DeleteFunc(layers, func(layer ownedCommand) bool {
			return e.callbackOwners[layer.command.Callback] == record.owner
		})
		e.refreshOwnedCommand(key)
	}
	e.effects = slices.DeleteFunc(e.effects, func(effect Effect) bool {
		_, pending := e.scheduled[effect.Argument]
		return e.callbackOwners[effect.Name] == record.owner || (effect.Name == "schedule.callback" && !pending)
	})
	e.cleanup = slices.DeleteFunc(e.cleanup, func(hook cleanupHook) bool {
		if hook.owner != record.owner {
			return false
		}
		e.disposing = append(e.disposing, hook)
		return true
	})
	delete(e.loaded, name)
	delete(e.packages, name)
	e.dirty = !e.setup
	e.refreshManifest()
	return nil
}
