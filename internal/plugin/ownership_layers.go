package plugin

import (
	"maps"
	"slices"
	"strings"

	"github.com/erdajt/wraith/internal/keymap"
)

type ownedCommand struct {
	owner   string
	command Command
	deleted bool
}

type ownedBinding struct {
	owner   string
	binding keymap.Binding
	deleted bool
}

type ownershipLayers struct {
	owner          string
	callbackOwners map[string]string
	commandLayers  map[string][]ownedCommand
	bindingLayers  map[string][]ownedBinding
}

// initOwnership records configuration bindings beneath later package overrides.
func (e *engine) initOwnership() {
	e.ownershipLayers = ownershipLayers{
		owner: "config", callbackOwners: map[string]string{},
		commandLayers: map[string][]ownedCommand{}, bindingLayers: map[string][]ownedBinding{},
	}
	for _, binding := range e.manifest.Config.Keymaps {
		id, _ := bindingIdentity(binding)
		e.bindingLayers[id] = []ownedBinding{{owner: e.owner, binding: binding}}
	}
}

// cloneOwnership preserves hidden registrations for rejected callbacks and package loads.
func (e *engine) cloneOwnership() ownershipLayers {
	copy := ownershipLayers{owner: e.owner, callbackOwners: maps.Clone(e.callbackOwners), commandLayers: map[string][]ownedCommand{}, bindingLayers: map[string][]ownedBinding{}}
	for name, layers := range e.commandLayers {
		copy.commandLayers[name] = slices.Clone(layers)
	}
	for id, layers := range e.bindingLayers {
		copy.bindingLayers[id] = slices.Clone(layers)
	}
	return copy
}

// bindingIdentity resolves equivalent key notation to one owner-layer identity.
func bindingIdentity(binding keymap.Binding) (string, error) {
	keys, err := keymap.Parse(binding.Keys)
	if err != nil {
		return "", err
	}
	return binding.Buffer + "\x00" + binding.Mode + "\x00" + strings.Join(keys, "\x00"), nil
}

// setOwnedCommand replaces this owner's definition without losing earlier owners.
func (e *engine) setOwnedCommand(command Command) {
	layers := slices.DeleteFunc(e.commandLayers[command.Name], func(layer ownedCommand) bool { return layer.owner == e.owner })
	e.commandLayers[command.Name] = append(layers, ownedCommand{owner: e.owner, command: command})
	e.refreshOwnedCommand(command.Name)
}

// refreshOwnedCommand restores the visible command or its built-in fallback.
func (e *engine) refreshOwnedCommand(name string) {
	layers := e.commandLayers[name]
	index := slices.IndexFunc(e.manifest.Commands, func(command Command) bool { return command.Name == name })
	if len(layers) == 0 || layers[len(layers)-1].deleted {
		if len(layers) == 0 {
			delete(e.commandLayers, name)
		}
		if index >= 0 {
			e.manifest.Commands = slices.Delete(e.manifest.Commands, index, index+1)
		}
		if !e.builtins["command."+name] {
			delete(e.actions, "command."+name)
		}
		return
	}
	command := layers[len(layers)-1].command
	if index >= 0 {
		e.manifest.Commands[index] = command
	} else {
		e.manifest.Commands = append(e.manifest.Commands, command)
	}
	e.actions["command."+name] = true
}

// setOwnedBinding replaces this owner's mapping and exposes it above earlier owners.
func (e *engine) setOwnedBinding(id string, binding keymap.Binding) {
	layers := slices.DeleteFunc(e.bindingLayers[id], func(layer ownedBinding) bool { return layer.owner == e.owner })
	e.bindingLayers[id] = append(layers, ownedBinding{owner: e.owner, binding: binding})
	e.refreshOwnedBinding(id)
}

// refreshOwnedBinding applies the top mapping while honoring explicit deletion layers.
func (e *engine) refreshOwnedBinding(id string) {
	e.manifest.Config.Keymaps = slices.DeleteFunc(e.manifest.Config.Keymaps, func(binding keymap.Binding) bool {
		key, _ := bindingIdentity(binding)
		return key == id
	})
	layers := e.bindingLayers[id]
	if len(layers) == 0 {
		delete(e.bindingLayers, id)
		return
	}
	if top := layers[len(layers)-1]; !top.deleted {
		e.manifest.Config.Keymaps = append(e.manifest.Config.Keymaps, top.binding)
	}
}

// retainOwnerCallbacks keeps hidden definitions callable after another package unloads.
func (e *engine) retainOwnerCallbacks(retained map[string]bool) {
	for _, layers := range e.commandLayers {
		for _, layer := range layers {
			if !layer.deleted {
				retained[layer.command.Callback] = true
			}
		}
	}
	for _, layers := range e.bindingLayers {
		for _, layer := range layers {
			if !layer.deleted {
				retained[layer.binding.Action] = true
			}
		}
	}
}

// removeOwnerRegistrations restores covered definitions and removes owned subscriptions.
func (e *engine) removeOwnerRegistrations(owner string) {
	for _, name := range slices.Sorted(maps.Keys(e.commandLayers)) {
		e.commandLayers[name] = slices.DeleteFunc(e.commandLayers[name], func(layer ownedCommand) bool { return layer.owner == owner })
		e.refreshOwnedCommand(name)
	}
	for _, id := range slices.Sorted(maps.Keys(e.bindingLayers)) {
		e.bindingLayers[id] = slices.DeleteFunc(e.bindingLayers[id], func(layer ownedBinding) bool { return layer.owner == owner })
		e.refreshOwnedBinding(id)
	}
	for name, ids := range e.events {
		e.events[name] = slices.DeleteFunc(ids, func(id string) bool { return e.callbackOwners[id] == owner })
	}
}
