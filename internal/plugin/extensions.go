package plugin

import (
	"encoding/json"
	"maps"
	"slices"
)

// ProviderDefinition declares callbacks selected by an explicit plugin provider profile.
type ProviderDefinition struct {
	Name, Description, Owner, Lease string
	Models, Show, Chat              string
	Revision                        uint64
}

// ToolDefinition binds a model-visible schema to one approved asynchronous handler.
type ToolDefinition struct {
	Name, Description, Owner, Lease, Callback, Approval string
	Parameters                                          json.RawMessage
	Revision                                            uint64
}

type extensionState struct {
	catalogDirty   bool
	providerLayers map[string][]ProviderDefinition
	toolLayers     map[string][]ToolDefinition
}

// cloneExtensions preserves covered definitions while a callback awaits host publication.
func (e *engine) cloneExtensions() extensionState {
	result := extensionState{catalogDirty: e.catalogDirty, providerLayers: maps.Clone(e.providerLayers), toolLayers: maps.Clone(e.toolLayers)}
	for name, layers := range result.providerLayers {
		result.providerLayers[name] = slices.Clone(layers)
	}
	for name, layers := range result.toolLayers {
		result.toolLayers[name] = slices.Clone(layers)
	}
	return result
}

// refreshExtensions publishes visible definitions and restores covered owners after removal.
func (e *engine) refreshExtensions() {
	e.catalogDirty = true
	e.manifest.Providers = nil
	e.manifest.Tools = nil
	for _, name := range slices.Sorted(maps.Keys(e.providerLayers)) {
		layers := e.providerLayers[name]
		if len(layers) > 0 {
			e.manifest.Providers = append(e.manifest.Providers, layers[len(layers)-1])
		}
	}
	for _, name := range slices.Sorted(maps.Keys(e.toolLayers)) {
		layers := e.toolLayers[name]
		if len(layers) > 0 {
			e.manifest.Tools = append(e.manifest.Tools, layers[len(layers)-1])
		}
	}
}

// removeOwnerExtensions removes only the retiring owner's layers before publication.
func (e *engine) removeOwnerExtensions(owner string) {
	for name, layers := range e.providerLayers {
		e.providerLayers[name] = slices.DeleteFunc(layers, func(definition ProviderDefinition) bool { return definition.Owner == owner })
		if len(e.providerLayers[name]) == 0 {
			delete(e.providerLayers, name)
		}
	}
	for name, layers := range e.toolLayers {
		e.toolLayers[name] = slices.DeleteFunc(layers, func(definition ToolDefinition) bool { return definition.Owner == owner })
		if len(e.toolLayers[name]) == 0 {
			delete(e.toolLayers, name)
		}
	}
	e.refreshExtensions()
}

// retainExtensionCallbacks keeps hidden implementations available until their owner leaves.
func (e *engine) retainExtensionCallbacks(retained map[string]bool) {
	for _, layers := range e.providerLayers {
		for _, definition := range layers {
			retained[definition.Models], retained[definition.Show], retained[definition.Chat] = true, true, true
		}
	}
	for _, layers := range e.toolLayers {
		for _, definition := range layers {
			retained[definition.Callback] = true
		}
	}
}
