package plugin

import lua "github.com/yuin/gopher-lua"

// installPlugins loads local setup modules once and records bounded shutdown hooks.
func (e *engine) installPlugins(api *lua.LTable) {
	L := e.L
	e.loaded = map[string]bool{}
	api.RawSetString("plugins", L.SetFuncs(L.NewTable(), map[string]lua.LGFunction{
		"load": func(L *lua.LState) int {
			e.registration(L)
			name := L.CheckString(1)
			if e.loaded[name] || len(e.loaded) >= 128 {
				L.RaiseError("plugin already loaded or plugin limit reached: %s", name)
			}
			opts := L.OptTable(2, L.NewTable())
			if err := L.CallByParam(lua.P{Fn: L.GetGlobal("require"), NRet: 1, Protect: true}, lua.LString(name)); err != nil {
				L.RaiseError("plugin %s: %v", name, err)
			}
			module, ok := L.Get(-1).(*lua.LTable)
			L.Pop(1)
			if !ok {
				L.RaiseError("plugin %s must return a table with setup", name)
			}
			setup, ok := module.RawGetString("setup").(*lua.LFunction)
			if !ok {
				L.RaiseError("plugin %s has no setup function", name)
			}
			if teardown, ok := module.RawGetString("teardown").(*lua.LFunction); ok {
				e.cleanup = append(e.cleanup, teardown)
			}
			if err := L.CallByParam(lua.P{Fn: setup, NRet: 1, Protect: true}, opts); err != nil {
				L.RaiseError("plugin %s setup: %v", name, err)
			}
			if cleanup, ok := L.Get(-1).(*lua.LFunction); ok {
				e.cleanup = append(e.cleanup, cleanup)
			}
			L.Pop(1)
			e.loaded[name] = true
			return 0
		},
	}))
}
