package plugin

import (
	"regexp"
	"slices"
	"strings"

	"github.com/erdajt/wraith/internal/keymap"
	lua "github.com/yuin/gopher-lua"
)

var customEventName = regexp.MustCompile(`^[a-z][a-z0-9_-]*(\.[a-z][a-z0-9_-]*)+$`)

var commandName = regexp.MustCompile(`^[a-z][a-z0-9-]*(\.[a-z0-9-]+)*!?$`)

// installRegistration exposes live bindings, commands and event subscriptions.
func (e *engine) installRegistration(api *lua.LTable) {
	L := e.L
	api.RawSetString("keymap", L.SetFuncs(L.NewTable(), map[string]lua.LGFunction{
		"set": func(L *lua.LState) int {
			e.registration(L)
			mode, keys := L.CheckString(1), L.CheckString(2)
			binding := keymap.Binding{Mode: mode, Keys: keys, Description: L.OptString(4, "lua mapping")}
			var callback *lua.LFunction
			switch target := L.Get(3).(type) {
			case lua.LString:
				binding.Action = string(target)
			case *lua.LTable:
				target.ForEach(func(key, value lua.LValue) {
					input, ok := value.(lua.LString)
					if key.String() != "keys" || !ok {
						L.RaiseError("native mapping requires {keys = string}")
					}
					binding.Input = string(input)
				})
			case *lua.LFunction:
				callback, binding.Action = target, "lua.callback.pending"
			default:
				L.RaiseError("keymap.set requires an action, native keys or callback")
			}
			if _, err := keymap.New([]keymap.Binding{binding}); err != nil {
				L.RaiseError("keymap.set: %v", err)
			}
			id, _ := bindingIdentity(binding)
			if len(e.bindingLayers[id]) == 0 && len(e.manifest.Config.Keymaps) >= 512 {
				L.RaiseError("keymap limit reached")
			}
			if callback != nil {
				binding.Action = e.callback(L, callback)
			}
			e.setOwnedBinding(id, binding)
			if !slices.Contains(e.manifest.Overridden, "keymaps") {
				e.manifest.Overridden = append(e.manifest.Overridden, "keymaps")
			}
			return 0
		},
		"del": func(L *lua.LState) int {
			e.registration(L)
			mode, text := L.CheckString(1), L.CheckString(2)
			id, err := bindingIdentity(keymap.Binding{Mode: mode, Keys: text})
			if err != nil {
				L.RaiseError("keymap.del: %v", err)
			}
			if layers := e.bindingLayers[id]; len(layers) > 0 {
				layers = slices.DeleteFunc(layers, func(layer ownedBinding) bool { return layer.owner == e.owner })
				e.bindingLayers[id] = append(layers, ownedBinding{owner: e.owner, deleted: true})
				e.refreshOwnedBinding(id)
			}
			if !slices.Contains(e.manifest.Overridden, "keymaps") {
				e.manifest.Overridden = append(e.manifest.Overridden, "keymaps")
			}
			return 0
		},
	}))
	api.RawSetString("commands", L.SetFuncs(L.NewTable(), map[string]lua.LGFunction{
		"create": func(L *lua.LState) int {
			e.registration(L)
			name := L.CheckString(1)
			replace, arguments := false, false
			if options := L.OptTable(4, nil); options != nil {
				options.ForEach(func(key, value lua.LValue) {
					flag, ok := value.(lua.LBool)
					if !ok || key.String() != "replace" && key.String() != "arguments" {
						L.RaiseError("command options require boolean replace or arguments")
					}
					if key.String() == "replace" {
						replace = bool(flag)
					} else {
						arguments = bool(flag)
					}
				})
			}
			existing := slices.ContainsFunc(e.manifest.Commands, func(command Command) bool { return command.Name == name })
			parent, _, dotted := strings.Cut(name, ".")
			reservedParent := dotted && e.builtins["command."+parent] && !slices.ContainsFunc(e.manifest.Commands, func(command Command) bool { return command.Name == parent })
			if reservedParent && !(replace && e.builtins["command."+name]) || !commandName.MatchString(name) || len(name) > 80 || e.actions["command."+name] && !replace || !existing && len(e.manifest.Commands) >= 128 {
				L.RaiseError("invalid, reserved or duplicate command: %s", name)
			}
			description := L.OptString(3, "lua command")
			callback := e.callback(L, L.Get(2))
			e.setOwnedCommand(Command{Name: name, Description: description, Callback: callback, NeedsArgument: arguments})
			return 0
		},
		"delete": func(L *lua.LState) int {
			e.registration(L)
			name := L.CheckString(1)
			if layers := e.commandLayers[name]; len(layers) > 0 {
				layers = slices.DeleteFunc(layers, func(layer ownedCommand) bool { return layer.owner == e.owner })
				e.commandLayers[name] = append(layers, ownedCommand{owner: e.owner, deleted: true})
				e.refreshOwnedCommand(name)
			}
			return 0
		},
	}))
	api.RawSetString("events", L.SetFuncs(L.NewTable(), map[string]lua.LGFunction{
		"on": func(L *lua.LState) int {
			e.registration(L)
			name := L.CheckString(1)
			if !slices.Contains([]string{"ready", "session_changed", "window_changed", "buffer_changed", "response_finished"}, name) && (!customEventName.MatchString(name) || len(name) > 128) {
				L.RaiseError("unknown event: %s", name)
			}
			if len(e.events[name]) >= 32 {
				L.RaiseError("events.on: event listener limit reached")
			}
			id := e.callback(L, L.Get(2))
			e.events[name] = append(e.events[name], id)
			L.Push(lua.LString(id))
			return 1
		},
		"emit": func(L *lua.LState) int {
			name := L.CheckString(1)
			if !customEventName.MatchString(name) || len(name) > 128 {
				L.RaiseError("events.emit requires a namespaced event")
			}
			value, err := decodeValue(L.Get(2))
			if err != nil {
				L.RaiseError("events.emit: %v", err)
			}
			e.emit(L, Effect{Name: "event.emit", Argument: name, Data: value})
			return 0
		},
		"off": func(L *lua.LState) int {
			if !e.setup && !e.closing {
				e.dirty = true
			}
			id := L.CheckString(1)
			for name, ids := range e.events {
				e.events[name] = slices.DeleteFunc(ids, func(value string) bool { return value == id })
			}
			return 0
		},
	}))
}

// registration stages structural changes for host validation before publication.
func (e *engine) registration(L *lua.LState) {
	if e.closing {
		L.RaiseError("registrations are disabled during teardown")
	}
	e.dirty = !e.setup
}
