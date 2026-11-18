package plugin

import (
	"os"
	"strconv"
	"strings"

	lua "github.com/yuin/gopher-lua"
)

// installWorkspace exposes snapshots and deferred operations without retaining ui pointers.
func (e *engine) installWorkspace(api *lua.LTable) {
	L := e.L
	L.SetFuncs(api, map[string]lua.LGFunction{
		"getenv": func(L *lua.LState) int {
			name := L.CheckString(1)
			if name == "" || len(name) > 256 || strings.ContainsAny(name, "=\x00") {
				L.RaiseError("invalid environment variable name")
			}
			value, exists := os.LookupEnv(name)
			if len(value) > 64<<10 {
				L.RaiseError("environment value exceeds 64 kib")
			}
			if exists {
				L.Push(lua.LString(value))
			} else {
				L.Push(lua.LNil)
			}
			return 1
		},
		"notify": func(L *lua.LState) int {
			e.emit(L, Effect{Name: "notify", Argument: L.CheckString(1)})
			return 0
		},
		"action": func(L *lua.LState) int {
			name := L.CheckString(1)
			if !e.actions[name] {
				L.RaiseError("unknown action: %s", name)
			}
			e.emit(L, Effect{Name: name, Argument: L.OptString(2, "")})
			return 0
		},
	})
	api.RawSetString("windows", L.SetFuncs(L.NewTable(), map[string]lua.LGFunction{
		"list": func(L *lua.LState) int {
			L.Push(e.snapshotValue(L, "windows"))
			return 1
		},
		"split": func(L *lua.LState) int {
			axis := L.OptString(1, "horizontal")
			if axis != "horizontal" && axis != "vertical" {
				L.RaiseError("split requires horizontal or vertical")
			}
			e.emit(L, Effect{Name: "window.split_" + axis})
			return 0
		},
		"focus": func(L *lua.LState) int {
			id := L.CheckInt(1)
			if id < 1 {
				L.RaiseError("window id must be positive")
			}
			e.emit(L, Effect{Name: "window.focus_id", Argument: strconv.Itoa(id)})
			return 0
		},
		"close": func(L *lua.LState) int {
			id := L.OptInt(1, 0)
			if id < 0 {
				L.RaiseError("window id must be positive")
			}
			e.emit(L, Effect{Name: "window.close_id", Argument: strconv.Itoa(id)})
			return 0
		},
	}))
	api.RawSetString("session", L.SetFuncs(L.NewTable(), map[string]lua.LGFunction{
		"get": func(L *lua.LState) int {
			L.Push(e.snapshotValue(L, "session"))
			return 1
		},
	}))
	api.RawSetString("prompt", L.SetFuncs(L.NewTable(), map[string]lua.LGFunction{
		"get": func(L *lua.LState) int {
			L.Push(e.snapshotValue(L, "prompt"))
			return 1
		},
		"set": func(L *lua.LState) int {
			e.emit(L, Effect{Name: "prompt.set", Argument: L.CheckString(1)})
			return 0
		},
	}))
}

// snapshotValue returns a new detached table for each inspection call.
func (e *engine) snapshotValue(L *lua.LState, key string) lua.LValue {
	if e.setup || e.closing {
		L.RaiseError("workspace snapshots are available only in callbacks")
	}
	object, _ := e.snapshot.(map[string]any)
	return encodeValue(L, object[key])
}
