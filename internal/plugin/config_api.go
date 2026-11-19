package plugin

import (
	"fmt"
	"slices"
	"strings"

	"github.com/erdajt/wraith/internal/keymap"
	lua "github.com/yuin/gopher-lua"
)

// installConfig exposes detached settings, validated updates and custom palettes.
func (e *engine) installConfig(api *lua.LTable) {
	L := e.L
	settings := L.SetFuncs(L.NewTable(), map[string]lua.LGFunction{
		"get": func(L *lua.LState) int {
			var value any = configValue(e.manifest.Config)
			if path := L.OptString(1, ""); path != "" {
				for _, key := range strings.Split(path, ".") {
					object, ok := value.(map[string]any)
					if !ok {
						L.RaiseError("unknown setting: %s", path)
					}
					value, ok = object[key]
					if !ok {
						L.RaiseError("unknown setting: %s", path)
					}
				}
			}
			L.Push(encodeValue(L, value))
			return 1
		},
		"set": func(L *lua.LState) int {
			value, err := decodeValue(L.Get(2))
			if err != nil {
				L.RaiseError("config.set: %v", err)
			}
			e.setConfig(L, L.CheckString(1), value)
			return 0
		},
		"setup": func(L *lua.LState) int {
			value, err := decodeValue(L.CheckTable(1))
			if err != nil {
				L.RaiseError("config.setup: %v", err)
			}
			object, ok := value.(map[string]any)
			if !ok {
				L.RaiseError("config.setup requires named settings")
			}
			e.setConfig(L, "", mergeValue(configValue(e.manifest.Config), object))
			if e.setup {
				var mark func(map[string]any, string)
				mark = func(fields map[string]any, prefix string) {
					for key, value := range fields {
						path := prefix + key
						if child, ok := value.(map[string]any); ok && len(child) > 0 {
							mark(child, path+".")
						} else {
							e.manifest.Overridden = append(e.manifest.Overridden, path)
						}
					}
				}
				mark(object, "")
			}
			return 0
		},
	})
	api.RawSetString("config", settings)
	api.RawSetString("theme", L.SetFuncs(L.NewTable(), map[string]lua.LGFunction{
		"register": func(L *lua.LState) int {
			name := L.CheckString(1)
			if len(name) == 0 || len(name) > 80 {
				L.RaiseError("theme name requires 1-80 bytes")
			}
			value, err := decodeValue(L.CheckTable(2))
			if err != nil {
				L.RaiseError("theme.register: %v", err)
			}
			themes := configValue(e.manifest.Config)["themes"].(map[string]any)
			themes[name] = value
			e.setConfig(L, "themes", themes)
			return 0
		},
		"use": func(L *lua.LState) int {
			e.setConfig(L, "theme", L.CheckString(1))
			return 0
		},
	}))
}

// setConfig applies validated settings during setup or returns them in a runtime action batch.
func (e *engine) setConfig(L *lua.LState, path string, value any) {
	if e.closing {
		L.RaiseError("configuration changes are disabled during teardown")
	}
	next, err := changeConfig(e.manifest.Config, path, value)
	if err != nil {
		L.RaiseError("config.%s: %v", path, err)
	}
	if e.setup {
		if path != "" {
			e.manifest.Overridden = append(e.manifest.Overridden, path)
		}
	} else {
		e.emit(L, Effect{Name: "config", Config: &next})
	}
	if !slices.Equal(e.manifest.Config.Keymaps, next.Keymaps) {
		e.syncOwnerBindings(next.Keymaps, e.owner)
	}
	e.manifest.Config = next
}

// mergeValue retains unspecified object fields while replacing arrays and scalar settings.
func mergeValue(current, update any) any {
	old, oldMap := current.(map[string]any)
	next, nextMap := update.(map[string]any)
	if !oldMap || !nextMap {
		if _, array := current.([]any); array && nextMap && len(next) == 0 {
			return []any{}
		}
		return update
	}
	for key, value := range next {
		old[key] = mergeValue(old[key], value)
	}
	return old
}

// emit bounds host operations and prevents setup or teardown from changing the live workspace.
func (e *engine) emit(L *lua.LState, effect Effect) {
	if e.setup || e.closing {
		L.RaiseError("runtime operations are disabled during setup and teardown")
	}
	if e.effectCount(effect.Name) >= 64 || len(effect.Argument) > 64<<10 {
		L.RaiseError("plugin action batch exceeds its limit")
	}
	if effect.Name == "input.feed" {
		keys, err := keymap.ParseInput(effect.Argument)
		if err != nil {
			L.RaiseError("input.feed: %v", err)
		}
		count := len(keys)
		for _, pending := range e.effects {
			if pending.Name == "input.feed" {
				previous, _ := keymap.ParseInput(pending.Argument)
				count += len(previous)
			}
		}
		if count > 128 {
			L.RaiseError("native input batch exceeds 128 keys")
		}
	}
	if effect.Name == "input.set_mode" && !slices.Contains([]string{"normal", "insert", "visual", "visual-line", "search"}, effect.Argument) {
		L.RaiseError("invalid input mode: %s", effect.Argument)
	}
	e.effects = append(e.effects, effect)
}

// callback retains a function under a stable id within this runtime generation.
func (e *engine) callback(L *lua.LState, value lua.LValue) string {
	fn, ok := value.(*lua.LFunction)
	if !ok {
		L.RaiseError("callback must be a function")
	}
	if len(e.functions) >= 512 {
		e.pruneCallbacks()
	}
	if len(e.functions) >= 512 {
		L.RaiseError("plugin callback limit reached")
	}
	e.serial++
	id := fmt.Sprintf("lua.callback.%d", e.serial)
	e.functions[id] = fn
	e.callbackOwners[id] = e.owner
	e.manifest.Callbacks = append(e.manifest.Callbacks, id)
	return id
}

// effectCount keeps deferred callbacks from consuming the ordinary host action budget.
func (e *engine) effectCount(name string) int {
	scheduled := name == "schedule.callback"
	count := 0
	for _, effect := range e.effects {
		if (effect.Name == "schedule.callback") == scheduled {
			count++
		}
	}
	return count
}
