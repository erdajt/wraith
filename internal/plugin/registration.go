package plugin

import (
	"regexp"
	"slices"
	"strings"

	"github.com/erdajt/wraith/internal/keymap"
	lua "github.com/yuin/gopher-lua"
)

var commandName = regexp.MustCompile(`^[a-z][a-z0-9-]*(\.[a-z0-9-]+)*$`)

// installRegistration exposes setup-only bindings, commands and event subscriptions.
func (e *engine) installRegistration(api *lua.LTable) {
	L := e.L
	api.RawSetString("keymap", L.SetFuncs(L.NewTable(), map[string]lua.LGFunction{
		"set": func(L *lua.LState) int {
			e.registration(L)
			mode, keys := L.CheckString(1), L.CheckString(2)
			name, ok := L.Get(3).(lua.LString)
			if !ok {
				name = lua.LString(e.callback(L, L.Get(3)))
			}
			binding := keymap.Binding{Mode: mode, Keys: keys, Action: string(name), Description: L.OptString(4, "lua mapping")}
			if _, err := keymap.New([]keymap.Binding{binding}); err != nil {
				L.RaiseError("keymap.set: %v", err)
			}
			if len(e.manifest.Config.Keymaps) >= 512 {
				L.RaiseError("keymap limit reached")
			}
			e.manifest.Config.Keymaps = append(e.manifest.Config.Keymaps, binding)
			e.manifest.Overridden = append(e.manifest.Overridden, "keymaps")
			return 0
		},
		"del": func(L *lua.LState) int {
			e.registration(L)
			mode, text := L.CheckString(1), L.CheckString(2)
			keys, err := keymap.Parse(text)
			if err != nil {
				L.RaiseError("keymap.del: %v", err)
			}
			e.manifest.Config.Keymaps = slices.DeleteFunc(e.manifest.Config.Keymaps, func(binding keymap.Binding) bool {
				other, _ := keymap.Parse(binding.Keys)
				return mode == binding.Mode && slices.Equal(keys, other)
			})
			e.manifest.Overridden = append(e.manifest.Overridden, "keymaps")
			return 0
		},
	}))
	api.RawSetString("commands", L.SetFuncs(L.NewTable(), map[string]lua.LGFunction{
		"create": func(L *lua.LState) int {
			e.registration(L)
			name := L.CheckString(1)
			parent, _, dotted := strings.Cut(name, ".")
			reservedParent := dotted && e.actions["command."+parent] && !slices.ContainsFunc(e.manifest.Commands, func(command Command) bool { return command.Name == parent })
			if reservedParent || !commandName.MatchString(name) || len(name) > 80 || e.actions["command."+name] || len(e.manifest.Commands) >= 128 {
				L.RaiseError("invalid, reserved or duplicate command: %s", name)
			}
			e.actions["command."+name] = true
			e.manifest.Commands = append(e.manifest.Commands, Command{name, L.OptString(3, "lua command"), e.callback(L, L.Get(2))})
			return 0
		},
		"delete": func(L *lua.LState) int {
			e.registration(L)
			name := L.CheckString(1)
			for i, command := range e.manifest.Commands {
				if command.Name == name {
					e.manifest.Commands = slices.Delete(e.manifest.Commands, i, i+1)
					delete(e.actions, "command."+name)
					break
				}
			}
			return 0
		},
	}))
	api.RawSetString("events", L.SetFuncs(L.NewTable(), map[string]lua.LGFunction{
		"on": func(L *lua.LState) int {
			e.registration(L)
			name := L.CheckString(1)
			if !slices.Contains([]string{"ready", "session_changed", "window_changed", "response_finished"}, name) {
				L.RaiseError("unknown event: %s", name)
			}
			id := e.callback(L, L.Get(2))
			e.events[name] = append(e.events[name], id)
			L.Push(lua.LString(id))
			return 1
		},
		"off": func(L *lua.LState) int {
			id := L.CheckString(1)
			for name, ids := range e.events {
				e.events[name] = slices.DeleteFunc(ids, func(value string) bool { return value == id })
			}
			return 0
		},
	}))
}

// registration limits structural changes to candidate setup so reload remains atomic.
func (e *engine) registration(L *lua.LState) {
	if !e.setup || e.closing {
		L.RaiseError("register plugins, commands and keymaps during setup, then reload")
	}
}
