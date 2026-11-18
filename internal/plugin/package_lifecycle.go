package plugin

import (
	"fmt"
	"maps"
	"slices"
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
	e.removeOwnerRegistrations(record.owner)
	for key, layers := range e.bindingLayers {
		e.bindingLayers[key] = slices.DeleteFunc(layers, func(layer ownedBinding) bool {
			return e.callbackOwners[layer.binding.Action] == record.owner
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
		return e.callbackOwners[effect.Name] == record.owner
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
